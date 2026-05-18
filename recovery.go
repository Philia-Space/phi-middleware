package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery returns a panic recovery middleware.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					if logger != nil {
						logger.Error("panic recovered",
							slog.String("method", r.Method),
							slog.String("path", r.URL.Path),
							slog.Any("error", err),
							slog.String("stack", string(debug.Stack())),
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
