package webhook

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"qinci/internal/core/domain"
	"qinci/internal/core/ports"
)

type Handler struct {
	webhookUsecase ports.WebhookUsecase
}

func NewHandler(webhookUsecase ports.WebhookUsecase) *Handler {
	return &Handler{
		webhookUsecase: webhookUsecase,
	}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metode HTTP harus POST", http.StatusMethodNotAllowed)
		return
	}

	event := r.Header.Get("X-GitHub-Event")
	if event == "ping" {
		log.Println("[WEBHOOK] Menerima event 'ping' dari GitHub. Webhook terhubung dengan sukses.")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "pong"}`))
		return
	}

	if event != "push" {
		log.Printf("[WEBHOOK] Mengabaikan event non-push: '%s'", event)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "event diabaikan (hanya push event yang diproses)"}`))
		return
	}

	payloadBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[WEBHOOK ERROR] Gagal membaca body request: %v", err)
		http.Error(w, "Gagal membaca body request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	signature := r.Header.Get("X-Hub-Signature-256")
	repoConfig, err := h.webhookUsecase.ProcessGitHubPush(r.Context(), event, signature, payloadBytes)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Printf("[WEBHOOK INFO] Repository tidak ditemukan atau tidak aktif: %v", err)
			http.Error(w, "Repository atau branch tidak terdaftar di database", http.StatusNotFound)
			return
		}
		if errors.Is(err, domain.ErrUnauthorized) {
			log.Printf("[WEBHOOK SECURITY] Signature tidak valid: %v", err)
			http.Error(w, "Signature webhook tidak valid", http.StatusUnauthorized)
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			log.Printf("[WEBHOOK ERROR] Input tidak valid: %v", err)
			http.Error(w, "JSON payload tidak valid", http.StatusBadRequest)
			return
		}

		log.Printf("[WEBHOOK ERROR] Error internal: %v", err)
		http.Error(w, "Kesalahan internal server", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Push event diterima dan proses git pull serta post commands telah dijadwalkan di background",
		"repo":    repoConfig.RepoName,
		"branch":  repoConfig.Branch,
	})
}
