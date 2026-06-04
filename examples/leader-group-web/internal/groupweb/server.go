package groupweb

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server exposes group leader election operations as HTTP handlers.
type Server struct {
	elector leader.GroupElector
	member  string
	router  chi.Router
}

// NewServer creates a server for one group election member.
func NewServer(elector leader.GroupElector, member string) *Server {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(middleware.CleanPath)
	router.Use(middleware.Timeout(10 * time.Second))

	server := &Server{elector: elector, member: member, router: router}
	router.Get("/healthz", server.health)
	router.Get("/group", server.group)
	router.Post("/campaign", server.campaign)
	router.Post("/resign", server.resign)
	return server
}

// ServeHTTP dispatches requests to the leader group API.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) group(w http.ResponseWriter, r *http.Request) {
	active, err := s.elector.ActiveCount(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	available, err := s.elector.AvailableSlots(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"active":    active,
		"available": available,
		"is_leader": s.elector.IsLeader(),
		"member":    s.member,
	})
}

func (s *Server) campaign(w http.ResponseWriter, r *http.Request) {
	err := s.elector.Campaign(r.Context())
	switch {
	case err == nil || errors.Is(err, leader.ErrAlreadyLeader):
		writeJSON(w, http.StatusOK, map[string]any{"is_leader": true, "member": s.member})
	default:
		writeError(w, http.StatusServiceUnavailable, err)
	}
}

func (s *Server) resign(w http.ResponseWriter, r *http.Request) {
	if err := s.elector.Resign(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
