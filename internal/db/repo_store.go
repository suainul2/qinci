package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"qinci/internal/model"
)

// RepositoryStore bertanggung jawab atas operasi query tabel repositories
type RepositoryStore struct {
	db *sql.DB
}

// NewRepositoryStore membuat instance RepositoryStore baru
func NewRepositoryStore(db *sql.DB) *RepositoryStore {
	return &RepositoryStore{db: db}
}

// FindByRepoAndBranch mengambil konfigurasi repository aktif berdasarkan nama repo dan branch (digunakan oleh Webhook)
func (s *RepositoryStore) FindByRepoAndBranch(ctx context.Context, repoName, branch string) (*model.RepositoryConfig, error) {
	query := `
		SELECT 
			id, user_id, repo_name, branch, username, password, 
			relative_path, COALESCE(webhook_secret, ''), COALESCE(post_commands, ''), 
			is_active, created_at, updated_at
		FROM repositories
		WHERE repo_name = ? AND branch = ? AND is_active = 1
		LIMIT 1
	`

	var repo model.RepositoryConfig
	err := s.db.QueryRowContext(ctx, query, repoName, branch).Scan(
		&repo.ID,
		&repo.UserID,
		&repo.RepoName,
		&repo.Branch,
		&repo.Username,
		&repo.Password,
		&repo.RelativePath,
		&repo.WebhookSecret,
		&repo.PostCommands,
		&repo.IsActive,
		&repo.CreatedAt,
		&repo.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &repo, nil
}

// FindByUserID mengambil seluruh repositori milik user tertentu
func (s *RepositoryStore) FindByUserID(ctx context.Context, userID int64) ([]model.RepositoryConfig, error) {
	query := `
		SELECT 
			id, user_id, repo_name, branch, username, password, 
			relative_path, COALESCE(webhook_secret, ''), COALESCE(post_commands, ''), 
			is_active, created_at, updated_at
		FROM repositories
		WHERE user_id = ?
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.RepositoryConfig
	for rows.Next() {
		var repo model.RepositoryConfig
		if err := rows.Scan(
			&repo.ID,
			&repo.UserID,
			&repo.RepoName,
			&repo.Branch,
			&repo.Username,
			&repo.Password,
			&repo.RelativePath,
			&repo.WebhookSecret,
			&repo.PostCommands,
			&repo.IsActive,
			&repo.CreatedAt,
			&repo.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, repo)
	}

	return list, rows.Err()
}

// FindByIDAndUserID mengambil satu repo berdasarkan ID dan kepemilikan UserID
func (s *RepositoryStore) FindByIDAndUserID(ctx context.Context, id, userID int64) (*model.RepositoryConfig, error) {
	query := `
		SELECT 
			id, user_id, repo_name, branch, username, password, 
			relative_path, COALESCE(webhook_secret, ''), COALESCE(post_commands, ''), 
			is_active, created_at, updated_at
		FROM repositories
		WHERE id = ? AND user_id = ?
		LIMIT 1
	`

	var repo model.RepositoryConfig
	err := s.db.QueryRowContext(ctx, query, id, userID).Scan(
		&repo.ID,
		&repo.UserID,
		&repo.RepoName,
		&repo.Branch,
		&repo.Username,
		&repo.Password,
		&repo.RelativePath,
		&repo.WebhookSecret,
		&repo.PostCommands,
		&repo.IsActive,
		&repo.CreatedAt,
		&repo.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &repo, nil
}

// Create menyimpan konfigurasi repository baru untuk seorang user
func (s *RepositoryStore) Create(ctx context.Context, repo *model.RepositoryConfig) error {
	query := `
		INSERT INTO repositories 
			(user_id, repo_name, branch, username, password, relative_path, webhook_secret, post_commands, is_active)
		VALUES 
			(?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := s.db.ExecContext(ctx, query,
		repo.UserID,
		repo.RepoName,
		repo.Branch,
		repo.Username,
		repo.Password,
		repo.RelativePath,
		repo.WebhookSecret,
		repo.PostCommands,
		repo.IsActive,
	)
	if err != nil {
		return fmt.Errorf("gagal membuat repository: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		repo.ID = id
	}
	return nil
}

// Update memperbarui data repository
func (s *RepositoryStore) Update(ctx context.Context, repo *model.RepositoryConfig) error {
	var query string
	var args []any

	if repo.Password != "" {
		// Update beserta password baru
		query = `
			UPDATE repositories 
			SET repo_name = ?, branch = ?, username = ?, password = ?, relative_path = ?, webhook_secret = ?, post_commands = ?, is_active = ?
			WHERE id = ? AND user_id = ?
		`
		args = []any{
			repo.RepoName, repo.Branch, repo.Username, repo.Password,
			repo.RelativePath, repo.WebhookSecret, repo.PostCommands, repo.IsActive,
			repo.ID, repo.UserID,
		}
	} else {
		// Update tanpa mengubah password yang sudah ada
		query = `
			UPDATE repositories 
			SET repo_name = ?, branch = ?, username = ?, relative_path = ?, webhook_secret = ?, post_commands = ?, is_active = ?
			WHERE id = ? AND user_id = ?
		`
		args = []any{
			repo.RepoName, repo.Branch, repo.Username,
			repo.RelativePath, repo.WebhookSecret, repo.PostCommands, repo.IsActive,
			repo.ID, repo.UserID,
		}
	}

	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

// Delete menghapus repository milik user
func (s *RepositoryStore) Delete(ctx context.Context, id, userID int64) error {
	query := `DELETE FROM repositories WHERE id = ? AND user_id = ?`
	_, err := s.db.ExecContext(ctx, query, id, userID)
	return err
}
