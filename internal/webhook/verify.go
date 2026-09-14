package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

var (
	ErrMissingSignature  = errors.New("signature X-Hub-Signature-256 tidak ditemukan")
	ErrInvalidFormat     = errors.New("format signature tidak valid (harus sha256=...)")
	ErrSignatureMismatch = errors.New("signature HMAC-SHA256 tidak cocok")
)

// VerifyGitHubHMAC memverifikasi payload request terhadap secret menggunakan algoritma HMAC-SHA256
func VerifyGitHubHMAC(payload []byte, secret, signatureHeader string) error {
	if strings.TrimSpace(signatureHeader) == "" {
		return ErrMissingSignature
	}

	parts := strings.SplitN(signatureHeader, "=", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return ErrInvalidFormat
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	// Bandingkan secara constant-time untuk mencegah timing attack
	if !hmac.Equal([]byte(parts[1]), []byte(expectedMAC)) {
		return ErrSignatureMismatch
	}

	return nil
}
