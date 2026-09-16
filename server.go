package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
)

type Server struct {
	cfg      *Config
	docker   *Docker
	upgrader websocket.Upgrader
}

func newServer(cfg *Config, docker *Docker) *Server {
	s := &Server{cfg: cfg, docker: docker}
	s.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 4096,
		// Same-origin only: we never accept cross-site websocket upgrades.
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // non-browser client
			}
			return s.clientAllowed(r.RemoteAddr)
		},
	}
	return s
}

// handler wires all routes. The LAN guard wraps everything, including the
// websocket endpoint, so the dashboard stays reachable from trusted subnets only.
func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/system", s.handleSystem)
	mux.HandleFunc("GET /api/build", s.handleBuild)
	mux.HandleFunc("GET /api/containers", s.handleListContainers)

	actions := []string{"start", "stop", "restart", "pause", "unpause"}
	for _, action := range actions {
		mux.HandleFunc("POST /api/containers/{id}/"+action, s.makeControlHandler(action))
	}
	mux.HandleFunc("POST /api/containers/{id}/remove", s.handleRemove)

	mux.HandleFunc("GET /ws/logs", s.handleLogsWS)

	ui, err := fs.Sub(uiFS, "static")
	if err != nil {
		panic(err) // static/ is embedded at build time, so this cannot happen
	}
	// The embedded files have a fixed modification time (go:embed), so the
	// browser would otherwise heuristically cache style.css/app.js forever
	// and never see a rebuilt theme. Force revalidation on every load.
	mux.Handle("GET /", noCache(http.FileServerFS(ui)))

	return s.lanGuard(mux)
}

// noCache tells clients to revalidate every request, so a rebuilt binary's
// embedded UI is always picked up without a manual hard refresh.
func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

// lanGuard rejects requests whose source address is not in an allowed subnet.
// Defaults to private/LAN ranges, so the dashboard cannot be reached from the
// public internet even if it were port-forwarded or bound to a public IP.
func (s *Server) lanGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.clientAllowed(r.RemoteAddr) {
			log.Printf("blocked request from %s to %s", r.RemoteAddr, r.URL.Path)
			http.Error(w, "forbidden: outside allowed local network", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientAllowed reports whether a peer address passes the subnet guard.
func (s *Server) clientAllowed(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && s.cfg.allowed(ip)
}

func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.ListTimeout)
	defer cancel()
	info, err := s.docker.EngineInfo(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleBuild(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": version, "build": buildTime})
}

func (s *Server) handleListContainers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.ListTimeout)
	defer cancel()
	rows, err := s.docker.ListContainers(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) makeControlHandler(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		ctx, cancel := context.WithTimeout(r.Context(), s.cfg.ListTimeout)
		defer cancel()
		if err := s.docker.Control(ctx, id, action, s.cfg.StopTimeout); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"ok": action})
	}
}

func (s *Server) handleRemove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	force, _ := strconv.ParseBool(r.URL.Query().Get("force"))
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.ListTimeout)
	defer cancel()
	if err := s.docker.Remove(ctx, id, force); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"ok": "remove"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	msg := err.Error()
	switch {
	case strings.Contains(msg, "No such container") || strings.Contains(msg, "not found"):
		status = http.StatusNotFound
	case strings.Contains(msg, "is not running") || strings.Contains(msg, "is paused"):
		status = http.StatusConflict
	case strings.Contains(msg, "is protected"):
		status = http.StatusForbidden
	}
	log.Printf("api error: %v", err)
	writeJSON(w, status, map[string]string{"error": msg})
}
