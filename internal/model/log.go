package model

import "time"

// RepositoryLog merepresentasikan rekaman eksekusi git pull dan build di database
type RepositoryLog struct {
	ID              int64     `json:"id"`
	RepositoryID    int64     `json:"repository_id"`
	TriggerType     string    `json:"trigger_type"` // "webhook" atau "manual"
	Status          string    `json:"status"`       // "success" atau "failed"
	Output          string    `json:"output"`
	ErrorMessage    string    `json:"error_message"`
	DurationSeconds float64   `json:"duration_seconds"`
	CreatedAt       time.Time `json:"created_at"`
}
