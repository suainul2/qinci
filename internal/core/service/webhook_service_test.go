package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func generateValidSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyGitHubHMAC_Valid(t *testing.T) {
	payload := []byte(`{"ref":"refs/heads/main","repository":{"name":"demo"}}`)
	secret := "my_secret_token"
	signature := generateValidSignature(payload, secret)

	if err := VerifyGitHubHMAC(payload, secret, signature); err != nil {
		t.Fatalf("seharusnya valid, tetapi mengembalikan error: %v", err)
	}
}

func TestVerifyGitHubHMAC_InvalidSignature(t *testing.T) {
	payload := []byte(`{"ref":"refs/heads/main"}`)
	secret := "my_secret_token"
	invalidSignature := "sha256=0000000000000000000000000000000000000000000000000000000000000000"

	if err := VerifyGitHubHMAC(payload, secret, invalidSignature); err == nil {
		t.Fatalf("seharusnya gagal karena signature palsu, tetapi berhasil")
	}
}

func TestVerifyGitHubHMAC_MissingSignature(t *testing.T) {
	payload := []byte(`{"ref":"refs/heads/main"}`)
	secret := "my_secret_token"

	if err := VerifyGitHubHMAC(payload, secret, ""); err == nil {
		t.Fatalf("seharusnya gagal jika header signature kosong")
	}
}

func TestVerifyGitHubHMAC_InvalidFormat(t *testing.T) {
	payload := []byte(`{"ref":"refs/heads/main"}`)
	secret := "my_secret_token"

	if err := VerifyGitHubHMAC(payload, secret, "invalid_prefix_signature"); err == nil {
		t.Fatalf("seharusnya gagal jika format header tidak diawali sha256=")
	}
}
