package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifyGitHubHMAC_Valid(t *testing.T) {
	secret := "test-secret-123"
	payload := []byte(`{"ref":"refs/heads/main","repository":{"name":"test-repo"}}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	validSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	err := VerifyGitHubHMAC(payload, secret, validSig)
	if err != nil {
		t.Fatalf("diharapkan signature valid, tetapi mendapat error: %v", err)
	}
}

func TestVerifyGitHubHMAC_InvalidSignature(t *testing.T) {
	secret := "test-secret-123"
	payload := []byte(`{"ref":"refs/heads/main"}`)
	invalidSig := "sha256=abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	err := VerifyGitHubHMAC(payload, secret, invalidSig)
	if err == nil {
		t.Fatalf("diharapkan error signature tidak cocok, tetapi berhasil")
	}
}

func TestVerifyGitHubHMAC_MissingSignature(t *testing.T) {
	secret := "test-secret-123"
	payload := []byte(`{"ref":"refs/heads/main"}`)

	err := VerifyGitHubHMAC(payload, secret, "")
	if err != ErrMissingSignature {
		t.Fatalf("diharapkan ErrMissingSignature, tetapi mendapat: %v", err)
	}
}

func TestVerifyGitHubHMAC_InvalidFormat(t *testing.T) {
	secret := "test-secret-123"
	payload := []byte(`{"ref":"refs/heads/main"}`)

	err := VerifyGitHubHMAC(payload, secret, "invalid_sig_without_prefix")
	if err != ErrInvalidFormat {
		t.Fatalf("diharapkan ErrInvalidFormat, tetapi mendapat: %v", err)
	}
}
