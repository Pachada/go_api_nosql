package middleware

import (
	"context"
	"net/http"
	"strings"

	jwtinfra "github.com/go-api-nosql/internal/infrastructure/jwt"
)

type contextKey string

const ClaimsKey contextKey = "claims"

// Auth returns middleware that validates the Bearer JWT and injects claims into context.
func Auth(provider *jwtinfra.Provider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"error":"missing or invalid authorization header"}`, http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := provider.Verify(tokenStr)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext extracts JWT claims from the request context.
func ClaimsFromContext(ctx context.Context) (*jwtinfra.Claims, bool) {
	c, ok := ctx.Value(ClaimsKey).(*jwtinfra.Claims)
	return c, ok
}
