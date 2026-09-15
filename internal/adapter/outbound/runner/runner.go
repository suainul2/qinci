package runner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"qinci/internal/core/domain"
	"qinci/internal/core/ports"
)

type Runner struct {
	logStore ports.LogStore
	userRepo ports.UserRepository
	notifier ports.Notifier
	mu       sync.Mutex
	locks    map[string]*sync.Mutex
}

var _ ports.CommandRunner = (*Runner)(nil)

func NewRunner(logStore ports.LogStore, userRepo ports.UserRepository, notifier ports.Notifier) *Runner {
	return &Runner{
		logStore: logStore,
		userRepo: userRepo,
		notifier: notifier,
		locks:    make(map[string]*sync.Mutex),
	}
}

func (r *Runner) getRepoLock(repo *domain.RepositoryConfig) *sync.Mutex {
	// ponytail: in-memory mutex map per repo key. Ceiling: single server process; upgrade path: database row lock atau redis distributed lock jika multi-instance.
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("id:%d", repo.ID)
	if repo.ID <= 0 {
		key = "name:" + repo.RepoName
	}

	lk, exists := r.locks[key]
	if !exists {
		lk = &sync.Mutex{}
		r.locks[key] = lk
	}
	return lk
}

func (r *Runner) Execute(ctx context.Context, repo *domain.RepositoryConfig, cloneURL, triggerType string) error {
	lk := r.getRepoLock(repo)
	lk.Lock()
	defer lk.Unlock()

	startTime := time.Now()
	var fullLog strings.Builder
	var execErr error

	appendLog := func(format string, a ...any) {
		line := fmt.Sprintf(format, a...)
		fullLog.WriteString(line)
		fullLog.WriteString("\n")
		log.Print(line)
	}

	appendLog("[RUNNER] Memulai eksekusi untuk repo '%s' branch '%s' (Trigger: %s)...", repo.RepoName, repo.Branch, triggerType)

	defer func() {
		duration := time.Since(startTime).Seconds()
		status := "success"
		errMsg := ""
		if execErr != nil {
			status = "failed"
			errMsg = execErr.Error()
			appendLog("[RUNNER FAILED] Selesai dengan error (%.2fs): %v", duration, execErr)
		} else {
			appendLog("[RUNNER SUCCESS] Selesai dengan sukses dalam %.2fs", duration)
		}

		if r.logStore != nil && repo.ID > 0 {
			repoLog := &domain.RepositoryLog{
				RepositoryID:    repo.ID,
				TriggerType:     triggerType,
				Status:          status,
				Output:          maskSensitive(fullLog.String(), repo.Password),
				ErrorMessage:    errMsg,
				DurationSeconds: duration,
			}
			dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := r.logStore.Insert(dbCtx, repoLog); err != nil {
				log.Printf("[LOG ERROR] Gagal menyimpan log eksekusi ke database: %v", err)
			}
		}

		if r.userRepo != nil && r.notifier != nil && repo.UserID > 0 {
			go func() {
				notifCtx, notifCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer notifCancel()

				user, err := r.userRepo.FindByID(notifCtx, repo.UserID)
				if err == nil && user != nil && user.TelegramChatID != "" {
					var icon, statusText string
					if status == "success" {
						icon = "✅"
						statusText = "SUKSES"
					} else {
						icon = "❌"
						statusText = "GAGAL"
					}

					msg := fmt.Sprintf(
						"%s <b>Eksekusi Repo: %s</b>\n\n"+
							"📦 <b>Repository:</b> <code>%s</code>\n"+
							"🌿 <b>Branch:</b> <code>%s</code>\n"+
							"🎯 <b>Status:</b> %s\n"+
							"⚡ <b>Pemicu:</b> %s\n"+
							"⏱ <b>Durasi:</b> %.2f detik\n"+
							"🕒 <b>Waktu:</b> %s",
						icon, statusText, repo.RepoName, repo.Branch, statusText, triggerType, duration, time.Now().Format("02 Jan 15:04:05 MST"),
					)
					if errMsg != "" {
						msg += fmt.Sprintf("\n\n⚠️ <b>Pesan Error:</b>\n<code>%s</code>", errMsg)
					}

					if err := r.notifier.Send(notifCtx, user.TelegramChatID, msg); err != nil {
						log.Printf("[TELEGRAM NOTIFIER] Gagal mengirim notifikasi ke chat_id '%s': %v", user.TelegramChatID, err)
					}
				}
			}()
		}
	}()

	absPath, err := filepath.Abs(repo.RelativePath)
	if err != nil {
		execErr = fmt.Errorf("gagal menyelesaikan path '%s': %w", repo.RelativePath, err)
		return execErr
	}

	info, err := os.Stat(absPath)
	if err != nil {
		execErr = fmt.Errorf("folder target '%s' (resolved: %s) tidak ditemukan: %w", repo.RelativePath, absPath, err)
		return execErr
	}
	if !info.IsDir() {
		execErr = fmt.Errorf("path '%s' bukan merupakan sebuah direktori", absPath)
		return execErr
	}

	gitDir := filepath.Join(absPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		execErr = fmt.Errorf("direktori '%s' bukan merupakan git repository (tidak ada folder .git)", absPath)
		return execErr
	}

	gitOut, err := r.runGitPull(ctx, repo, absPath, cloneURL)
	if gitOut != "" {
		appendLog("[GIT OUTPUT]\n%s", gitOut)
	}
	if err != nil {
		execErr = fmt.Errorf("git pull gagal: %w", err)
		return execErr
	}
	appendLog("[RUNNER] Git pull berhasil di '%s'", absPath)

	if strings.TrimSpace(repo.PostCommands) != "" {
		cmdOut, err := r.runPostCommandsWithOutput(ctx, absPath, repo.PostCommands)
		if cmdOut != "" {
			fullLog.WriteString(cmdOut)
			fullLog.WriteString("\n")
		}
		if err != nil {
			execErr = fmt.Errorf("eksekusi post_commands gagal: %w", err)
			return execErr
		}
		appendLog("[RUNNER] Semua post_commands berhasil dijalankan untuk '%s'", repo.RepoName)
	}

	return nil
}

