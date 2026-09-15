package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"qinci/internal/adapter/inbound/web"
	"qinci/internal/adapter/inbound/webhook"
	"qinci/internal/adapter/outbound/mysql"
	"qinci/internal/adapter/outbound/runner"
	"qinci/internal/adapter/outbound/session"
	"qinci/internal/adapter/outbound/telegram"
	"qinci/internal/config"
	"qinci/internal/core/service"
)

func main() {
	log.Println("==================================================")
	log.Println(" GitHub Webhook Auto-Pull & Post-Command Runner  ")
	log.Println("==================================================")

	// 1. Konfigurasi
	cfg := config.LoadConfig()
	log.Printf("[CONFIG] Server port: %s", cfg.Port)
	log.Printf("[CONFIG] Menghubungkan ke MySQL di %s:%s/%s...", cfg.DBHost, cfg.DBPort, cfg.DBName)

	// 2. Outbound Adapter: Database
	database, err := mysql.InitDB(cfg.DSN())
	if err != nil {
		log.Fatalf("[FATAL] Gagal menghubungkan ke MySQL: %v\nPeriksa konfigurasi DB_HOST, DB_USER, DB_PASSWORD, dan DB_NAME.", err)
	}
	defer database.Close()
	log.Println("[DATABASE] Koneksi ke MySQL berhasil dan siap digunakan.")

	// 3. Templates UI
	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("[FATAL] Gagal membaca templates HTML: %v", err)
	}

	// 4. Outbound Adapters
	userRepo := mysql.NewUserRepository(database)
	repoStore := mysql.NewRepositoryStore(database)
	logStore := mysql.NewLogStore(database)
	sessionManager := session.NewSessionManager(24 * time.Hour)
	telegramNotifier := telegram.NewTelegramNotifier(cfg.TelegramBotToken)
	commandRunner := runner.NewRunner(logStore, userRepo, telegramNotifier)

	// 5. Core Services (Application / Usecases)
	authService := service.NewAuthService(userRepo, cfg.IsProduction())
	userService := service.NewUserService(userRepo, telegramNotifier)
	repoService := service.NewRepositoryService(repoStore, logStore, commandRunner)
	webhookService := service.NewWebhookService(repoStore, commandRunner, cfg.GlobalSecret)

	// 6. Inbound Adapters: HTTP Handlers
	authHandler := web.NewAuthHandler(authService, sessionManager, tmpl, cfg.IsProduction())
	userHandler := web.NewUserHandler(userService, tmpl)
	repoHandler := web.NewRepoHandler(repoService, tmpl)
	webhookHandler := webhook.NewHandler(webhookService)

	// Background worker pembersih log lama
	go func() {
		log.Printf("[LOG CLEANER] Rutinitas pembersih log aktif (retensi: %d hari)", cfg.LogRetentionDays)
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()

		cleanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, _ = logStore.PurgeOldLogs(cleanCtx, cfg.LogRetentionDays)
		cancel()

		for range ticker.C {
			ctx, c := context.WithTimeout(context.Background(), 30*time.Second)
			_, _ = logStore.PurgeOldLogs(ctx, cfg.LogRetentionDays)
			c()
		}
	}()

	// 7. Routing HTTP Server
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP","time":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	mux.HandleFunc("/webhook", webhookHandler.Handle)

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authHandler.HandleLogin(w, r)
		} else {
			authHandler.ShowLoginPage(w, r)
		}
	})

	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authHandler.HandleRegister(w, r)
		} else {
			authHandler.ShowRegisterPage(w, r)
		}
	})

	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authHandler.HandleLogout(w, r)
		} else {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}
	})

	mux.HandleFunc("/", web.AuthMiddleware(sessionManager, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		repoHandler.Index(w, r)
	}))

	mux.HandleFunc("/repos/new", web.AuthMiddleware(sessionManager, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			repoHandler.HandleCreate(w, r)
		} else {
			repoHandler.ShowCreateForm(w, r)
		}
	}))

	mux.HandleFunc("/repos/edit", web.AuthMiddleware(sessionManager, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			repoHandler.HandleUpdate(w, r)
		} else {
			repoHandler.ShowEditForm(w, r)
		}
	}))

	mux.HandleFunc("/repos/delete", web.AuthMiddleware(sessionManager, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			repoHandler.HandleDelete(w, r)
		} else {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
	}))

	mux.HandleFunc("/repos/trigger", web.AuthMiddleware(sessionManager, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			repoHandler.HandleTriggerManual(w, r)
		} else {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
	}))

	mux.HandleFunc("/repos/logs", web.AuthMiddleware(sessionManager, func(w http.ResponseWriter, r *http.Request) {
		repoHandler.ShowLogs(w, r)
	}))

	// Route Pengaturan Akun & Notifikasi Telegram
	mux.HandleFunc("/settings", web.AuthMiddleware(sessionManager, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			userHandler.HandleUpdateSettings(w, r)
		} else {
			userHandler.ShowSettings(w, r)
		}
	}))

	mux.HandleFunc("/settings/test-telegram", web.AuthMiddleware(sessionManager, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			userHandler.HandleTestTelegram(w, r)
		} else {
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
		}
	}))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("[SERVER] Dashboard Web UI: http://localhost:%s/login", cfg.Port)
		log.Printf("[SERVER] Webhook Endpoint : POST http://0.0.0.0:%s/webhook", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[SERVER FATAL] Gagal menjalankan server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("\n[SERVER] Menerima sinyal shutdown, mematikan server secara graceful...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("[SERVER ERROR] Shutdown paksa akibat error: %v", err)
	}

	log.Println("[SERVER] Server berhasil dimatikan dengan aman.")
}
