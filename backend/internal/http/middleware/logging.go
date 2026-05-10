package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// requestIDKey is the unexported context key for the per-request correlation ID.
type requestIDKey struct{}

// RequestLogger threads chi's request ID into the context (so downstream code
// and logs can pick it up via RequestIDFromContext) and injects a per-request
// slog.Logger that carries request_id automatically on every log call.
//
// Use it AFTER chi's RequestID middleware in the chain so the ID is available.
func RequestLogger(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := middleware.GetReqID(r.Context())
			logger := base.With("request_id", rid, "method", r.Method, "path", r.URL.Path)

			ctx := context.WithValue(r.Context(), requestIDKey{}, rid)
			ctx = NewLoggerContext(ctx, logger)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// loggerKey is the context key for the per-request logger.
type loggerKey struct{}

// NewLoggerContext returns a new context with the given logger attached.
// Exposed so tests and non-HTTP callers can construct contexts with loggers.
func NewLoggerContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// LoggerFromContext returns the request-scoped logger if one was attached,
// otherwise slog.Default(). Always safe to call; never returns nil.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// RequestIDFromContext returns the request id threaded by RequestLogger.
// Returns empty string if no middleware ran.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}
