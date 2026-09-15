package session

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const CookieName = "qinci_session"

type SessionData struct {
	UserID    int64
	Username  string
	FullName  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]SessionData
	duration time.Duration
}

func NewSessionManager(duration time.Duration) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]SessionData),
		duration: duration,
	}

	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		for range ticker.C {
			sm.cleanExpired()
		}
	}()

	return sm
}

func (sm *SessionManager) CreateSession(w http.ResponseWriter, userID int64, username, fullName string) string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)

	now := time.Now()
	expires := now.Add(sm.duration)

	sm.mu.Lock()
	sm.sessions[token] = SessionData{
		UserID:    userID,
		Username:  username,
		FullName:  fullName,
		CreatedAt: now,
		ExpiresAt: expires,
	}
	sm.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return token
}

func (sm *SessionManager) GetSession(r *http.Request) (*SessionData, bool) {
	cookie, err := r.Cookie(CookieName)
	if err != nil || cookie.Value == "" {
		return nil, false
	}

	sm.mu.RLock()
	data, exists := sm.sessions[cookie.Value]
	sm.mu.RUnlock()

	if !exists {
		return nil, false
	}

	if time.Now().After(data.ExpiresAt) {
		sm.DestroySession(nil, r)
		return nil, false
	}

	return &data, true
}

func (sm *SessionManager) DestroySession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(CookieName)
	if err == nil && cookie.Value != "" {
		sm.mu.Lock()
		delete(sm.sessions, cookie.Value)
		sm.mu.Unlock()
	}

	if w != nil {
		http.SetCookie(w, &http.Cookie{
			Name:     CookieName,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
		})
	}
}

func (sm *SessionManager) cleanExpired() {
	now := time.Now()
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for token, sess := range sm.sessions {
		if now.After(sess.ExpiresAt) {
			delete(sm.sessions, token)
		}
	}
}
