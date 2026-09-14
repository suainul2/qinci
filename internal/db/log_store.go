package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"qinci/internal/model"
)

// LogStore bertanggung jawab atas operasi query dan pembersihan tabel repository_logs
type LogStore struct {
	db *sql.DB
}

// NewLogStore membuat instance LogStore baru
func NewLogStore(db *sql.DB) *LogStore {
	return &LogStore{db: db}
}

// Insert mencatat hasil eksekusi ke tabel repository_logs
func (s *LogStore) Insert(ctx context.Context, l *model.RepositoryLog) error {
	query := `
		INSERT INTO repository_logs 
			(repository_id, trigger_type, status, output, error_message, duration_seconds)
		VALUES 
			(?, ?, ?, ?, ?, ?)
	`
	res, err := s.db.ExecContext(ctx, query,
		l.RepositoryID,
		l.TriggerType,
		l.Status,
		l.Output,
		l.ErrorMessage,
		l.DurationSeconds,
	)
	if err != nil {
		return fmt.Errorf("gagal mencatat log eksekusi: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		l.ID = id
	}
	return nil
}

// FindByRepoID mengambil seluruh log dari suatu repository tertentu
func (s *LogStore) FindByRepoID(ctx context.Context, repoID int64, limit int) ([]model.RepositoryLog, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT 
			id, repository_id, trigger_type, status, 
			COALESCE(output, ''), COALESCE(error_message, ''), 
			duration_seconds, created_at
		FROM repository_logs
		WHERE repository_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, repoID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []model.RepositoryLog
	for rows.Next() {
		var l model.RepositoryLog
		if err := rows.Scan(
			&l.ID,
			&l.RepositoryID,
			&l.TriggerType,
			&l.Status,
			&l.Output,
			&l.ErrorMessage,
			&l.DurationSeconds,
			&l.CreatedAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}

	return logs, rows.Err()
}

// PurgeOldLogs menghapus log di database yang usianya sudah melewati batas hari retensi (retentionDays)
func (s *LogStore) PurgeOldLogs(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}

	query := `
		DELETE FROM repository_logs 
		WHERE created_at < DATE_SUB(NOW(), INTERVAL ? DAY)
	`
	res, err := s.db.ExecContext(ctx, query, retentionDays)
	if err != nil {
		return 0, fmt.Errorf("gagal membersihkan log lama: %w", err)
	}

	affected, err := res.RowsAffected()
	if err == nil && affected > 0 {
		log.Printf("[LOG PURGE] Berhasil membersihkan %d log lama yang melewati batas %d hari dari database.", affected, retentionDays)
	}
	return affected, nil
}
