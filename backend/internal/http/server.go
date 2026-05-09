// Package httpapi wires the Chi router to the HTTP handlers.
//
// Imported under the name `httpapi` rather than `http` to avoid shadowing the
// stdlib `net/http` at every call site that imports both.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/khamseaffan/japan-concierge/backend/internal/config"
	"github.com/khamseaffan/japan-concierge/backend/internal/http/handlers"
	"github.com/khamseaffan/japan-concierge/backend/internal/http/middleware"
	"github.com/khamseaffan/japan-concierge/backend/internal/service"
)

// NewRouter builds the API router. The returned http.Handler can be passed to
// http.ListenAndServe or wrapped further (e.g. for TLS in production).
func NewRouter(cfg *config.Config, logger *slog.Logger, tracker *service.TrackerService) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30_000_000_000)) // 30s — tighten when we know real p95s

	// Tenant middleware: in single-user mode, hardcodes user_id. Replace with
	// a JWT-extracting middleware once auth is wired up.
	if cfg.SingleUserMode {
		r.Use(middleware.SingleUser(cfg.SingleUserID))
	}

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	events := &handlers.EventsHandler{Tracker: tracker, Logger: logger}

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/life-events", events.Create)
	})

	return r
}
