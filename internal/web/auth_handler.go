package web

import (
	"html/template"
	"net/http"
	"strings"

	"qinci/internal/config"
	"qinci/internal/db"
	"qinci/internal/session"
)

// AuthHandler mengelola proses login, registrasi, dan logout
type AuthHandler struct {
	cfg       *config.Config
	userStore *db.UserStore
	sm        *session.SessionManager
	tmpl      *template.Template
}

// NewAuthHandler membuat instance AuthHandler baru
func NewAuthHandler(cfg *config.Config, userStore *db.UserStore, sm *session.SessionManager, tmpl *template.Template) *AuthHandler {
	return &AuthHandler{
		cfg:       cfg,
		userStore: userStore,
		sm:        sm,
		tmpl:      tmpl,
	}
}

// ShowLoginPage menampilkan form login
func (h *AuthHandler) ShowLoginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.sm.GetSession(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	_ = h.tmpl.ExecuteTemplate(w, "login.html", map[string]any{
		"Error":        r.URL.Query().Get("error"),
		"Success":      r.URL.Query().Get("success"),
		"IsProduction": h.cfg.IsProduction(),
	})
}

// HandleLogin memproses autentikasi form login
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?error=Permintaan+tidak+valid", http.StatusSeeOther)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	if username == "" || password == "" {
		http.Redirect(w, r, "/login?error=Username+dan+password+wajib+diisi", http.StatusSeeOther)
		return
	}

	user, err := h.userStore.Authenticate(r.Context(), username, password)
	if err != nil {
		http.Redirect(w, r, "/login?error=Username+atau+password+salah", http.StatusSeeOther)
		return
	}

	h.sm.CreateSession(w, user.ID, user.Username, user.FullName)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ShowRegisterPage menampilkan form registrasi
func (h *AuthHandler) ShowRegisterPage(w http.ResponseWriter, r *http.Request) {
	if h.cfg.IsProduction() {
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

// HandleRegister memproses registrasi user baru
func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if h.cfg.IsProduction() {
		http.Redirect(w, r, "/login?error=Pendaftaran+pengguna+baru+dinonaktifkan+pada+mode+production", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/register?error=Permintaan+tidak+valid", http.StatusSeeOther)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	fullName := strings.TrimSpace(r.FormValue("full_name"))
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	if username == "" || password == "" {
		http.Redirect(w, r, "/register?error=Username+dan+password+wajib+diisi", http.StatusSeeOther)
		return
	}
	if password != confirmPassword {
		http.Redirect(w, r, "/register?error=Konfirmasi+password+tidak+cocok", http.StatusSeeOther)
		return
	}
	if len(password) < 6 {
		http.Redirect(w, r, "/register?error=Password+minimal+6+karakter", http.StatusSeeOther)
		return
	}

	// Cek apakah username sudah dipakai
	existing, _ := h.userStore.FindByUsername(r.Context(), username)
	if existing != nil {
		http.Redirect(w, r, "/register?error=Username+sudah+digunakan", http.StatusSeeOther)
		return
	}

	_, err := h.userStore.Create(r.Context(), username, password, fullName)
	if err != nil {
		http.Redirect(w, r, "/register?error=Gagal+mendaftarkan+user", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/login?success=Pendaftaran+berhasil.+Silakan+masuk.", http.StatusSeeOther)
}

// HandleLogout menghapus sesi pengguna
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	h.sm.DestroySession(w, r)
	http.Redirect(w, r, "/login?success=Anda+telah+berhasil+keluar", http.StatusSeeOther)
}
