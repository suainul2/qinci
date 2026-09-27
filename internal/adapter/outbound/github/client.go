package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type HookConfig struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Secret      string `json:"secret,omitempty"`
	InsecureSSL string `json:"insecure_ssl,omitempty"`
}

type HookPayload struct {
	Name   string      `json:"name,omitempty"`
	Active bool        `json:"active"`
	Events []string    `json:"events,omitempty"`
	Config *HookConfig `json:"config"`
}

type HookResponse struct {
	ID     int64       `json:"id"`
	URL    string      `json:"url"`
	Active bool        `json:"active"`
	Events []string    `json:"events"`
	Config *HookConfig `json:"config"`
}

// SyncWebhook mencari webhook dengan target URL yang sama di repository GitHub.
// Jika sudah ada, diupdate via PATCH. Jika belum ada, dibuat baru via POST.
func (c *Client) SyncWebhook(ctx context.Context, token, repoName, webhookURL, secret string) (string, error) {
	repoName = strings.TrimSpace(repoName)
	token = strings.TrimSpace(token)
	webhookURL = strings.TrimSpace(webhookURL)

	if repoName == "" {
		return "", fmt.Errorf("repo_name tidak boleh kosong")
	}
	if token == "" {
		return "", fmt.Errorf("token personal access token (password) GitHub tidak tersedia")
	}
	if webhookURL == "" {
		return "", fmt.Errorf("webhook URL tidak valid")
	}

	// 1. Ambil list hooks yang ada di GitHub
	apiListURL := fmt.Sprintf("https://api.github.com/repos/%s/hooks", repoName)
	reqList, err := http.NewRequestWithContext(ctx, http.MethodGet, apiListURL, nil)
	if err != nil {
		return "", fmt.Errorf("gagal membuat request: %w", err)
	}
	c.setHeaders(reqList, token)

	respList, err := c.httpClient.Do(reqList)
	if err != nil {
		return "", fmt.Errorf("gagal menghubungi GitHub API: %w", err)
	}
	defer respList.Body.Close()

	if respList.StatusCode == http.StatusUnauthorized || respList.StatusCode == http.StatusForbidden {
		body, _ := io.ReadAll(respList.Body)
		return "", fmt.Errorf("akses GitHub ditolak (HTTP %d). Pastikan Personal Access Token memiliki scope 'repo' atau 'admin:repo_hook': %s", respList.StatusCode, string(body))
	}
	if respList.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("repository '%s' tidak ditemukan di GitHub atau token tidak memiliki izin", repoName)
	}

	var existingHooks []HookResponse
	if err := json.NewDecoder(respList.Body).Decode(&existingHooks); err != nil {
		return "", fmt.Errorf("gagal membaca respon webhook list: %w", err)
	}

	// 2. Cari apakah webhook dengan URL sama (atau domain yang sama) sudah pernah dipasang
	var matchedHook *HookResponse
	for _, h := range existingHooks {
		if h.Config != nil && (h.Config.URL == webhookURL || strings.Contains(h.Config.URL, "/webhook")) {
			matchedHook = &h
			break
		}
	}

	hookConfig := &HookConfig{
		URL:         webhookURL,
		ContentType: "json",
		Secret:      secret,
		InsecureSSL: "0",
	}

	if matchedHook != nil {
		// Update existing webhook via PATCH
		patchPayload := HookPayload{
			Active: true,
			Events: []string{"push"},
			Config: hookConfig,
		}
		bodyBytes, _ := json.Marshal(patchPayload)

		patchURL := fmt.Sprintf("https://api.github.com/repos/%s/hooks/%d", repoName, matchedHook.ID)
		reqPatch, err := http.NewRequestWithContext(ctx, http.MethodPatch, patchURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", err
		}
		c.setHeaders(reqPatch, token)

		respPatch, err := c.httpClient.Do(reqPatch)
		if err != nil {
			return "", fmt.Errorf("gagal update webhook: %w", err)
		}
		defer respPatch.Body.Close()

		if respPatch.StatusCode < 200 || respPatch.StatusCode >= 300 {
			b, _ := io.ReadAll(respPatch.Body)
			return "", fmt.Errorf("gagal update webhook (HTTP %d): %s", respPatch.StatusCode, string(b))
		}

		return fmt.Sprintf("Webhook GitHub (ID: %d) berhasil diperbarui ke %s", matchedHook.ID, webhookURL), nil
	}

	// Buat webhook baru via POST
	createPayload := HookPayload{
		Name:   "web",
		Active: true,
		Events: []string{"push"},
		Config: hookConfig,
	}
	bodyBytes, _ := json.Marshal(createPayload)

	reqCreate, err := http.NewRequestWithContext(ctx, http.MethodPost, apiListURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	c.setHeaders(reqCreate, token)

	respCreate, err := c.httpClient.Do(reqCreate)
	if err != nil {
		return "", fmt.Errorf("gagal membuat webhook baru: %w", err)
	}
	defer respCreate.Body.Close()

	if respCreate.StatusCode < 200 || respCreate.StatusCode >= 300 {
		b, _ := io.ReadAll(respCreate.Body)
		return "", fmt.Errorf("gagal mendaftarkan webhook baru (HTTP %d): %s", respCreate.StatusCode, string(b))
	}

	var createdHook HookResponse
	_ = json.NewDecoder(respCreate.Body).Decode(&createdHook)

	return fmt.Sprintf("Webhook GitHub baru (ID: %d) berhasil dipasang ke %s", createdHook.ID, webhookURL), nil
}

func (c *Client) setHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "QinCI-Automation-Client")
}
