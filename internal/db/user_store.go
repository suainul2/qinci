package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"qinci/internal/model"

	"golang.org/x/crypto/bcrypt"
)

// UserStore menangani interaksi database untuk model User
type UserStore struct {
	db *sql.DB
}

// NewUserStore membuat instance UserStore baru
func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

// Create mendaftarkan user baru dengan enkripsi password bcrypt
func (s *UserStore) Create(ctx context.Context, username, password, fullName string) (*model.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("gagal mengenkripsi password: %w", err)
	}

	query := `
		INSERT INTO users (username, password_hash, full_name)
		VALUES (?, ?, ?)
	`
	res, err := s.db.ExecContext(ctx, query, username, string(hashedPassword), fullName)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &model.User{
		ID:           id,
		Username:     username,
		PasswordHash: string(hashedPassword),
		FullName:     fullName,
	}, nil
}

// FindByUsername mencari user berdasarkan username
func (s *UserStore) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	query := `
		SELECT id, username, password_hash, COALESCE(full_name, ''), created_at, updated_at
		FROM users
		WHERE username = ?
		LIMIT 1
	`
	var u model.User
	err := s.db.QueryRowContext(ctx, query, username).Scan(
		&u.ID,
		&u.Username,
		&u.PasswordHash,
		&u.FullName,
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

// FindByID mencari user berdasarkan ID
func (s *UserStore) FindByID(ctx context.Context, id int64) (*model.User, error) {
	query := `
		SELECT id, username, password_hash, COALESCE(full_name, ''), created_at, updated_at
		FROM users
		WHERE id = ?
		LIMIT 1
	`
	var u model.User
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID,
		&u.Username,
		&u.PasswordHash,
		&u.FullName,
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

// Authenticate memverifikasi username dan password
func (s *UserStore) Authenticate(ctx context.Context, username, password string) (*model.User, error) {
	user, err := s.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("username atau password salah")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("username atau password salah")
	}

	return user, nil
}
