package web

import (
	"context"
	"net/http"

	"qinci/internal/adapter/outbound/session"
)

type contextKey string

const UserContextKey contextKey = "authenticated_user"

func AuthMiddleware(sm *session.SessionManager, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := sm.GetSession(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, sess)
		next(w, r.WithContext(ctx))
	}
}

func GetUserFromContext(r *http.Request) *session.SessionData {
	if val := r.Context().Value(UserContextKey); val != nil {
		if sess, ok := val.(*session.SessionData); ok {
			return sess
		}
	}
	return nil
}
