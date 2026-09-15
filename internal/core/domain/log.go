package domain

import "time"

type RepositoryLog struct {
	ID              int64     `json:"id"`
	RepositoryID    int64     `json:"repository_id"`
	TriggerType     string    `json:"trigger_type"`
	Status          string    `json:"status"`
	Output          string    `json:"output"`
	ErrorMessage    string    `json:"error_message"`
	DurationSeconds float64   `json:"duration_seconds"`
	CreatedAt       time.Time `json:"created_at"`
}
