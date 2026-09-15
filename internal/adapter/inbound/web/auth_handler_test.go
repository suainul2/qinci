package web

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"qinci/internal/adapter/outbound/session"
	"qinci/internal/core/domain"
	"qinci/internal/core/ports"
)

type mockAuthUsecase struct {
	loginFn    func(ctx context.Context, username, password string) (*domain.User, error)
	registerFn func(ctx context.Context, username, password, confirmPassword, fullName, telegramChatID string) (*domain.User, error)
}

func (m *mockAuthUsecase) Login(ctx context.Context, username, password string) (*domain.User, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, username, password)
	}
	return nil, nil
}

func (m *mockAuthUsecase) Register(ctx context.Context, username, password, confirmPassword, fullName, telegramChatID string) (*domain.User, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, username, password, confirmPassword, fullName, telegramChatID)
	}
	return nil, nil
}

var _ ports.AuthUsecase = (*mockAuthUsecase)(nil)

func TestAuthHandler_RegisterBlockedInProduction(t *testing.T) {
	tmpl := template.Must(template.New("dummy").Parse(""))
	sm := session.NewSessionManager(1 * time.Hour)
	mock := &mockAuthUsecase{}
	handler := NewAuthHandler(mock, sm, tmpl, true)

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
