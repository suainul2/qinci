package db

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// InitDB membuka koneksi pool ke database MySQL
func InitDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// Atur pooling koneksi yang optimal
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
