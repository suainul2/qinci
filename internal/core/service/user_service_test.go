package service

import (
	"context"
	"testing"

	"qinci/internal/core/domain"
	"qinci/internal/core/ports"
)

type mockUserRepo struct {
	user *domain.User
}

func (m *mockUserRepo) Create(ctx context.Context, username, passwordHash, fullName, telegramChatID string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	return m.user, nil
}
func (m *mockUserRepo) UpdateTelegramChatID(ctx context.Context, id int64, telegramChatID string) error {
	if m.user != nil {
		m.user.TelegramChatID = telegramChatID
	}
	return nil
}

var _ ports.UserRepository = (*mockUserRepo)(nil)

type mockNotifier struct {
	sentTarget  string
	sentMessage string
}

func (m *mockNotifier) Send(ctx context.Context, recipient, message string) error {
	m.sentTarget = recipient
	m.sentMessage = message
	return nil
}

var _ ports.Notifier = (*mockNotifier)(nil)

func TestUserService_UpdateAndSendTest(t *testing.T) {
	repo := &mockUserRepo{
		user: &domain.User{
			ID:             1,
			Username:       "testuser",
			TelegramChatID: "",
		},
	}
	notifier := &mockNotifier{}
	svc := NewUserService(repo, notifier)

	ctx := context.Background()
	if err := svc.UpdateTelegramSettings(ctx, 1, "987654321"); err != nil {
		t.Fatalf("unexpected error updating telegram: %v", err)
	}

	if repo.user.TelegramChatID != "987654321" {
		t.Fatalf("expected chat ID '987654321', got '%s'", repo.user.TelegramChatID)
	}

	if err := svc.SendTestNotification(ctx, "987654321"); err != nil {
		t.Fatalf("unexpected error sending test: %v", err)
	}

	if notifier.sentTarget != "987654321" {
		t.Fatalf("expected recipient '987654321', got '%s'", notifier.sentTarget)
	}
}
