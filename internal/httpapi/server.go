package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/state"
	appstatus "git.tai.pe/homelab/mikrotik-adguard-identity/internal/status"
)

type Server struct {
	Address         string
	State           *state.State
	Status          *appstatus.Tracker
	KidControlToken string
}

type statusResponse struct {
	Status string `json:"status"`
	Radius struct {
		Users    int `json:"users"`
		Sessions int `json:"sessions"`
		NAS      int `json:"nas"`
	} `json:"radius"`
	Identity struct {
		MappedIPs        int `json:"mapped_ips"`
		UnmappedSessions int `json:"unmapped_sessions"`
		DHCPLeases       int `json:"dhcp_leases"`
	} `json:"identity"`
	AdGuard struct {
		Status    string `json:"status"`
		LastSync  string `json:"last_sync,omitempty"`
		LastError string `json:"last_error,omitempty"`
	} `json:"adguard"`
	RouterOS struct {
		Status        string `json:"status"`
		LastReconcile string `json:"last_reconcile,omitempty"`
		LastError     string `json:"last_error,omitempty"`
	} `json:"routeros"`
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /api/status", s.apiStatus)
	mux.HandleFunc("GET /api/kid-control", s.apiKidControl)

	srv := &http.Server{
		Addr:              s.Address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	log.Printf("HTTP status API: http://%s", s.Address)
	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	snap := s.Status.Snapshot()
	state := "ok"
	code := http.StatusOK
	if snap.RouterOS.Status == "error" || snap.AdGuard.Status == "error" {
		state = "degraded"
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]string{"status": state})
}

func (s *Server) apiStatus(w http.ResponseWriter, _ *http.Request) {
	stats := s.State.Stats()
	snap := s.Status.Snapshot()
	overall := "ok"
	if snap.RouterOS.Status == "error" || snap.AdGuard.Status == "error" {
		overall = "degraded"
	}

	var out statusResponse
	out.Status = overall
	out.Radius.Users = stats.Users
	out.Radius.Sessions = stats.Sessions
	out.Radius.NAS = stats.NAS
	out.Identity.MappedIPs = stats.MappedIPs
	out.Identity.UnmappedSessions = stats.UnmappedSessions
	out.Identity.DHCPLeases = stats.DHCPLeases
	out.AdGuard.Status = snap.AdGuard.Status
	if snap.AdGuard.Status == "error" {
		out.AdGuard.LastError = "sync failed"
	}
	if !snap.AdGuard.Last.IsZero() {
		out.AdGuard.LastSync = snap.AdGuard.Last.Format(time.RFC3339)
	}
	out.RouterOS.Status = snap.RouterOS.Status
	if snap.RouterOS.Status == "error" {
		out.RouterOS.LastError = "reconcile failed"
	}
	if !snap.RouterOS.Last.IsZero() {
		out.RouterOS.LastReconcile = snap.RouterOS.Last.Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiKidControl(w http.ResponseWriter, r *http.Request) {
	// The endpoint contains usernames and MAC addresses, unlike /api/status.
	// Keep it disabled unless an explicit bearer token is configured.
	if s.KidControlToken == "" {
		http.NotFound(w, r)
		return
	}

	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if len(auth) <= len(prefix) || auth[:len(prefix)] != prefix ||
		subtle.ConstantTimeCompare([]byte(auth[len(prefix):]), []byte(s.KidControlToken)) != 1 {
		w.Header().Set("WWW-Authenticate", `Bearer realm="kid-control"`)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Devices []state.KidControlDevice `json:"devices"`
	}{Devices: s.State.KidControlDevices()})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
