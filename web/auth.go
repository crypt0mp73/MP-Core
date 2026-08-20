package web

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/crypt0mp73/mp-core/database"
)

var (
	sessions  = make(map[string]time.Time)
	sessionMu sync.RWMutex
)

const sessionCookie = "mp_session"

func newSessionToken() string {
	b := make([]byte, 32)
	rand.Read(b)
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
		http.Redirect(w, r, s.cfg.BasePath+"login?error=1", http.StatusSeeOther)
		return
	}

	token := newSessionToken()
	sessionMu.Lock()
	sessions[token] = time.Now().Add(24 * time.Hour)
	sessionMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     s.cfg.BasePath,
		HttpOnly: true,
		MaxAge:   86400,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, s.cfg.BasePath, http.StatusSeeOther)
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
		Path:   s.cfg.BasePath,
		MaxAge: -1,
	})
	http.Redirect(w, r, s.cfg.BasePath+"login", http.StatusSeeOther)
}

func (s *Server) isAuthenticated(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	sessionMu.RLock()
	exp, ok := sessions[c.Value]
	sessionMu.RUnlock()
	return ok && time.Now().Before(exp)
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.isAuthenticated(r) {
			http.Redirect(w, r, s.cfg.BasePath+"login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}
