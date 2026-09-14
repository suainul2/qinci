package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunPostCommands(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runner_test_*")
	if err != nil {
		t.Fatalf("gagal membuat temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	r := NewRunner()

	// Tes multi line commands dengan baris kosong dan baris komentar
	commands := `
# ini komentar
echo hello > output.txt

# baris kosong di atas dan komentar ini
echo world >> output.txt
`
	ctx := context.Background()
	err = r.runPostCommands(ctx, tempDir, commands)
	if err != nil {
		t.Fatalf("runPostCommands gagal: %v", err)
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
