package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"qinci/internal/core/domain"
)

func TestRunPostCommands(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runner_test_*")
	if err != nil {
		t.Fatalf("gagal membuat temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	r := NewRunner(nil, nil, nil)

	commands := `
# ini komentar
echo hello > output.txt

# baris kosong di atas dan komentar ini
echo world >> output.txt
`
	ctx := context.Background()
	_, err = r.runPostCommandsWithOutput(ctx, tempDir, commands)
	if err != nil {
		t.Fatalf("runPostCommandsWithOutput gagal: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tempDir, "output.txt"))
	if err != nil {
		t.Fatalf("gagal membaca output.txt: %v", err)
	}

	outStr := string(content)
	if len(outStr) == 0 {
		t.Fatalf("output file kosong")
	}
}

func TestBuildAuthURL(t *testing.T) {
	rawURL := "https://github.com/myuser/my-repo.git"
	username := "myuser"
	password := "ghp_tokensecret"

	res, err := buildAuthURL(rawURL, username, password)
	if err != nil {
		t.Fatalf("buildAuthURL error: %v", err)
	}

	expected := "https://myuser:ghp_tokensecret@github.com/myuser/my-repo.git"
	if res != expected {
		t.Fatalf("diharapkan %s, didapat %s", expected, res)
	}
}

func TestMaskSensitive(t *testing.T) {
	raw := "fatal: could not read Username for 'https://myuser:secret123@github.com'"
	masked := maskSensitive(raw, "secret123")
	if masked == raw {
		t.Fatalf("sensitive password tidak tersensor")
	}
}

func TestRunnerLock(t *testing.T) {
	r := NewRunner(nil, nil, nil)
	repo := &domain.RepositoryConfig{ID: 101, RepoName: "org/repo"}

	l1 := r.getRepoLock(repo)
	l2 := r.getRepoLock(repo)
	if l1 != l2 {
		t.Fatalf("expected identical mutex instance for the same repo")
	}

	l1.Lock()
	acquired := make(chan bool)
	go func() {
		r.getRepoLock(repo).Lock()
		acquired <- true
	}()

	select {
	case <-acquired:
		t.Fatalf("lock was acquired while already held")
	case <-time.After(50 * time.Millisecond):
	}

	l1.Unlock()
	select {
	case <-acquired:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("lock was not acquired after unlock")
	}
}

func TestRunnerCoalesceQueue(t *testing.T) {
	r := NewRunner(nil, nil, nil)
	repo := &domain.RepositoryConfig{ID: 202, RepoName: "org/coalesce"}

	q := r.getRepoQueue(repo)
	q.mu.Lock()
	q.running = true
	q.mu.Unlock()

	done1 := make(chan error, 1)
	go func() {
		done1 <- r.Execute(context.Background(), repo, "url1", "webhook")
	}()

	time.Sleep(50 * time.Millisecond)
	q.mu.Lock()
	if q.pending == nil {
		q.mu.Unlock()
		t.Fatalf("expected run to be queued as pending")
	}
	if q.pending.triggerType != "webhook" {
		q.mu.Unlock()
		t.Fatalf("expected pending triggerType to be webhook")
	}
	q.mu.Unlock()

	done2 := make(chan error, 1)
	go func() {
		done2 <- r.Execute(context.Background(), repo, "url2", "manual")
	}()

	select {
	case err := <-done1:
		if err != nil {
			t.Fatalf("superseded pending call should receive nil, got: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timed out waiting for superseded pending to unblock")
	}

	q.mu.Lock()
	if q.pending == nil || q.pending.triggerType != "manual" {
		q.mu.Unlock()
		t.Fatalf("expected latest pending triggerType to be manual")
	}
	nextDone := q.pending.done
	q.pending = nil
	q.running = false
	q.mu.Unlock()

	nextDone <- nil
	select {
	case err := <-done2:
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timed out waiting for done2")
	}
}
