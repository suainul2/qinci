package webhook

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"qinci/internal/config"
	"qinci/internal/db"
	"qinci/internal/model"
	"qinci/internal/runner"
)

// GitHubPushPayload merepresentasikan struktur JSON minimal dari GitHub push event webhook
type GitHubPushPayload struct {
	Ref        string `json:"ref"` // contoh: "refs/heads/main"
	Repository struct {
		Name     string `json:"name"`      // contoh: "my-web-app"
		FullName string `json:"full_name"` // contoh: "myuser/my-web-app"
		CloneURL string `json:"clone_url"` // contoh: "https://github.com/myuser/my-web-app.git"
	} `json:"repository"`
}

// Handler melayani endpoint HTTP GitHub Webhook
type Handler struct {
	cfg       *config.Config
	repoStore *db.RepositoryStore
	runner    *runner.Runner
}

// NewHandler membuat instance webhook Handler baru
func NewHandler(cfg *config.Config, repoStore *db.RepositoryStore, r *runner.Runner) *Handler {
	return &Handler{
		cfg:       cfg,
		repoStore: repoStore,
		runner:    r,
	}
}

// Handle menerima HTTP POST dari GitHub
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metode HTTP harus POST", http.StatusMethodNotAllowed)
		return
	}

	// 1. Periksa header X-GitHub-Event (hanya proses push event)
	event := r.Header.Get("X-GitHub-Event")
	if event == "ping" {
		log.Println("[WEBHOOK] Menerima event 'ping' dari GitHub. Webhook terhubung dengan sukses.")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "pong"}`))
		return
	}
	if event != "push" {
		log.Printf("[WEBHOOK] Mengabaikan event non-push: '%s'", event)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "event diabaikan (hanya push event yang diproses)"}`))
		return
	}

	// 2. Baca seluruh body request
	payloadBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[WEBHOOK ERROR] Gagal membaca body request: %v", err)
		http.Error(w, "Gagal membaca body request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 3. Parse JSON payload
	var payload GitHubPushPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		log.Printf("[WEBHOOK ERROR] JSON payload tidak valid: %v", err)
		http.Error(w, "JSON payload tidak valid", http.StatusBadRequest)
		return
	}

	// Ekstrak nama branch dari ref (misal: "refs/heads/main" -> "main")
	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	repoFullName := payload.Repository.FullName
	repoShortName := payload.Repository.Name

	log.Printf("[WEBHOOK] Diterima push event: repo='%s' branch='%s'", repoFullName, branch)

	// 4. Cari konfigurasi repo di MySQL berdasarkan full_name (misal "owner/repo"), jika tidak ada fallback ke short name
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	repoConfig, err := h.repoStore.FindByRepoAndBranch(ctx, repoFullName, branch)
	if err != nil {
		log.Printf("[WEBHOOK ERROR] Database error mencari repo '%s': %v", repoFullName, err)
		http.Error(w, "Kesalahan internal database", http.StatusInternalServerError)
		return
	}

	if repoConfig == nil && repoShortName != "" {
		// Coba cari dengan nama pendek jika belum ditemukan
		repoConfig, err = h.repoStore.FindByRepoAndBranch(ctx, repoShortName, branch)
		if err != nil {
			log.Printf("[WEBHOOK ERROR] Database error mencari repo '%s': %v", repoShortName, err)
			http.Error(w, "Kesalahan internal database", http.StatusInternalServerError)
			return
		}
	}

	if repoConfig == nil {
		log.Printf("[WEBHOOK INFO] Repository '%s' (branch: '%s') tidak ditemukan di database atau tidak aktif.", repoFullName, branch)
		http.Error(w, "Repository atau branch tidak terdaftar di database", http.StatusNotFound)
		return
	}

	// 5. Verifikasi Signature HMAC SHA-256
	signatureHeader := r.Header.Get("X-Hub-Signature-256")
	secretToUse := repoConfig.WebhookSecret
	if secretToUse == "" {
		secretToUse = h.cfg.GlobalSecret
	}

	if secretToUse != "" {
		if err := VerifyGitHubHMAC(payloadBytes, secretToUse, signatureHeader); err != nil {
			log.Printf("[WEBHOOK SECURITY] Signature tidak valid untuk repo '%s': %v", repoFullName, err)
			http.Error(w, "Signature webhook tidak valid", http.StatusUnauthorized)
			return
		}
	} else {
		log.Printf("[WEBHOOK WARNING] Secret webhook tidak disetel untuk '%s'. Melewati verifikasi HMAC.", repoFullName)
	}

	// 6. Jalankan proses Git Pull & Post Commands di Background Worker (Goroutine)
	// GitHub webhook memiliki timeout 10 detik, jadi kembalikan response 200 OK segera.
	go func(cfg model.RepositoryConfig, cloneURL string) {
		bgCtx, bgCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer bgCancel()

		if err := h.runner.Execute(bgCtx, &cfg, cloneURL, "webhook"); err != nil {
			log.Printf("[BACKGROUND ERROR] Eksekusi repo '%s' gagal: %v", cfg.RepoName, err)
		} else {
			log.Printf("[BACKGROUND SUCCESS] Eksekusi repo '%s' selesai dengan sukses.", cfg.RepoName)
		}
	}(*repoConfig, payload.Repository.CloneURL)

	// 7. Respon sukses ke GitHub
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Push event diterima dan proses git pull serta post commands telah dijadwalkan di background",
		"repo":    repoConfig.RepoName,
		"branch":  repoConfig.Branch,
	})
}
