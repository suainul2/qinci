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

	"qinci/internal/config"
	"qinci/internal/db"
	"qinci/internal/runner"
	"qinci/internal/session"
	"qinci/internal/web"
	"qinci/internal/webhook"
)

func main() {
	log.Println("==================================================")
	log.Println(" GitHub Webhook Auto-Pull & Post-Command Runner  ")
	log.Println("==================================================")

	// 1. Muat konfigurasi
	cfg := config.LoadConfig()
	log.Printf("[CONFIG] Server port: %s", cfg.Port)
	log.Printf("[CONFIG] Menghubungkan ke MySQL di %s:%s/%s...", cfg.DBHost, cfg.DBPort, cfg.DBName)

	// 2. Hubungkan ke database MySQL
	database, err := db.InitDB(cfg.DSN())
	if err != nil {
		log.Fatalf("[FATAL] Gagal menghubungkan ke MySQL: %v\nPeriksa konfigurasi DB_HOST, DB_USER, DB_PASSWORD, dan DB_NAME.", err)
	}
	defer database.Close()
	log.Println("[DATABASE] Koneksi ke MySQL berhasil dan siap digunakan.")

	// 3. Parse file templates HTML
	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("[FATAL] Gagal membaca templates HTML: %v", err)
	}

	// 4. Inisialisasi komponen store, session manager, runner, dan handler
	userStore := db.NewUserStore(database)
	repoStore := db.NewRepositoryStore(database)
	sessionManager := session.NewSessionManager(24 * time.Hour) // Sesi aktif 24 jam
	commandRunner := runner.NewRunner()

	authHandler := web.NewAuthHandler(cfg, userStore, sessionManager, tmpl)
	repoHandler := web.NewRepoHandler(repoStore, commandRunner, tmpl)
	webhookHandler := webhook.NewHandler(cfg, repoStore, commandRunner)

	// 5. Siapkan HTTP Server dan Routing
	mux := http.NewServeMux()

	// Endpoint health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP","time":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Endpoint Webhook GitHub (tidak memerlukan session cookie, divalidasi via secret HMAC)
	mux.HandleFunc("/webhook", webhookHandler.Handle)

	// Route Autentikasi Web
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

	// Route Terproteksi (Hanya bisa diakses setelah login)
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

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 6. Jalankan server dalam goroutine terpisah
	go func() {
		log.Printf("[SERVER] Dashboard Web UI: http://localhost:%s/login", cfg.Port)
		log.Printf("[SERVER] Webhook Endpoint : POST http://0.0.0.0:%s/webhook", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[SERVER FATAL] Gagal menjalankan server: %v", err)
		}
	}()

	// 7. Graceful shutdown listening to interrupt / SIGTERM
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
