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

// Services is the bundle of service-layer dependencies the router wires up.
// Defined as a struct so the constructor signature doesn't grow each time we
// add a service.
type Services struct {
	Tracker *service.TrackerService
	Visas   *service.VisaService
	Tasks   *service.TaskService
}

// NewRouter builds the API router. The returned http.Handler can be passed to
// http.ListenAndServe or wrapped further (e.g. for TLS in production).
//
// pinger is the database pinger used by /healthz; pass the *pgxpool.Pool.
func NewRouter(cfg *config.Config, logger *slog.Logger, svcs Services, pinger handlers.HealthPinger) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.RequestLogger(logger))     // attaches request-scoped slog with request_id
	r.Use(middleware.SlogRequestLogger(logger)) // emits one "request completed" line per request
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30_000_000_000)) // 30s — tighten when we know real p95s

	// Tenant middleware: in single-user mode, hardcodes user_id. Replace with
	// a JWT-extracting middleware once auth is wired up.
	if cfg.SingleUserMode {
		r.Use(middleware.SingleUser(cfg.SingleUserID))
	}

	// /healthz pings the database so cloud readiness probes catch real outages,
	// not just "the process is up". Lives outside /api/v1 by convention.
	r.Get("/healthz", handlers.Healthz(pinger))

	events := &handlers.EventsHandler{Tracker: svcs.Tracker}
	visas := &handlers.VisasHandler{Visas: svcs.Visas}
	tasks := &handlers.TasksHandler{Tasks: svcs.Tasks}

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/visas", visas.Create)
		r.Get("/visas/active", visas.GetActive)
		r.Get("/visa-types", visas.ListTypes)

		r.Post("/life-events", events.Create)

		r.Get("/tasks", tasks.List)
		r.Post("/tasks/{id}/done", tasks.MarkDone)
	})

	return r
}
