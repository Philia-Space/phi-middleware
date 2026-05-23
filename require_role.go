package middleware

import (
	"encoding/json"
	"net/http"
)

// RequireRole returns a middleware that checks if the authenticated user has at least one of the required roles.
// It must be applied AFTER AuthJWKS middleware so that claims are present in the context.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetUserFromContext(r.Context())
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error": map[string]string{
						"code":    "UNAUTHORIZED",
						"message": "authentication required",
					},
				})
				return
			}

			// Check if user has any of the required roles
			hasRole := false
			for _, required := range roles {
				for _, userRole := range claims.Roles {
					if userRole == required {
						hasRole = true
						break
					}
				}
				if hasRole {
					break
				}
			}

			if !hasRole {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error": map[string]string{
						"code":    "FORBIDDEN",
						"message": "insufficient permissions",
					},
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