func (r *Runner) runGitPull(ctx context.Context, repo *domain.RepositoryConfig, repoDir, cloneURL string) (string, error) {
	checkoutCmd := exec.CommandContext(ctx, "git", "checkout", repo.Branch)
	checkoutCmd.Dir = repoDir
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return string(out), fmt.Errorf("git checkout %s error: %s (%w)", repo.Branch, string(out), err)
	}

	pullArgs := []string{"pull"}
	if repo.Username != "" && repo.Password != "" && cloneURL != "" {
		authURL, err := buildAuthURL(cloneURL, repo.Username, repo.Password)
		if err == nil {
			pullArgs = []string{"pull", authURL, repo.Branch}
		}
	}

	pullCmd := exec.CommandContext(ctx, "git", pullArgs...)
	pullCmd.Dir = repoDir

	var stdout, stderr bytes.Buffer
	pullCmd.Stdout = &stdout
	pullCmd.Stderr = &stderr

	err := pullCmd.Run()
	if err != nil {
		outErr := maskSensitive(stderr.String(), repo.Password)
		return outErr, fmt.Errorf("%w: %s", err, outErr)
	}

	return maskSensitive(stdout.String(), repo.Password), nil
}

func (r *Runner) runPostCommandsWithOutput(ctx context.Context, workingDir, commands string) (string, error) {
	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(commands))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		rawLine := strings.TrimSpace(scanner.Text())

		if rawLine == "" || strings.HasPrefix(rawLine, "#") || strings.HasPrefix(rawLine, "//") {
			continue
		}

		sb.WriteString(fmt.Sprintf("[COMMAND #%d] Menjalankan: %s\n", lineNum, rawLine))

		cmd, err := buildShellCommand(ctx, rawLine, workingDir)
		if err != nil {
			errMsg := fmt.Sprintf("baris #%d ('%s') error persiapan shell: %v\n", lineNum, rawLine, err)
			sb.WriteString(errMsg)
			return sb.String(), fmt.Errorf("%s", errMsg)
		}

		startTime := time.Now()
		output, err := cmd.CombinedOutput()
		duration := time.Since(startTime)

		outStr := strings.TrimSpace(string(output))
		sb.WriteString(fmt.Sprintf("[COMMAND #%d OUTPUT] (%v):\n%s\n", lineNum, duration, outStr))

		if err != nil {
			if outStr != "" {
				return sb.String(), fmt.Errorf("baris #%d ('%s') error: %s (%w)", lineNum, rawLine, outStr, err)
			}
			return sb.String(), fmt.Errorf("baris #%d ('%s') exit status error: %w", lineNum, rawLine, err)
		}
	}

	return sb.String(), scanner.Err()
}

func buildShellCommand(ctx context.Context, commandStr, workingDir string) (*exec.Cmd, error) {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", commandStr)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", commandStr)
	}

	cmd.Dir = workingDir
	cmd.Env = os.Environ()
	return cmd, nil
}

func buildAuthURL(rawURL, username, password string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(username, password)
	return u.String(), nil
}

func maskSensitive(input, secret string) string {
	if secret == "" {
		return input
	}
	return strings.ReplaceAll(input, secret, "********")
}
