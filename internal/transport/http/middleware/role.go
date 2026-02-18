package middleware

import (
	"net/http"
	"strings"
)

// RequireRole returns middleware that checks the role's access list.
// allowedControllers is the list of controller names this role may access.
// An empty allowList means the role has unrestricted access.
func RequireRole(allowedControllers []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowedControllers) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			// Derive controller name from first path segment after /v1/
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/"), "/")
			if len(parts) == 0 {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			controller := parts[0]
			for _, a := range allowedControllers {
				if a == controller {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		})
	}
}
