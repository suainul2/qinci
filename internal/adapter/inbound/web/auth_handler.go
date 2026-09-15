package web

import (
	"errors"
	"html/template"
	"net/http"
	"strings"

	"qinci/internal/adapter/outbound/session"
	"qinci/internal/core/domain"
	"qinci/internal/core/ports"
)

type AuthHandler struct {
	authUsecase ports.AuthUsecase
	sm          *session.SessionManager
	tmpl        *template.Template
	isProd      bool
}

func NewAuthHandler(authUsecase ports.AuthUsecase, sm *session.SessionManager, tmpl *template.Template, isProd bool) *AuthHandler {
	return &AuthHandler{
		authUsecase: authUsecase,
		sm:          sm,
		tmpl:        tmpl,
		isProd:      isProd,
	}
}

func (h *AuthHandler) ShowLoginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.sm.GetSession(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	_ = h.tmpl.ExecuteTemplate(w, "login.html", map[string]any{
		"Error":        r.URL.Query().Get("error"),
		"Success":      r.URL.Query().Get("success"),
		"IsProduction": h.isProd,
	})
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?error=Permintaan+tidak+valid", http.StatusSeeOther)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := h.authUsecase.Login(r.Context(), username, password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Redirect(w, r, "/login?error=Username+dan+password+wajib+diisi", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/login?error=Username+atau+password+salah", http.StatusSeeOther)
		return
	}

	h.sm.CreateSession(w, user.ID, user.Username, user.FullName)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) ShowRegisterPage(w http.ResponseWriter, r *http.Request) {
	if h.isProd {
		http.Redirect(w, r, "/login?error=Pendaftaran+pengguna+baru+dinonaktifkan+pada+mode+production", http.StatusSeeOther)
		return
	}
	if _, ok := h.sm.GetSession(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	_ = h.tmpl.ExecuteTemplate(w, "register.html", map[string]any{
		"Error": r.URL.Query().Get("error"),
	})
}

func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if h.isProd {
		http.Redirect(w, r, "/login?error=Pendaftaran+pengguna+baru+dinonaktifkan+pada+mode+production", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/register?error=Permintaan+tidak+valid", http.StatusSeeOther)
		return
	}

	username := r.FormValue("username")
	fullName := r.FormValue("full_name")
	telegramChatID := r.FormValue("telegram_chat_id")
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	_, err := h.authUsecase.Register(r.Context(), username, password, confirmPassword, fullName, telegramChatID)
	if err != nil {
		if errors.Is(err, domain.ErrRegistrationBlocked) {
			http.Redirect(w, r, "/login?error=Pendaftaran+pengguna+baru+dinonaktifkan+pada+mode+production", http.StatusSeeOther)
			return
		}
		if errors.Is(err, domain.ErrUsernameExists) {
			http.Redirect(w, r, "/register?error=Username+sudah+digunakan", http.StatusSeeOther)
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Redirect(w, r, "/register?error=Username+dan+password+wajib+diisi", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/register?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/login?success=Pendaftaran+berhasil.+Silakan+masuk.", http.StatusSeeOther)
}

func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	h.sm.DestroySession(w, r)
	http.Redirect(w, r, "/login?success=Anda+telah+berhasil+keluar", http.StatusSeeOther)
}
