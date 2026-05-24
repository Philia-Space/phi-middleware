package middleware

import (
	"net/http"
	"time"

	"github.com/philiaspace/phi-core/observability"
)

// Logger returns a request logging middleware.
func Logger(logger observability.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			if logger != nil {
				logger.Info(r.Context(), "request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", rw.statusCode,
					"duration", duration,
					"remote_addr", r.RemoteAddr,
					"user_agent", r.UserAgent(),
				)
			}
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
