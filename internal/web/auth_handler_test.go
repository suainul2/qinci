package web

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"qinci/internal/config"
	"qinci/internal/session"
)

func TestAuthHandler_RegisterBlockedInProduction(t *testing.T) {
	cfg := &config.Config{
		AppEnv: "production",
	}

	tmpl := template.Must(template.New("dummy").Parse(""))
	sm := session.NewSessionManager(1 * time.Hour)
	handler := NewAuthHandler(cfg, nil, sm, tmpl)

	// 1. Uji GET /register (harus redirect ke /login dengan pesan error)
	reqGet := httptest.NewRequest(http.MethodGet, "/register", nil)
	wGet := httptest.NewRecorder()
	handler.ShowRegisterPage(wGet, reqGet)

	respGet := wGet.Result()
	if respGet.StatusCode != http.StatusSeeOther {
		t.Fatalf("diharapkan status %d, didapat %d", http.StatusSeeOther, respGet.StatusCode)
	}
	locGet := respGet.Header.Get("Location")
	if !strings.Contains(locGet, "/login") || !strings.Contains(locGet, "production") {
		t.Fatalf("diharapkan redirect ke /login dengan pesan production, didapat: %s", locGet)
	}

	// 2. Uji POST /register (harus langsung ditolak dan redirect ke /login)
	formData := url.Values{}
	formData.Set("username", "badactor")
	formData.Set("password", "secret123")
	formData.Set("confirm_password", "secret123")

	reqPost := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(formData.Encode()))
	reqPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wPost := httptest.NewRecorder()
	handler.HandleRegister(wPost, reqPost)

	respPost := wPost.Result()
	if respPost.StatusCode != http.StatusSeeOther {
		t.Fatalf("diharapkan status %d, didapat %d", http.StatusSeeOther, respPost.StatusCode)
	}
	locPost := respPost.Header.Get("Location")
	if !strings.Contains(locPost, "/login") || !strings.Contains(locPost, "production") {
		t.Fatalf("diharapkan POST /register diblokir di production, didapat: %s", locPost)
	}
}
