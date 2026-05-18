package middleware

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// JWK represents a JSON Web Key
type JWK struct {
	KeyType   string `json:"kty"`
	Use       string `json:"use"`
	KeyID     string `json:"kid"`
	Algorithm string `json:"alg"`
	N         string `json:"n"`
	E         string `json:"e"`
}

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWKSFetcher handles JWKS fetching and caching
type JWKSFetcher struct {
	issuerURL    string
	jwksEndpoint string
	cacheTTL     time.Duration
	httpClient   *http.Client
	cache        *JWKSCache
	mu           sync.RWMutex
}

// JWKSCache holds cached public keys and expiry
type JWKSCache struct {
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
	mu        sync.RWMutex
}

// NewJWKSFetcher creates a new JWKS fetcher
func NewJWKSFetcher(issuerURL, jwksEndpoint string, cacheTTL time.Duration) *JWKSFetcher {
	return &JWKSFetcher{
		issuerURL:    issuerURL,
		jwksEndpoint: jwksEndpoint,
		cacheTTL:     cacheTTL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		cache: &JWKSCache{
			keys: make(map[string]*rsa.PublicKey),
		},
	}
}

// FetchJWKS fetches the JWKS from the issuer, using cache if available
func (f *JWKSFetcher) FetchJWKS() (*JWKS, error) {
	f.cache.mu.RLock()
	if time.Now().Before(f.cache.expiresAt) {
		defer f.cache.mu.RUnlock()
		return f.buildCachedJWKS(), nil
	}
	f.cache.mu.RUnlock()

	f.mu.Lock()
	defer f.mu.Unlock()

	f.cache.mu.RLock()
	if time.Now().Before(f.cache.expiresAt) {
		defer f.cache.mu.RUnlock()
		return f.buildCachedJWKS(), nil
	}
	f.cache.mu.RUnlock()

	url := f.issuerURL + f.jwksEndpoint
	log.Printf("Fetching JWKS from: %s", url)

	resp, err := f.httpClient.Get(url)
	if err != nil {
		if len(f.cache.keys) > 0 {
			log.Printf("JWKS fetch failed, using stale cache: %v", err)
			return f.buildCachedJWKS(), nil
		}
		return nil, fmt.Errorf("failed to fetch JWKS from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	f.cache.mu.Lock()
	defer f.cache.mu.Unlock()

	for _, key := range jwks.Keys {
		pubKey, err := jwkToRSAPublicKey(&key)
		if err != nil {
			log.Printf("Warning: failed to parse key %s: %v", key.KeyID, err)
			continue
		}
		f.cache.keys[key.KeyID] = pubKey
	}

	f.cache.expiresAt = time.Now().Add(f.cacheTTL)
	log.Printf("JWKS updated: %d keys cached, expires at %v", len(f.cache.keys), f.cache.expiresAt)

	return &jwks, nil
}

// GetPublicKey retrieves a public key by kid from cache
func (f *JWKSFetcher) GetPublicKey(kid string) (*rsa.PublicKey, error) {
	f.cache.mu.RLock()
	defer f.cache.mu.RUnlock()

	pubKey, exists := f.cache.keys[kid]
	if !exists {
		return nil, fmt.Errorf("public key not found for kid: %s", kid)
	}

	return pubKey, nil
}

func (f *JWKSFetcher) buildCachedJWKS() *JWKS {
	jwks := &JWKS{Keys: make([]JWK, 0, len(f.cache.keys))}

	for kid, pubKey := range f.cache.keys {
		jwk := JWK{
			KeyType:   "RSA",
			Use:       "sig",
			KeyID:     kid,
			Algorithm: "RS256",
			E:         "AQAB",
		}
		nBytes := pubKey.N.Bytes()
		jwk.N = base64.RawURLEncoding.EncodeToString(nBytes)
		jwks.Keys = append(jwks.Keys, jwk)
	}

	return jwks
}

func jwkToRSAPublicKey(jwk *JWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	pubKey := &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}

	return pubKey, nil
}
