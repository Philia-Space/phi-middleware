package middleware

import (
	"context"
	"net/http"
	"strings"
)

// ContextKey is the key for storing user info in request context.
type ContextKey struct{}

// UserInfo holds extracted user information from JWT.
type UserInfo struct {
	UserID string
	Roles  []string
}

// JWTConfig holds JWT validation configuration.
type JWTConfig struct {
	Secret      string
	SkipPaths   []string
	HeaderName  string
	TokenPrefix string
}

// DefaultJWTConfig returns sensible defaults.
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		Secret:      "dev-secret-change-in-production",
		SkipPaths:   []string{"/health", "/swagger"},
		HeaderName:  "Authorization",
		TokenPrefix: "Bearer ",
	}
}

// AuthJWT returns a JWT authentication middleware.
func AuthJWT(cfg ...JWTConfig) func(http.Handler) http.Handler {
	c := DefaultJWTConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for configured paths
			for _, path := range c.SkipPaths {
				if r.URL.Path == path || strings.HasPrefix(r.URL.Path, path+"/") {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Extract token
			authHeader := r.Header.Get(c.HeaderName)
			if authHeader == "" {
				http.Error(w, `{"success":false,"error":{"code":"UNAUTHORIZED","message":"missing authorization header"}}`, http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(authHeader, c.TokenPrefix)
			if token == authHeader {
				http.Error(w, `{"success":false,"error":{"code":"UNAUTHORIZED","message":"invalid authorization format"}}`, http.StatusUnauthorized)
				return
			}

			// TODO: Implement actual JWT validation
			// For now, pass through with a placeholder user
			user := &UserInfo{
				UserID: "placeholder-user",
				Roles:  []string{"member"},
			}

			ctx := context.WithValue(r.Context(), ContextKey{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext extracts user info from request context.
func GetUserFromContext(ctx context.Context) (*UserInfo, bool) {
	user, ok := ctx.Value(ContextKey{}).(*UserInfo)
	return user, ok
}
