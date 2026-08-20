package web

import (
	_ "embed"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/crypt0mp73/mp-core/config"
)

//go:embed html/login.html
var loginHTML []byte

//go:embed html/panel.html
var panelHTML []byte

type Server struct {
	cfg     *config.Config
	version string
	router  *http.ServeMux
}

func NewServer(cfg *config.Config, version string) *Server {
	s := &Server{cfg: cfg, version: version}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	mux := http.NewServeMux()

	base := strings.TrimRight(s.cfg.BasePath, "/")
	if base == "" {
		base = ""
	}

	// Public
	mux.HandleFunc("GET "+base+"/login", s.loginPage)
	mux.HandleFunc("POST "+base+"/login", s.loginSubmit)
	mux.HandleFunc("GET "+base+"/logout", s.logout)

	// Protected
	mux.HandleFunc("GET "+base+"/", s.requireAuth(s.dashboard))
	mux.HandleFunc("GET "+base+"/api/stats", s.requireAuth(s.apiStats))
	mux.HandleFunc("GET "+base+"/api/version", s.requireAuth(s.apiVersion))

	// Inbounds
	mux.HandleFunc("GET "+base+"/api/inbounds", s.requireAuth(s.listInbounds))
	mux.HandleFunc("POST "+base+"/api/inbounds", s.requireAuth(s.createInbound))
	mux.HandleFunc("PUT "+base+"/api/inbounds", s.requireAuth(s.updateInbound))
	mux.HandleFunc("DELETE "+base+"/api/inbounds", s.requireAuth(s.deleteInbound))

	// Clients
	mux.HandleFunc("GET "+base+"/api/clients", s.requireAuth(s.listClients))
	mux.HandleFunc("POST "+base+"/api/clients", s.requireAuth(s.createClient))
	mux.HandleFunc("PUT "+base+"/api/clients", s.requireAuth(s.updateClient))
	mux.HandleFunc("DELETE "+base+"/api/clients", s.requireAuth(s.deleteClient))
	mux.HandleFunc("POST "+base+"/api/clients/toggle", s.requireAuth(s.toggleClient))
	mux.HandleFunc("POST "+base+"/api/clients/reset-traffic", s.requireAuth(s.resetClientTraffic))

	// Routing
	mux.HandleFunc("GET "+base+"/api/routing", s.requireAuth(s.listRouting))
	mux.HandleFunc("POST "+base+"/api/routing", s.requireAuth(s.createRouting))
	mux.HandleFunc("PUT "+base+"/api/routing", s.requireAuth(s.updateRouting))
	mux.HandleFunc("DELETE "+base+"/api/routing", s.requireAuth(s.deleteRouting))

	// Outbounds
	mux.HandleFunc("GET "+base+"/api/outbounds", s.requireAuth(s.listOutbounds))
	mux.HandleFunc("POST "+base+"/api/outbounds", s.requireAuth(s.createOutbound))
	mux.HandleFunc("PUT "+base+"/api/outbounds", s.requireAuth(s.updateOutbound))
	mux.HandleFunc("DELETE "+base+"/api/outbounds", s.requireAuth(s.deleteOutbound))

	// Settings
	mux.HandleFunc("GET "+base+"/api/settings", s.requireAuth(s.getSettings))
	mux.HandleFunc("POST "+base+"/api/settings", s.requireAuth(s.saveSettings))

	// Services
	mux.HandleFunc("POST "+base+"/api/tor/rotate", s.requireAuth(s.torRotate))
	mux.HandleFunc("GET "+base+"/api/tor/status", s.requireAuth(s.torStatus))
	mux.HandleFunc("POST "+base+"/api/ssh/toggle", s.requireAuth(s.sshToggle))
	mux.HandleFunc("POST "+base+"/api/masterdns/generate", s.requireAuth(s.masterdnsGenerate))
	mux.HandleFunc("POST "+base+"/api/armor/run", s.requireAuth(s.armorRun))
	mux.HandleFunc("GET "+base+"/api/armor/status", s.requireAuth(s.armorStatus))

	// Certificates
	mux.HandleFunc("POST "+base+"/api/certificates/issue", s.requireAuth(s.issueCert))

	// Reboot
	mux.HandleFunc("POST "+base+"/api/system/reboot", s.requireAuth(s.systemReboot))

	// Subscription link generation
	mux.HandleFunc("GET "+base+"/sub/", s.serveSubscription)

	s.router = mux
}

func (s *Server) Start() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.cfg.Port),
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return srv.ListenAndServe()
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(panelHTML)
}

func (s *Server) apiVersion(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, map[string]string{"version": s.version})
}
