package service

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"qinci/internal/core/domain"
	"qinci/internal/core/ports"
)

type AuthService struct {
	userRepo ports.UserRepository
	isProd   bool
}

func NewAuthService(userRepo ports.UserRepository, isProd bool) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		isProd:   isProd,
	}
}

func (s *AuthService) Login(ctx context.Context, username, password string) (*domain.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, domain.ErrInvalidInput
	}

	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, domain.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	return user, nil
}

func (s *AuthService) Register(ctx context.Context, username, password, confirmPassword, fullName, telegramChatID string) (*domain.User, error) {
	if s.isProd {
		return nil, domain.ErrRegistrationBlocked
	}

	username = strings.TrimSpace(username)
	fullName = strings.TrimSpace(fullName)
	telegramChatID = strings.TrimSpace(telegramChatID)

	if username == "" || password == "" {
		return nil, domain.ErrInvalidInput
	}
	if password != confirmPassword {
		return nil, fmt.Errorf("konfirmasi password tidak cocok")
	}
	if len(password) < 6 {
		return nil, fmt.Errorf("password minimal 6 karakter")
	}

	existing, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrUsernameExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("gagal mengenkripsi password: %w", err)
	}

	return s.userRepo.Create(ctx, username, string(hashedPassword), fullName, telegramChatID)
}
