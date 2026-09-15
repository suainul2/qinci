package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"qinci/internal/core/domain"
	"qinci/internal/core/ports"
)

type RepositoryStore struct {
	db *sql.DB
}

var _ ports.RepositoryStore = (*RepositoryStore)(nil)

func NewRepositoryStore(db *sql.DB) *RepositoryStore {
	return &RepositoryStore{db: db}
}

func (s *RepositoryStore) FindByRepoAndBranch(ctx context.Context, repoName, branch string) (*domain.RepositoryConfig, error) {
	query := `
		SELECT 
			id, user_id, repo_name, branch, username, password, 
			relative_path, COALESCE(webhook_secret, ''), COALESCE(post_commands, ''), 
			is_active, created_at, updated_at
		FROM repositories
		WHERE repo_name = ? AND branch = ? AND is_active = 1
		LIMIT 1
	`

	var repo domain.RepositoryConfig
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

func (s *RepositoryStore) FindByUserID(ctx context.Context, userID int64) ([]domain.RepositoryConfig, error) {
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

	var list []domain.RepositoryConfig
	for rows.Next() {
		var repo domain.RepositoryConfig
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

func (s *RepositoryStore) FindByIDAndUserID(ctx context.Context, id, userID int64) (*domain.RepositoryConfig, error) {
	query := `
		SELECT 
			id, user_id, repo_name, branch, username, password, 
			relative_path, COALESCE(webhook_secret, ''), COALESCE(post_commands, ''), 
			is_active, created_at, updated_at
		FROM repositories
		WHERE id = ? AND user_id = ?
		LIMIT 1
	`

	var repo domain.RepositoryConfig
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

func (s *RepositoryStore) Create(ctx context.Context, repo *domain.RepositoryConfig) error {
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

func (s *RepositoryStore) Update(ctx context.Context, repo *domain.RepositoryConfig) error {
	var query string
	var args []any

	if repo.Password != "" {
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

func (s *RepositoryStore) Delete(ctx context.Context, id, userID int64) error {
	query := `DELETE FROM repositories WHERE id = ? AND user_id = ?`
	_, err := s.db.ExecContext(ctx, query, id, userID)
	return err
}
