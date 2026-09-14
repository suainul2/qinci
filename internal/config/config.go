package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config menampung pengaturan runtime aplikasi
type Config struct {
	AppEnv           string // "local" atau "production"
	Port             string
	DBHost           string
	DBPort           string
	DBUser           string
	DBPassword       string
	DBName           string
	GlobalSecret     string
	LogRetentionDays int // Jumlah hari retensi penyimpanan log di database
}

// LoadConfig memuat konfigurasi dari file .env (jika ada) dan environment variables dengan nilai default
func LoadConfig() *Config {
	// Muat file .env dari folder root jika tersedia
	if err := godotenv.Load(); err != nil {
		log.Println("[CONFIG INFO] File .env tidak ditemukan atau tidak dapat dibaca, menggunakan environment variables sistem / default")
	} else {
		log.Println("[CONFIG INFO] Berhasil memuat konfigurasi dari file .env")
	}

	return &Config{
		AppEnv:           getEnv("APP_ENV", "local"),
		Port:             getEnv("APP_PORT", "8080"),
		DBHost:           getEnv("DB_HOST", "127.0.0.1"),
		DBPort:           getEnv("DB_PORT", "3306"),
		DBUser:           getEnv("DB_USER", "root"),
		DBPassword:       getEnv("DB_PASSWORD", ""),
		DBName:           getEnv("DB_NAME", "github_webhook"),
		GlobalSecret:     getEnv("GLOBAL_WEBHOOK_SECRET", ""),
		LogRetentionDays: getEnvInt("LOG_RETENTION_DAYS", 7), // default 7 hari
	}
}

// IsProduction mengembalikan true jika APP_ENV bernilai "production"
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

// DSN menghasilkan connection string DSN untuk MySQL
func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local",
		c.DBUser,
		c.DBPassword,
		c.DBHost,
		c.DBPort,
		c.DBName,
	)
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}
