package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"qinci/internal/core/domain"
	"qinci/internal/core/ports"
)

type RepositoryService struct {
	repoStore ports.RepositoryStore
	logStore  ports.LogStore
	runner    ports.CommandRunner
}

func NewRepositoryService(repoStore ports.RepositoryStore, logStore ports.LogStore, runner ports.CommandRunner) *RepositoryService {
	return &RepositoryService{
		repoStore: repoStore,
		logStore:  logStore,
		runner:    runner,
	}
}

func (s *RepositoryService) ListUserRepositories(ctx context.Context, userID int64) ([]domain.RepositoryConfig, error) {
	return s.repoStore.FindByUserID(ctx, userID)
}

func (s *RepositoryService) GetRepository(ctx context.Context, id, userID int64) (*domain.RepositoryConfig, error) {
	repo, err := s.repoStore.FindByIDAndUserID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, domain.ErrNotFound
	}
	return repo, nil
}

func (s *RepositoryService) CreateRepository(ctx context.Context, repo *domain.RepositoryConfig) error {
	repo.RepoName = strings.TrimSpace(repo.RepoName)
	repo.Branch = strings.TrimSpace(repo.Branch)
	repo.Username = strings.TrimSpace(repo.Username)
	repo.Password = strings.TrimSpace(repo.Password)
	repo.RelativePath = strings.TrimSpace(repo.RelativePath)
	repo.WebhookSecret = strings.TrimSpace(repo.WebhookSecret)
	repo.PostCommands = strings.TrimSpace(repo.PostCommands)

	if repo.RepoName == "" || repo.Username == "" || repo.Password == "" || repo.RelativePath == "" {
		return fmt.Errorf("%w: nama repo, username, password/PAT, dan relative path wajib diisi", domain.ErrInvalidInput)
	}
	if repo.Branch == "" {
		repo.Branch = "main"
	}

	return s.repoStore.Create(ctx, repo)
}

func (s *RepositoryService) UpdateRepository(ctx context.Context, repo *domain.RepositoryConfig) error {
	repo.RepoName = strings.TrimSpace(repo.RepoName)
	repo.Branch = strings.TrimSpace(repo.Branch)
	repo.Username = strings.TrimSpace(repo.Username)
	repo.Password = strings.TrimSpace(repo.Password)
	repo.RelativePath = strings.TrimSpace(repo.RelativePath)
	repo.WebhookSecret = strings.TrimSpace(repo.WebhookSecret)
	repo.PostCommands = strings.TrimSpace(repo.PostCommands)

	if repo.RepoName == "" || repo.Username == "" || repo.RelativePath == "" {
		return fmt.Errorf("%w: semua field wajib harus diisi", domain.ErrInvalidInput)
	}
	if repo.Branch == "" {
		repo.Branch = "main"
	}

	return s.repoStore.Update(ctx, repo)
}

func (s *RepositoryService) DeleteRepository(ctx context.Context, id, userID int64) error {
	return s.repoStore.Delete(ctx, id, userID)
}

func (s *RepositoryService) TriggerManual(ctx context.Context, id, userID int64) (*domain.RepositoryConfig, error) {
	repo, err := s.GetRepository(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	cloneURL := fmt.Sprintf("https://github.com/%s.git", repo.RepoName)
	go func(cfg domain.RepositoryConfig) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := s.runner.Execute(bgCtx, &cfg, cloneURL, "manual"); err != nil {
			log.Printf("[BACKGROUND ERROR] Manual trigger '%s' gagal: %v", cfg.RepoName, err)
		}
	}(*repo)

	return repo, nil
}

func (s *RepositoryService) GetRepositoryLogs(ctx context.Context, repoID, userID int64, limit int) (*domain.RepositoryConfig, []domain.RepositoryLog, error) {
	repo, err := s.GetRepository(ctx, repoID, userID)
	if err != nil {
		return nil, nil, err
	}

	logs, err := s.logStore.FindByRepoID(ctx, repo.ID, limit)
	if err != nil {
		return nil, nil, err
	}

	return repo, logs, nil
}
