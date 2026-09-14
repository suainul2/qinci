package model

import "time"

// RepositoryConfig merepresentasikan konfigurasi repository yang tersimpan di MySQL
type RepositoryConfig struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	RepoName      string    `json:"repo_name"`
	Branch        string    `json:"branch"`
	Username      string    `json:"username"`
	Password      string    `json:"-"` // PAT atau Password disembunyikan dari JSON response
	RelativePath  string    `json:"relative_path"`
	WebhookSecret string    `json:"-"` // Secret disembunyikan dari JSON response
	PostCommands  string    `json:"post_commands"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
