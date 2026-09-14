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

	"qinci/internal/model"
)

// Runner bertanggung jawab untuk menjalankan git pull dan rangkaian post commands
type Runner struct{}

// NewRunner membuat instance Runner baru
func NewRunner() *Runner {
	return &Runner{}
}

// Execute menjalankan proses git pull diikuti post_commands
func (r *Runner) Execute(ctx context.Context, repo *model.RepositoryConfig, cloneURL string) error {
	log.Printf("[RUNNER] Memulai eksekusi untuk repo '%s' branch '%s'...", repo.RepoName, repo.Branch)

	// 1. Validasi & resolve relative path ke absolute path
	absPath, err := filepath.Abs(repo.RelativePath)
	if err != nil {
		return fmt.Errorf("gagal menyelesaikan path '%s': %w", repo.RelativePath, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("folder target '%s' (resolved: %s) tidak ditemukan: %w", repo.RelativePath, absPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path '%s' bukan merupakan sebuah direktori", absPath)
	}

	gitDir := filepath.Join(absPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("direktori '%s' bukan merupakan git repository (tidak ada folder .git)", absPath)
	}

	// 2. Jalankan git pull
	if err := r.runGitPull(ctx, repo, absPath, cloneURL); err != nil {
		return fmt.Errorf("git pull gagal: %w", err)
	}
	log.Printf("[RUNNER] Git pull berhasil di '%s'", absPath)

	// 3. Jalankan post commands jika ada
	if strings.TrimSpace(repo.PostCommands) != "" {
		if err := r.runPostCommands(ctx, absPath, repo.PostCommands); err != nil {
			return fmt.Errorf("eksekusi post_commands gagal: %w", err)
		}
		log.Printf("[RUNNER] Semua post_commands berhasil dijalankan untuk '%s'", repo.RepoName)
	}

	return nil
}

// runGitPull mengeksekusi git checkout & git pull dengan autentikasi HTTPS
func (r *Runner) runGitPull(ctx context.Context, repo *model.RepositoryConfig, repoDir, cloneURL string) error {
	// Pastikan berada di branch target
	checkoutCmd := exec.CommandContext(ctx, "git", "checkout", repo.Branch)
	checkoutCmd.Dir = repoDir
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout %s error: %s (%w)", repo.Branch, string(out), err)
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
		// Sensor password jika muncul di output error
		outErr := maskSensitive(stderr.String(), repo.Password)
		return fmt.Errorf("%w: %s", err, outErr)
	}

	log.Printf("[GIT OUTPUT]\n%s", maskSensitive(stdout.String(), repo.Password))
	return nil
}

// runPostCommands menjalankan custom command baris demi baris di folder repo target
func (r *Runner) runPostCommands(ctx context.Context, workingDir, commands string) error {
	scanner := bufio.NewScanner(strings.NewReader(commands))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		rawLine := strings.TrimSpace(scanner.Text())

		// Abaikan baris kosong atau baris komentar
		if rawLine == "" || strings.HasPrefix(rawLine, "#") || strings.HasPrefix(rawLine, "//") {
			continue
		}

		log.Printf("[COMMAND #%d] Menjalankan: %s (di %s)", lineNum, rawLine, workingDir)

		cmd, err := buildShellCommand(ctx, rawLine, workingDir)
		if err != nil {
			return fmt.Errorf("baris #%d ('%s') error persiapan shell: %w", lineNum, rawLine, err)
		}

		startTime := time.Now()
		output, err := cmd.CombinedOutput()
		duration := time.Since(startTime)

		if err != nil {
			log.Printf("[COMMAND #%d GAGAL] (%v) Output:\n%s", lineNum, duration, string(output))
			return fmt.Errorf("baris #%d ('%s') exit status error: %w", lineNum, rawLine, err)
		}

		log.Printf("[COMMAND #%d SELESAI] (%v) Output:\n%s", lineNum, duration, string(output))
	}

	return scanner.Err()
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
