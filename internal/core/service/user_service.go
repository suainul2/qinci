package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"qinci/internal/core/domain"
	"qinci/internal/core/ports"
)

type UserService struct {
	userRepo ports.UserRepository
	notifier ports.Notifier
}

var _ ports.UserUsecase = (*UserService)(nil)

func NewUserService(userRepo ports.UserRepository, notifier ports.Notifier) *UserService {
	return &UserService{
		userRepo: userRepo,
		notifier: notifier,
	}
}

func (s *UserService) GetProfile(ctx context.Context, userID int64) (*domain.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, domain.ErrNotFound
	}
	return user, nil
}

func (s *UserService) UpdateTelegramSettings(ctx context.Context, userID int64, telegramChatID string) error {
	telegramChatID = strings.TrimSpace(telegramChatID)
	return s.userRepo.UpdateTelegramChatID(ctx, userID, telegramChatID)
}

func (s *UserService) SendTestNotification(ctx context.Context, telegramChatID string) error {
	telegramChatID = strings.TrimSpace(telegramChatID)
	if telegramChatID == "" {
		return fmt.Errorf("ID Telegram (Chat ID) belum diisi")
	}

	msg := fmt.Sprintf(
		"🔔 <b>Qinci Webhook Auto-Pull</b>\n\n"+
			"Halo! Ini adalah <b>pesan uji coba</b> dari bot server Qinci.\n"+
			"Waktu: <code>%s</code>\n\n"+
			"Pengaturan Telegram Chat ID Anda telah berhasil terhubung!",
		time.Now().Format("02 Jan 2006 15:04:05 MST"),
	)

	return s.notifier.Send(ctx, telegramChatID, msg)
}
