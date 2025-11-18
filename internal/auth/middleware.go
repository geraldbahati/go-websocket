package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type KindeClaims struct {
	jwt.RegisteredClaims
	GivenName    string                 `json:"given_name"`
	FamilyName   string                 `json:"family_name"`
	Email        string                 `json:"email"`
	Picture      string                 `json:"picture"`
	OrgCode      string                 `json:"org_code"`
	Permissions  []string               `json:"permissions"`
	FeatureFlags map[string]interface{} `json:"feature_flags"`
}

type JWKS struct {
	Keys []JWK `json:"keys"`
}

type JWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	Alg string `json:"alg"`
}

var (
	kindeJWKS    *JWKS
	jwksMutex    sync.RWMutex
	kindeIssuer  string
	jwksCache    = make(map[string]*rsa.PublicKey)
	jwksCacheMux sync.RWMutex
)

// InitJWKS fetches and caches Kinde's JWKS
func InitJWKS(issuerURL string) error {
	kindeIssuer = issuerURL

	if err := refreshJWKS(); err != nil {
		return err
	}

	// Refresh JWKS every 24 hours
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			if err := refreshJWKS(); err != nil {
				log.Printf("Error refreshing JWKS: %v", err)
			} else {
				log.Println("JWKS refreshed successfully")
			}
		}
	}()

	return nil
}

func refreshJWKS() error {
	jwksURL := fmt.Sprintf("%s/.well-known/jwks.json", kindeIssuer)

	log.Printf("Fetching JWKS from: %s", jwksURL)

	resp, err := http.Get(jwksURL)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS: %w", err)
	}

	jwksMutex.Lock()
	kindeJWKS = &jwks
	jwksMutex.Unlock()

	// Clear cache to force re-conversion
	jwksCacheMux.Lock()
	jwksCache = make(map[string]*rsa.PublicKey)
	jwksCacheMux.Unlock()

	log.Printf("JWKS loaded with %d keys", len(jwks.Keys))

	return nil
}

// ValidateToken validates a Kinde JWT token
func ValidateToken(tokenString string) (*KindeClaims, error) {
	// Remove "Bearer " prefix if present
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	if tokenString == "" {
		return nil, errors.New("token is empty")
	}

	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &KindeClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get kid from token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("kid not found in token header")
		}

		// Get public key for this kid
		publicKey, err := getPublicKey(kid)
		if err != nil {
			return nil, err
		}

		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*KindeClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	// Verify issuer
	if claims.Issuer != kindeIssuer {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", kindeIssuer, claims.Issuer)
	}

	// Verify expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		return nil, errors.New("token expired")
	}

	return claims, nil
}

// getPublicKey retrieves and caches public key for a given kid
func getPublicKey(kid string) (*rsa.PublicKey, error) {
	// Check cache first
	jwksCacheMux.RLock()
	if key, exists := jwksCache[kid]; exists {
		jwksCacheMux.RUnlock()
		return key, nil
	}
	jwksCacheMux.RUnlock()

	// Find key in JWKS
	jwksMutex.RLock()
	defer jwksMutex.RUnlock()

	if kindeJWKS == nil {
		return nil, errors.New("JWKS not initialized")
	}

	for _, jwk := range kindeJWKS.Keys {
		if jwk.Kid == kid {
			publicKey, err := jwkToPublicKey(jwk)
			if err != nil {
				return nil, err
			}

			// Cache it
			jwksCacheMux.Lock()
			jwksCache[kid] = publicKey
			jwksCacheMux.Unlock()

			return publicKey, nil
		}
	}

	return nil, fmt.Errorf("key with kid %s not found in JWKS", kid)
}

// jwkToPublicKey converts JWK to RSA public key
func jwkToPublicKey(jwk JWK) (*rsa.PublicKey, error) {
	// Decode base64url-encoded modulus (n)
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	// Decode base64url-encoded exponent (e)
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// Convert n to big.Int
	n := new(big.Int).SetBytes(nBytes)

	// Convert e to int
	var e int
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	return &rsa.PublicKey{
		N: n,
		E: e,
	}, nil
}

// ExtractTokenFromRequest extracts JWT from request (query param or header)
func ExtractTokenFromRequest(r *http.Request) string {
	// Try query parameter first
	token := r.URL.Query().Get("token")
	if token != "" {
		return token
	}

	// Try Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	return ""
}
