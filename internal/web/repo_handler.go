package web

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"qinci/internal/db"
	"qinci/internal/model"
	"qinci/internal/runner"
)

// RepoHandler mengelola halaman dashboard dan CRUD repositori user
type RepoHandler struct {
	repoStore *db.RepositoryStore
	runner    *runner.Runner
	tmpl      *template.Template
}

// NewRepoHandler membuat instance RepoHandler baru
func NewRepoHandler(repoStore *db.RepositoryStore, r *runner.Runner, tmpl *template.Template) *RepoHandler {
	return &RepoHandler{
		repoStore: repoStore,
		runner:    r,
		tmpl:      tmpl,
	}
}

// Index menampilkan daftar repositori milik user yang sedang login
func (h *RepoHandler) Index(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	repos, err := h.repoStore.FindByUserID(r.Context(), user.UserID)
	if err != nil {
		http.Error(w, "Gagal memuat repositori", http.StatusInternalServerError)
		return
	}

	_ = h.tmpl.ExecuteTemplate(w, "index.html", map[string]any{
		"User":    user,
		"Repos":   repos,
		"Success": r.URL.Query().Get("success"),
		"Error":   r.URL.Query().Get("error"),
	})
}

// ShowCreateForm menampilkan form tambah repo
func (h *RepoHandler) ShowCreateForm(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	_ = h.tmpl.ExecuteTemplate(w, "form.html", map[string]any{
		"User":   user,
		"IsEdit": false,
		"Repo":   model.RepositoryConfig{Branch: "main", IsActive: true},
		"Error":  r.URL.Query().Get("error"),
	})
}

// HandleCreate memproses penambahan repo baru
func (h *RepoHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/repos/new?error=Data+tidak+valid", http.StatusSeeOther)
		return
	}

	repoName := strings.TrimSpace(r.FormValue("repo_name"))
	branch := strings.TrimSpace(r.FormValue("branch"))
	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password"))
	relativePath := strings.TrimSpace(r.FormValue("relative_path"))
	webhookSecret := strings.TrimSpace(r.FormValue("webhook_secret"))
	postCommands := strings.TrimSpace(r.FormValue("post_commands"))
	isActive := r.FormValue("is_active") == "1" || r.FormValue("is_active") == "on"

	if repoName == "" || username == "" || password == "" || relativePath == "" {
		http.Redirect(w, r, "/repos/new?error=Nama+repo,+username,+password/PAT,+dan+relative+path+wajib+diisi", http.StatusSeeOther)
		return
	}
	if branch == "" {
		branch = "main"
	}

	repo := &model.RepositoryConfig{
		UserID:        user.UserID,
		RepoName:      repoName,
		Branch:        branch,
		Username:      username,
		Password:      password,
		RelativePath:  relativePath,
		WebhookSecret: webhookSecret,
		PostCommands:  postCommands,
		IsActive:      isActive,
	}

	if err := h.repoStore.Create(r.Context(), repo); err != nil {
		http.Redirect(w, r, "/repos/new?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/?success=Repository+berhasil+ditambahkan", http.StatusSeeOther)
}

// ShowEditForm menampilkan form edit repo
func (h *RepoHandler) ShowEditForm(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/?error=ID+repository+tidak+valid", http.StatusSeeOther)
		return
	}

	repo, err := h.repoStore.FindByIDAndUserID(r.Context(), id, user.UserID)
	if err != nil || repo == nil {
		http.Redirect(w, r, "/?error=Repository+tidak+ditemukan", http.StatusSeeOther)
		return
	}

	_ = h.tmpl.ExecuteTemplate(w, "form.html", map[string]any{
		"User":   user,
		"IsEdit": true,
		"Repo":   repo,
		"Error":  r.URL.Query().Get("error"),
	})
}

// HandleUpdate memproses perbaruan data repo
func (h *RepoHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=Data+tidak+valid", http.StatusSeeOther)
		return
	}

	idStr := r.FormValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/?error=ID+tidak+valid", http.StatusSeeOther)
		return
	}

	repoName := strings.TrimSpace(r.FormValue("repo_name"))
	branch := strings.TrimSpace(r.FormValue("branch"))
	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password")) // jika kosong, password lama tidak berubah
	relativePath := strings.TrimSpace(r.FormValue("relative_path"))
	webhookSecret := strings.TrimSpace(r.FormValue("webhook_secret"))
	postCommands := strings.TrimSpace(r.FormValue("post_commands"))
	isActive := r.FormValue("is_active") == "1" || r.FormValue("is_active") == "on"

	if repoName == "" || username == "" || relativePath == "" {
		http.Redirect(w, r, fmt.Sprintf("/repos/edit?id=%d&error=Semua+field+wajib+harus+diisi", id), http.StatusSeeOther)
		return
	}
	if branch == "" {
		branch = "main"
	}

	repo := &model.RepositoryConfig{
		ID:            id,
		UserID:        user.UserID,
		RepoName:      repoName,
		Branch:        branch,
		Username:      username,
		Password:      password,
		RelativePath:  relativePath,
		WebhookSecret: webhookSecret,
		PostCommands:  postCommands,
		IsActive:      isActive,
	}

	if err := h.repoStore.Update(r.Context(), repo); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/repos/edit?id=%d&error=%s", id, strings.ReplaceAll(err.Error(), " ", "+")), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/?success=Repository+berhasil+diperbarui", http.StatusSeeOther)
}

// HandleDelete memproses penghapusan repository
func (h *RepoHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=Request+tidak+valid", http.StatusSeeOther)
		return
	}

	idStr := r.FormValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/?error=ID+tidak+valid", http.StatusSeeOther)
		return
	}

	if err := h.repoStore.Delete(r.Context(), id, user.UserID); err != nil {
		http.Redirect(w, r, "/?error=Gagal+menghapus+repository", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/?success=Repository+berhasil+dihapus", http.StatusSeeOther)
}

// HandleTriggerManual memicu proses git pull & post command secara langsung dari web UI
func (h *RepoHandler) HandleTriggerManual(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=Request+tidak+valid", http.StatusSeeOther)
		return
	}

	idStr := r.FormValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/?error=ID+tidak+valid", http.StatusSeeOther)
		return
	}

	repo, err := h.repoStore.FindByIDAndUserID(r.Context(), id, user.UserID)
	if err != nil || repo == nil {
		http.Redirect(w, r, "/?error=Repository+tidak+ditemukan", http.StatusSeeOther)
		return
	}

	// Trigger runner di background
	cloneURL := fmt.Sprintf("https://github.com/%s.git", repo.RepoName)
	go func(cfg model.RepositoryConfig) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		_ = h.runner.Execute(bgCtx, &cfg, cloneURL)
	}(*repo)

	http.Redirect(w, r, fmt.Sprintf("/?success=Trigger+manual+untuk+'%s'+telah+dijalankan+di+latar+belakang", repo.RepoName), http.StatusSeeOther)
}
