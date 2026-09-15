package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"qinci/internal/core/domain"
	"qinci/internal/core/ports"
)

type GitHubPushPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
}

type WebhookService struct {
	repoStore    ports.RepositoryStore
	runner       ports.CommandRunner
	globalSecret string
}

func NewWebhookService(repoStore ports.RepositoryStore, runner ports.CommandRunner, globalSecret string) *WebhookService {
	return &WebhookService{
		repoStore:    repoStore,
		runner:       runner,
		globalSecret: globalSecret,
	}
}

func VerifyGitHubHMAC(payload []byte, secret, signatureHeader string) error {
	if signatureHeader == "" {
		return errors.New("header X-Hub-Signature-256 tidak ditemukan")
	}
	if !strings.HasPrefix(signatureHeader, "sha256=") {
		return errors.New("format header signature tidak valid (harus diawali 'sha256=')")
	}

	actualSigHex := strings.TrimPrefix(signatureHeader, "sha256=")
	actualSig, err := hex.DecodeString(actualSigHex)
	if err != nil {
		return fmt.Errorf("gagal decode hex signature: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(actualSig, expectedSig) {
		return errors.New("signature tidak cocok")
	}

	return nil
}

func (s *WebhookService) ProcessGitHubPush(ctx context.Context, event, signature string, payloadBytes []byte) (*domain.RepositoryConfig, error) {
	if event != "push" {
		return nil, nil
	}

	var payload GitHubPushPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON payload", domain.ErrInvalidInput)
	}

	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	repoFullName := payload.Repository.FullName
	repoShortName := payload.Repository.Name

	repoConfig, err := s.repoStore.FindByRepoAndBranch(ctx, repoFullName, branch)
	if err != nil {
		return nil, err
	}
	if repoConfig == nil && repoShortName != "" {
		repoConfig, err = s.repoStore.FindByRepoAndBranch(ctx, repoShortName, branch)
		if err != nil {
			return nil, err
		}
	}
	if repoConfig == nil {
		return nil, domain.ErrNotFound
	}

	secretToUse := repoConfig.WebhookSecret
	if secretToUse == "" {
		secretToUse = s.globalSecret
	}

	if secretToUse != "" {
		if err := VerifyGitHubHMAC(payloadBytes, secretToUse, signature); err != nil {
			return nil, fmt.Errorf("%w: %s", domain.ErrUnauthorized, err.Error())
		}
	}

	go func(cfg domain.RepositoryConfig, cloneURL string) {
		bgCtx, bgCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer bgCancel()
		if err := s.runner.Execute(bgCtx, &cfg, cloneURL, "webhook"); err != nil {
			log.Printf("[BACKGROUND ERROR] Eksekusi repo '%s' gagal: %v", cfg.RepoName, err)
		} else {
			log.Printf("[BACKGROUND SUCCESS] Eksekusi repo '%s' selesai dengan sukses.", cfg.RepoName)
		}
	}(*repoConfig, payload.Repository.CloneURL)

	return repoConfig, nil
}
