package middleware

import (
	"context"
	"net/http"
	"strings"

	jwtinfra "github.com/go-api-nosql/internal/infrastructure/jwt"
)

type contextKey string

const claimsKey contextKey = "claims"

// Auth returns middleware that validates the Bearer JWT and injects claims into context.
func Auth(provider *jwtinfra.Provider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeJSONError(w, http.StatusUnauthorized, "missing or invalid authorization header")
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := provider.Verify(tokenStr)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext extracts JWT claims from the request context.
func ClaimsFromContext(ctx context.Context) (*jwtinfra.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*jwtinfra.Claims)
	return c, ok
}
