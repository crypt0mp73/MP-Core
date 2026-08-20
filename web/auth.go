package web

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/mp-core/panel/database"
)

// In-memory session store (upgraded to DB-backed sessions in Milestone 2).
var (
	sessions  = make(map[string]time.Time)
	sessionMu sync.RWMutex
)

const sessionCookie = "mp_session"

func newSessionToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(loginHTML)
}

func (s *Server) loginSubmit(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	ok, err := database.CheckLogin(username, password)
	if err != nil || !ok {
		http.Redirect(w, r, "/login?error=1", http.StatusSeeOther)
		return
	}

	token := newSessionToken()
	if token == "" {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sessionMu.Lock()
	sessions[token] = time.Now().Add(24 * time.Hour)
	sessionMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		sessionMu.Lock()
		delete(sessions, c.Value)
		sessionMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookie,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) isAuthenticated(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	sessionMu.RLock()
	exp, ok := sessions[c.Value]
	sessionMu.RUnlock()
	if !ok {
		return false
	}
	return time.Now().Before(exp)
}

// requireAuth wraps a handler, redirecting to /login if unauthenticated.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.isAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}
