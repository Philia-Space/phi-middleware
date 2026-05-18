package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWKSAuthConfig holds configuration for RS256 JWT validation via JWKS
type JWKSAuthConfig struct {
	IssuerURL      string
	JWKSEndpoint   string
	ExpectedIssuer string
	Audience       string
	CacheTTL       time.Duration
	SkipPaths      []string
}

// DefaultJWKSAuthConfig returns sensible defaults
func DefaultJWKSAuthConfig() JWKSAuthConfig {
	return JWKSAuthConfig{
		IssuerURL:      "http://localhost:8080",
		JWKSEndpoint:   "/.well-known/jwks.json",
		ExpectedIssuer: "http://localhost:8080",
		Audience:       "philia-space",
		CacheTTL:       5 * time.Minute,
		SkipPaths:      []string{"/health", "/.well-known", "/api/auth/login", "/api/auth/logout"},
	}
}

// Claims represents JWT claims
type Claims struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Name     string   `json:"name"`
	Roles    []string `json:"roles"`
	TokenType string  `json:"type"`
	jwt.RegisteredClaims
}

// AuthJWKS returns a JWKS-based JWT authentication middleware
func AuthJWKS(cfg ...JWKSAuthConfig) func(http.Handler) http.Handler {
	c := DefaultJWKSAuthConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}

	fetcher := NewJWKSFetcher(c.IssuerURL, c.JWKSEndpoint, c.CacheTTL)
	// Pre-fetch JWKS on startup
	go func() {
		fetcher.FetchJWKS()
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for configured paths
			for _, path := range c.SkipPaths {
				if r.URL.Path == path || strings.HasPrefix(r.URL.Path, path+"/") {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Extract Bearer token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"success":false,"error":{"code":"UNAUTHORIZED","message":"missing authorization header"}}`))
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"success":false,"error":{"code":"UNAUTHORIZED","message":"invalid authorization header format"}}`))
				return
			}

			tokenString := parts[1]

			// Parse and validate token
			issuer := c.ExpectedIssuer
			if issuer == "" {
				issuer = c.IssuerURL
			}
			claims, err := validateRS256Token(tokenString, fetcher, issuer, c.Audience)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(fmt.Sprintf(`{"success":false,"error":{"code":"UNAUTHORIZED","message":"%s"}}`, err.Error())))
				return
			}

			// Store claims in context
			ctx := context.WithValue(r.Context(), ContextKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func validateRS256Token(tokenString string, fetcher *JWKSFetcher, issuer, audience string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("unexpected signing method")
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("missing kid in token header")
		}

		pubKey, err := fetcher.GetPublicKey(kid)
		if err != nil {
			// Try to refresh JWKS
			fetcher.FetchJWKS()
			pubKey, err = fetcher.GetPublicKey(kid)
			if err != nil {
				return nil, fmt.Errorf("failed to get public key for kid %s: %w", kid, err)
			}
		}

		return pubKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims type")
	}

	// Validate issuer
	if claims.Issuer != issuer {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", issuer, claims.Issuer)
	}

	// Validate audience
	if len(audience) > 0 {
		audienceMatch := false
		for _, aud := range claims.Audience {
			if aud == audience {
				audienceMatch = true
				break
			}
		}
		if !audienceMatch {
			return nil, fmt.Errorf("invalid audience")
		}
	}

	// Validate token type
	if claims.TokenType != "access" {
		return nil, fmt.Errorf("invalid token type: %s", claims.TokenType)
	}

	return claims, nil
}

// ContextKey is the key for storing user info in request context
type ContextKey struct{}

// GetUserFromContext extracts user claims from request context
func GetUserFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(ContextKey{}).(*Claims)
	return claims, ok
}
