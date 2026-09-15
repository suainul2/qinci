package mysql

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"qinci/internal/core/domain"
	"qinci/internal/core/ports"
)

type UserRepository struct {
	db *sql.DB
}

var _ ports.UserRepository = (*UserRepository)(nil)

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (s *UserRepository) Create(ctx context.Context, username, passwordHash, fullName, telegramChatID string) (*domain.User, error) {
	query := `
		INSERT INTO users (username, password_hash, full_name, telegram_chat_id)
		VALUES (?, ?, ?, ?)
	`
	res, err := s.db.ExecContext(ctx, query, username, passwordHash, fullName, strings.TrimSpace(telegramChatID))
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &domain.User{
		ID:             id,
		Username:       username,
		PasswordHash:   passwordHash,
		FullName:       fullName,
		TelegramChatID: strings.TrimSpace(telegramChatID),
	}, nil
}

func (s *UserRepository) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	query := `
		SELECT id, username, password_hash, COALESCE(full_name, ''), COALESCE(telegram_chat_id, ''), created_at, updated_at
		FROM users
		WHERE username = ?
		LIMIT 1
	`
	var u domain.User
	err := s.db.QueryRowContext(ctx, query, username).Scan(
		&u.ID,
		&u.Username,
		&u.PasswordHash,
		&u.FullName,
		&u.TelegramChatID,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserRepository) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	query := `
		SELECT id, username, password_hash, COALESCE(full_name, ''), COALESCE(telegram_chat_id, ''), created_at, updated_at
		FROM users
		WHERE id = ?
		LIMIT 1
	`
	var u domain.User
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID,
		&u.Username,
		&u.PasswordHash,
		&u.FullName,
		&u.TelegramChatID,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserRepository) UpdateTelegramChatID(ctx context.Context, id int64, telegramChatID string) error {
	query := `UPDATE users SET telegram_chat_id = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, strings.TrimSpace(telegramChatID), id)
	return err
}
