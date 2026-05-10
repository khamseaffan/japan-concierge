package handlers

import (
	"context"
	"net/http"
	"time"
)

// HealthPinger is anything that can ping a backend dependency. *pgxpool.Pool
// satisfies this. Defined as an interface so the handler doesn't depend on
// the concrete pgx type and so tests can pass a fake.
type HealthPinger interface {
	Ping(ctx context.Context) error
}

// Healthz returns an http.HandlerFunc that pings the database with a short
// timeout. Returns 200 if reachable, 503 otherwise.
//
// Cloud platforms (Vercel, Railway, Fly, k8s) call this to gate traffic and
// to detect when a deploy has failed. It must not return 200 if the database
// is down — that would tell the platform "I'm healthy" while every real
// request is failing.
func Healthz(pool HealthPinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unhealthy",
				"reason": "database unreachable",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
