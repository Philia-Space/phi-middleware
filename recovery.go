package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/philiaspace/phi-core/observability"
)

// Recovery returns a panic recovery middleware.
func Recovery(logger observability.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					if logger != nil {
						logger.Error(r.Context(), "panic recovered",
							"method", r.Method,
							"path", r.URL.Path,
							"error", err,
							"stack", string(debug.Stack()),
						)
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"success":false,"error":{"code":"INTERNAL_ERROR","message":"internal server error"}}`))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
