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
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"qinci/internal/core/domain"
	"qinci/internal/core/ports"
)

type repoRunItem struct {
	ctx         context.Context
	repo        *domain.RepositoryConfig
	cloneURL    string
	triggerType string
	done        chan error
}

type repoQueue struct {
	mu      sync.Mutex
	running bool
	pending *repoRunItem
}

type Runner struct {
	logStore ports.LogStore
	userRepo ports.UserRepository
	notifier ports.Notifier
	mu       sync.Mutex
	queues   map[string]*repoQueue
}

var _ ports.CommandRunner = (*Runner)(nil)

func NewRunner(logStore ports.LogStore, userRepo ports.UserRepository, notifier ports.Notifier) *Runner {
	return &Runner{
		logStore: logStore,
		userRepo: userRepo,
		notifier: notifier,
		queues:   make(map[string]*repoQueue),
	}
}

func (r *Runner) getRepoQueue(repo *domain.RepositoryConfig) *repoQueue {
	// ponytail: in-memory queue map per repo key. Ceiling: single server process; upgrade path: redis queue / distributed worker.
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("id:%d", repo.ID)
	if repo.ID <= 0 {
		key = "name:" + repo.RepoName
	}

	q, exists := r.queues[key]
	if !exists {
		q = &repoQueue{}
		r.queues[key] = q
	}
	return q
}

func (r *Runner) getRepoLock(repo *domain.RepositoryConfig) *sync.Mutex {
	return &r.getRepoQueue(repo).mu
}

func (r *Runner) Execute(ctx context.Context, repo *domain.RepositoryConfig, cloneURL, triggerType string) error {
	q := r.getRepoQueue(repo)
	q.mu.Lock()

	if q.running {
		log.Printf("[RUNNER] Repo '%s' sedang berjalan. Eksekusi digabung (coalesced) ke antrean berikutnya.", repo.RepoName)
		doneCh := make(chan error, 1)
		if q.pending != nil {
			q.pending.done <- nil
		}
		q.pending = &repoRunItem{
			ctx:         ctx,
			repo:        repo,
			cloneURL:    cloneURL,
			triggerType: triggerType,
			done:        doneCh,
		}
		q.mu.Unlock()

		select {
		case err := <-doneCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	q.running = true
	q.mu.Unlock()

	currentCtx := ctx
	currentRepo := repo
	currentURL := cloneURL
	currentType := triggerType
	var currentDone chan error

	for {
		execErr := r.executeRun(currentCtx, currentRepo, currentURL, currentType)
		if currentDone != nil {
			currentDone <- execErr
		}

		q.mu.Lock()
		if q.pending == nil {
			q.running = false
			q.mu.Unlock()
			return execErr
		}

		next := q.pending
		q.pending = nil
		q.mu.Unlock()

		currentCtx = next.ctx
		currentRepo = next.repo
		currentURL = next.cloneURL
		currentType = next.triggerType
		currentDone = next.done
	}
}

func (r *Runner) executeRun(ctx context.Context, repo *domain.RepositoryConfig, cloneURL, triggerType string) error {

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
		shell := "/bin/bash"
		args := []string{"-l", "-c", commandStr}
		if _, err := exec.LookPath("bash"); err != nil {
			if _, err := os.Stat("/bin/bash"); err != nil {
				shell = "/bin/sh"
				args = []string{"-c", commandStr}
			}
		}
		cmd = exec.CommandContext(ctx, shell, args...)
	}

	cmd.Dir = workingDir

	env := os.Environ()
	if runtime.GOOS != "windows" {
		// ponytail: fallback home & standard paths when daemon strips env. Ceiling: common dev locations; upgrade path: custom env config.
		hasHome := false
		for _, e := range env {
			if strings.HasPrefix(e, "HOME=") && len(e) > 5 {
				hasHome = true
				break
			}
		}
		if !hasHome {
			if u, err := user.Current(); err == nil && u.HomeDir != "" {
				env = append(env, "HOME="+u.HomeDir)
			}
		}
	}
	cmd.Env = env
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
