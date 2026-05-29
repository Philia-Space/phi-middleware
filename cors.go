package middleware

import (
	"log"
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSConfig returns sensible defaults.
//
// Per CORS specification (Fetch Standard §6.2), wildcard origin ("*") MUST NOT
// be combined with Access-Control-Allow-Credentials: true. Browsers will reject
// such responses. Therefore AllowCredentials defaults to false. Services that
// need credentials must provide an explicit list of AllowedOrigins.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With"},
		ExposedHeaders:   []string{"X-Total-Count", "X-Page", "X-Per-Page"},
		AllowCredentials: false,
		MaxAge:           86400,
	}
}

// CORS returns a CORS middleware handler.
func CORS(cfg ...CORSConfig) func(http.Handler) http.Handler {
	c := DefaultCORSConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}

	// Safety check: wildcard origin + credentials is forbidden by CORS spec.
	// If both are set, disable credentials and warn.
	if c.AllowCredentials {
		for _, o := range c.AllowedOrigins {
			if o == "*" {
				log.Printf("[CORS WARNING] AllowCredentials=true with wildcard origin \"*\" is forbidden by CORS spec. Disabling AllowCredentials. Provide explicit AllowedOrigins to use credentials.")
				c.AllowCredentials = false
				break
			}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if c.AllowCredentials {
				// When credentials are enabled, we MUST echo back the exact origin
				// (never "*") per CORS spec. Only set the header if origin is in the
				// explicit allow-list.
				for _, o := range c.AllowedOrigins {
					if o == origin {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						w.Header().Set("Vary", "Origin")
						break
					}
				}
			} else {
				// No credentials — safe to use wildcard.
				if origin != "" {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				}
			}

			w.Header().Set("Access-Control-Allow-Methods", strings.Join(c.AllowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(c.AllowedHeaders, ", "))
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(c.ExposedHeaders, ", "))
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(c.MaxAge))

			if c.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
