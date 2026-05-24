package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// HeaderKey is the HTTP header for request IDs.
const HeaderKey = "X-Request-Id"

// ContextKey is the context key for storing request IDs.
type ContextKey struct{}

// Middleware injects a request ID into each request.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(HeaderKey)
		if requestID == "" {
			requestID = generateID()
		}

		// Set on response
		w.Header().Set(HeaderKey, requestID)

		// Add to context
		ctx := context.WithValue(r.Context(), ContextKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext extracts the request ID from context.
func FromContext(ctx context.Context) string {
	id, ok := ctx.Value(ContextKey{}).(string)
	if !ok {
		return ""
	}
	return id
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
