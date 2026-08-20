package web

import (
	_ "embed"
	"fmt"
	"net/http"
	"time"

	"github.com/mp-core/panel/config"
)

//go:embed html/login.html
var loginHTML []byte

//go:embed html/panel.html
var panelHTML []byte

// Server is the MP-CORE HTTP server.
type Server struct {
	cfg    *config.Config
	router *http.ServeMux
}

// NewServer builds the server and wires all routes.
func NewServer(cfg *config.Config) *Server {
	s := &Server{cfg: cfg}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	mux := http.NewServeMux()

	// Public auth routes
	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.loginSubmit)
	mux.HandleFunc("GET /logout", s.logout)

	// Protected routes (require authentication)
	mux.HandleFunc("GET /{$}", s.requireAuth(s.dashboard))
	mux.HandleFunc("GET /api/status", s.requireAuth(s.apiStatus))

	s.router = mux
}

// Start runs the HTTP server.
func (s *Server) Start() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.cfg.Port),
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return srv.ListenAndServe()
}

// dashboard serves the authenticated panel UI.
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(panelHTML)
}

// apiStatus is a simple health/status endpoint.
func (s *Server) apiStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"running","version":"3.1.0"}`))
}
