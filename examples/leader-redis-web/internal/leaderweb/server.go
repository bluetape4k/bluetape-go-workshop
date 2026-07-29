package leaderweb

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server 는 리더 선출 작업을 HTTP 핸들러로 노출한다.
type Server struct {
	elector leader.Elector
	member  string
	router  chi.Router
}

// NewServer 는 하나의 선출 멤버를 위한 서버를 생성한다.
func NewServer(elector leader.Elector, member string) *Server {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(middleware.CleanPath)
	router.Use(middleware.Timeout(10 * time.Second))

	server := &Server{
		elector: elector,
		member:  member,
		router:  router,
	}

	router.Get("/healthz", server.health)
	router.Get("/leader", server.leader)
	router.Post("/campaign", server.campaign)
	router.Post("/resign", server.resign)

	return server
}

// ServeHTTP 는 요청을 리더 웹 API로 전달한다.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) leader(w http.ResponseWriter, r *http.Request) {
	current, err := s.elector.Leader(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"leader":    current,
		"is_leader": s.elector.IsLeader(),
		"member":    s.member,
	})
}

func (s *Server) campaign(w http.ResponseWriter, r *http.Request) {
	err := s.elector.Campaign(r.Context())
	switch {
	case err == nil || errors.Is(err, leader.ErrAlreadyLeader):
		writeJSON(w, http.StatusOK, map[string]any{
			"is_leader": true,
			"member":    s.member,
		})
	case errors.Is(err, leader.ErrNotLeader):
		writeError(w, http.StatusConflict, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
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
