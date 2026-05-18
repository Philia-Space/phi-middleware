package middleware

import (
	"net/http"
)

// Chain chains multiple middleware together.
func Chain(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// ChainFunc chains middleware for http.HandlerFunc.
func ChainFunc(handler http.HandlerFunc, middlewares ...func(http.Handler) http.Handler) http.Handler {
	return Chain(handler, middlewares...)
}
