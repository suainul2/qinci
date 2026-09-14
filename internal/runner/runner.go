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
	"time"

	"qinci/internal/db"
	"qinci/internal/model"
)

// Runner bertanggung jawab untuk menjalankan git pull dan rangkaian post commands
type Runner struct {
	logStore *db.LogStore
}

// NewRunner membuat instance Runner baru
func NewRunner(logStore *db.LogStore) *Runner {
	return &Runner{
		logStore: logStore,
	}
}

// Execute menjalankan proses git pull diikuti post_commands, dan mencatat log ke database
func (r *Runner) Execute(ctx context.Context, repo *model.RepositoryConfig, cloneURL, triggerType string) error {
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

		// Simpan record ke database jika logStore tersedia dan ID repository valid
		if r.logStore != nil && repo.ID > 0 {
			repoLog := &model.RepositoryLog{
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
	}()

	// 1. Validasi & resolve relative path ke absolute path
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

	// 2. Jalankan git pull
	gitOut, err := r.runGitPull(ctx, repo, absPath, cloneURL)
	if gitOut != "" {
		appendLog("[GIT OUTPUT]\n%s", gitOut)
	}
	if err != nil {
		execErr = fmt.Errorf("git pull gagal: %w", err)
		return execErr
	}
	appendLog("[RUNNER] Git pull berhasil di '%s'", absPath)

	// 3. Jalankan post commands jika ada
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

// runGitPull mengeksekusi git checkout & git pull dengan autentikasi HTTPS
func (r *Runner) runGitPull(ctx context.Context, repo *model.RepositoryConfig, repoDir, cloneURL string) (string, error) {
	// Pastikan berada di branch target
	checkoutCmd := exec.CommandContext(ctx, "git", "checkout", repo.Branch)
	checkoutCmd.Dir = repoDir
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return string(out), fmt.Errorf("git checkout %s error: %s (%w)", repo.Branch, string(out), err)
	}

	// Siapkan authenticated remote URL jika kredensial tersedia
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

// runPostCommandsWithOutput menjalankan custom command baris demi baris dan mengumpulkan output
func (r *Runner) runPostCommandsWithOutput(ctx context.Context, workingDir, commands string) (string, error) {
	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(commands))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		rawLine := strings.TrimSpace(scanner.Text())

		// Abaikan baris kosong atau baris komentar
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

// buildShellCommand membuat command yang dibungkus shell sesuai OS host (Windows vs Unix/Linux)
func buildShellCommand(ctx context.Context, commandStr, workingDir string) (*exec.Cmd, error) {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		// Gunakan cmd.exe /C di Windows
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", commandStr)
	} else {
		// Gunakan /bin/sh -c di Linux / MacOS
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", commandStr)
	}

	cmd.Dir = workingDir
	cmd.Env = os.Environ() // mewarisi environment variables host termasuk PATH
	return cmd, nil
}

// buildAuthURL menyisipkan username & password/PAT ke URL HTTPS
func buildAuthURL(rawURL, username, password string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(username, password)
	return u.String(), nil
}

// maskSensitive menyamarkan password pada string log
func maskSensitive(input, secret string) string {
	if secret == "" {
		return input
	}
	return strings.ReplaceAll(input, secret, "********")
}
