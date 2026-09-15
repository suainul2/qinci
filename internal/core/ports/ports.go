package ports

import (
	"context"

	"qinci/internal/core/domain"
)

// --- Driven / Outbound Ports (Secondary) ---

type UserRepository interface {
	Create(ctx context.Context, username, passwordHash, fullName, telegramChatID string) (*domain.User, error)
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	FindByID(ctx context.Context, id int64) (*domain.User, error)
	UpdateTelegramChatID(ctx context.Context, id int64, telegramChatID string) error
}

type RepositoryStore interface {
	FindByRepoAndBranch(ctx context.Context, repoName, branch string) (*domain.RepositoryConfig, error)
	FindByUserID(ctx context.Context, userID int64) ([]domain.RepositoryConfig, error)
	FindByIDAndUserID(ctx context.Context, id, userID int64) (*domain.RepositoryConfig, error)
	Create(ctx context.Context, repo *domain.RepositoryConfig) error
	Update(ctx context.Context, repo *domain.RepositoryConfig) error
	Delete(ctx context.Context, id, userID int64) error
}

type LogStore interface {
	Insert(ctx context.Context, l *domain.RepositoryLog) error
	FindByRepoID(ctx context.Context, repoID int64, limit int) ([]domain.RepositoryLog, error)
	PurgeOldLogs(ctx context.Context, retentionDays int) (int64, error)
}

type CommandRunner interface {
	Execute(ctx context.Context, repo *domain.RepositoryConfig, cloneURL, triggerType string) error
}

type Notifier interface {
	Send(ctx context.Context, recipient, message string) error
}

// --- Driving / Inbound Ports (Primary / Use Cases) ---

type AuthUsecase interface {
	Login(ctx context.Context, username, password string) (*domain.User, error)
	Register(ctx context.Context, username, password, confirmPassword, fullName, telegramChatID string) (*domain.User, error)
}

type UserUsecase interface {
	GetProfile(ctx context.Context, userID int64) (*domain.User, error)
	UpdateTelegramSettings(ctx context.Context, userID int64, telegramChatID string) error
	SendTestNotification(ctx context.Context, telegramChatID string) error
}

type RepositoryUsecase interface {
	ListUserRepositories(ctx context.Context, userID int64) ([]domain.RepositoryConfig, error)
	GetRepository(ctx context.Context, id, userID int64) (*domain.RepositoryConfig, error)
	CreateRepository(ctx context.Context, repo *domain.RepositoryConfig) error
	UpdateRepository(ctx context.Context, repo *domain.RepositoryConfig) error
	DeleteRepository(ctx context.Context, id, userID int64) error
	TriggerManual(ctx context.Context, id, userID int64) (*domain.RepositoryConfig, error)
	GetRepositoryLogs(ctx context.Context, repoID, userID int64, limit int) (*domain.RepositoryConfig, []domain.RepositoryLog, error)
}

type WebhookUsecase interface {
	ProcessGitHubPush(ctx context.Context, event, signature string, payloadBytes []byte) (*domain.RepositoryConfig, error)
}
