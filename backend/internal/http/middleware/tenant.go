// Package middleware contains HTTP middleware for the api server.
package middleware

import (
	"context"
	"net/http"
)

// userIDKey is an unexported type used as a context key. The unexported type
// prevents collisions with other packages that might use the same string.
type userIDKey struct{}

// SingleUser injects a hardcoded user_id into the request context for every
// request. This is what makes "single-user mode" work: handlers always see a
// user via UserIDFromContext, regardless of whether auth is wired up.
//
// When auth (Clerk, etc.) is added later, this middleware is replaced with one
// that extracts user_id from the validated JWT. Handlers don't change.
func SingleUser(userID int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), userIDKey{}, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext returns the user id injected by a tenant middleware.
// Returns (0, false) if no middleware ran — handlers should treat this as
// an unauthenticated request and respond 401.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(userIDKey{}).(int64)
	return v, ok
}
