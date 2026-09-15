package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUsernameExists      = errors.New("username already exists")
	ErrRegistrationBlocked = errors.New("registration blocked in production")
	ErrInvalidInput        = errors.New("invalid input")
)

type User struct {
	ID             int64     `json:"id"`
	Username       string    `json:"username"`
	PasswordHash   string    `json:"-"`
	FullName       string    `json:"full_name"`
	TelegramChatID string    `json:"telegram_chat_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
