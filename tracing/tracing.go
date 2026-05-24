package tracing

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Span represents a simple trace span.
type Span struct {
	TraceID   string
	SpanID    string
	Operation string
	StartTime time.Time
}

// ContextKey is the context key for storing trace information.
type ContextKey struct{}

// Middleware adds basic trace context to each request.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-Id")
		if traceID == "" {
			traceID = generateTraceID()
		}

		span := &Span{
			TraceID:   traceID,
			SpanID:      generateSpanID(),
			Operation:   fmt.Sprintf("%s %s", r.Method, r.URL.Path),
			StartTime:   time.Now(),
		}

		w.Header().Set("X-Trace-Id", traceID)

		ctx := context.WithValue(r.Context(), ContextKey{}, span)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext extracts the trace span from context.
func FromContext(ctx context.Context) (*Span, bool) {
	span, ok := ctx.Value(ContextKey{}).(*Span)
	return span, ok
}

// Duration returns how long the span has been active.
func (s *Span) Duration() time.Duration {
	return time.Since(s.StartTime)
}

func generateTraceID() string {
	return fmt.Sprintf("trace-%d", time.Now().UnixNano())
}

func generateSpanID() string {
	return fmt.Sprintf("span-%d", time.Now().UnixNano())
}
