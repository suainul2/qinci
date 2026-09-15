package web

import (
	"html/template"
	"net/http"
	"strings"

	"qinci/internal/core/ports"
)

type UserHandler struct {
	userUsecase ports.UserUsecase
	tmpl        *template.Template
}

func NewUserHandler(userUsecase ports.UserUsecase, tmpl *template.Template) *UserHandler {
	return &UserHandler{
		userUsecase: userUsecase,
		tmpl:        tmpl,
	}
}

func (h *UserHandler) ShowSettings(w http.ResponseWriter, r *http.Request) {
	sessionUser := GetUserFromContext(r)
	if sessionUser == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := h.userUsecase.GetProfile(r.Context(), sessionUser.UserID)
	if err != nil {
		http.Error(w, "Gagal memuat profil pengguna", http.StatusInternalServerError)
		return
	}

	_ = h.tmpl.ExecuteTemplate(w, "settings.html", map[string]any{
		"User":    user,
		"Success": r.URL.Query().Get("success"),
		"Error":   r.URL.Query().Get("error"),
	})
}

func (h *UserHandler) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	sessionUser := GetUserFromContext(r)
	if sessionUser == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/settings?error=Permintaan+tidak+valid", http.StatusSeeOther)
		return
	}

	telegramChatID := strings.TrimSpace(r.FormValue("telegram_chat_id"))
	if err := h.userUsecase.UpdateTelegramSettings(r.Context(), sessionUser.UserID, telegramChatID); err != nil {
		http.Redirect(w, r, "/settings?error=Gagal+menyimpan+pengaturan+Telegram", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/settings?success=Pengaturan+Telegram+berhasil+disimpan", http.StatusSeeOther)
}

func (h *UserHandler) HandleTestTelegram(w http.ResponseWriter, r *http.Request) {
	sessionUser := GetUserFromContext(r)
	if sessionUser == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/settings?error=Permintaan+tidak+valid", http.StatusSeeOther)
		return
	}

	telegramChatID := strings.TrimSpace(r.FormValue("telegram_chat_id"))
	if telegramChatID == "" {
		user, _ := h.userUsecase.GetProfile(r.Context(), sessionUser.UserID)
		if user != nil {
			telegramChatID = user.TelegramChatID
		}
	}

	if telegramChatID == "" {
		http.Redirect(w, r, "/settings?error=Telegram+Chat+ID+belum+diisi", http.StatusSeeOther)
		return
	}

	if err := h.userUsecase.SendTestNotification(r.Context(), telegramChatID); err != nil {
		http.Redirect(w, r, "/settings?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/settings?success=Pesan+uji+coba+berhasil+dikirim+ke+Telegram!", http.StatusSeeOther)
}
