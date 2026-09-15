package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"qinci/internal/core/domain"
	"qinci/internal/core/ports"
)

type RepoHandler struct {
	repoUsecase ports.RepositoryUsecase
	tmpl        *template.Template
}

func NewRepoHandler(repoUsecase ports.RepositoryUsecase, tmpl *template.Template) *RepoHandler {
	return &RepoHandler{
		repoUsecase: repoUsecase,
		tmpl:        tmpl,
	}
}

func (h *RepoHandler) Index(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	repos, err := h.repoUsecase.ListUserRepositories(r.Context(), user.UserID)
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

func (h *RepoHandler) ShowCreateForm(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	_ = h.tmpl.ExecuteTemplate(w, "form.html", map[string]any{
		"User":   user,
		"IsEdit": false,
		"Repo":   domain.RepositoryConfig{Branch: "main", IsActive: true},
		"Error":  r.URL.Query().Get("error"),
	})
}

func (h *RepoHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/repos/new?error=Data+tidak+valid", http.StatusSeeOther)
		return
	}

	repo := &domain.RepositoryConfig{
		UserID:        user.UserID,
		RepoName:      r.FormValue("repo_name"),
		Branch:        r.FormValue("branch"),
		Username:      r.FormValue("username"),
		Password:      r.FormValue("password"),
		RelativePath:  r.FormValue("relative_path"),
		WebhookSecret: r.FormValue("webhook_secret"),
		PostCommands:  r.FormValue("post_commands"),
		IsActive:      r.FormValue("is_active") == "1" || r.FormValue("is_active") == "on",
	}

	if err := h.repoUsecase.CreateRepository(r.Context(), repo); err != nil {
		http.Redirect(w, r, "/repos/new?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/?success=Repository+berhasil+ditambahkan", http.StatusSeeOther)
}

func (h *RepoHandler) ShowEditForm(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/?error=ID+repository+tidak+valid", http.StatusSeeOther)
		return
	}

	repo, err := h.repoUsecase.GetRepository(r.Context(), id, user.UserID)
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

func (h *RepoHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=Data+tidak+valid", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/?error=ID+tidak+valid", http.StatusSeeOther)
		return
	}

	repo := &domain.RepositoryConfig{
		ID:            id,
		UserID:        user.UserID,
		RepoName:      r.FormValue("repo_name"),
		Branch:        r.FormValue("branch"),
		Username:      r.FormValue("username"),
		Password:      r.FormValue("password"),
		RelativePath:  r.FormValue("relative_path"),
		WebhookSecret: r.FormValue("webhook_secret"),
		PostCommands:  r.FormValue("post_commands"),
		IsActive:      r.FormValue("is_active") == "1" || r.FormValue("is_active") == "on",
	}

	if err := h.repoUsecase.UpdateRepository(r.Context(), repo); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/repos/edit?id=%d&error=%s", id, strings.ReplaceAll(err.Error(), " ", "+")), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/?success=Repository+berhasil+diperbarui", http.StatusSeeOther)
}

func (h *RepoHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=Request+tidak+valid", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/?error=ID+tidak+valid", http.StatusSeeOther)
		return
	}

	if err := h.repoUsecase.DeleteRepository(r.Context(), id, user.UserID); err != nil {
		http.Redirect(w, r, "/?error=Gagal+menghapus+repository", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/?success=Repository+berhasil+dihapus", http.StatusSeeOther)
}

func (h *RepoHandler) HandleTriggerManual(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=Request+tidak+valid", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/?error=ID+tidak+valid", http.StatusSeeOther)
		return
	}

	repo, err := h.repoUsecase.TriggerManual(r.Context(), id, user.UserID)
	if err != nil {
		http.Redirect(w, r, "/?error=Repository+tidak+ditemukan", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/?success=Trigger+manual+untuk+'%s'+telah+dijalankan+di+latar+belakang", repo.RepoName), http.StatusSeeOther)
}

func (h *RepoHandler) ShowLogs(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/?error=ID+repository+tidak+valid", http.StatusSeeOther)
		return
	}

	repo, logs, err := h.repoUsecase.GetRepositoryLogs(r.Context(), id, user.UserID, 50)
	if err != nil {
		http.Redirect(w, r, "/?error=Repository+tidak+ditemukan", http.StatusSeeOther)
		return
	}

	_ = h.tmpl.ExecuteTemplate(w, "logs.html", map[string]any{
		"User": user,
		"Repo": repo,
		"Logs": logs,
	})
}
