// Package auth provides authentication and access control for all Runix interfaces.
//
// It supports multiple authentication modes:
//   - disabled: no authentication (for local development)
//   - basic: HTTP Basic Auth with username/password or password hash
//   - token: Bearer token authentication
//
// The package provides middleware for HTTP servers and an Authenticator interface
// for custom integrations.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/runixio/runix/pkg/types"
	"golang.org/x/crypto/bcrypt"
)

// Authenticator validates credentials and returns an error if authentication fails.
type Authenticator interface {
	// Authenticate validates an HTTP request and returns nil on success.
	Authenticate(r *http.Request) error

	// Mode returns the authentication mode name.
	Mode() string
}

// NewAuthenticator creates an Authenticator from the given AuthSettings.
// Returns a NoAuth authenticator if auth is disabled.
func NewAuthenticator(cfg types.AuthSettings) (Authenticator, error) {
	if !cfg.IsEnabled() {
		return &NoAuth{}, nil
	}

	mode := cfg.EffectiveMode()
	switch mode {
	case types.AuthModeDisabled:
		return &NoAuth{}, nil
	case types.AuthModeBasic:
		return newBasicAuth(cfg)
	case types.AuthModeToken:
		return newTokenAuth(cfg)
	default:
		return nil, fmt.Errorf("unsupported auth mode: %q", mode)
	}
}

// --- NoAuth ---

// NoAuth is an authenticator that accepts all requests.
type NoAuth struct{}

func (a *NoAuth) Authenticate(r *http.Request) error { return nil }
func (a *NoAuth) Mode() string                       { return "disabled" }

// --- BasicAuth ---

// BasicAuth validates HTTP Basic Authentication.
type BasicAuth struct {
	username     string
	passwordHash string // bcrypt hash
	password     string // plain text (fallback for dev)
	localOnly    bool
}

func newBasicAuth(cfg types.AuthSettings) (*BasicAuth, error) {
	a := &BasicAuth{
		username:  cfg.Username,
		localOnly: cfg.LocalOnly,
	}

	if cfg.PasswordHash != "" {
		// Validate that the hash looks like a bcrypt hash.
		if !strings.HasPrefix(cfg.PasswordHash, "$2a$") &&
			!strings.HasPrefix(cfg.PasswordHash, "$2b$") &&
			!strings.HasPrefix(cfg.PasswordHash, "$2y$") {
			return nil, fmt.Errorf("security.auth.password_hash must be a bcrypt hash")
		}
		a.passwordHash = cfg.PasswordHash
	} else {
		a.password = cfg.Password
	}

	return a, nil
}

func (a *BasicAuth) Mode() string { return "basic" }

func (a *BasicAuth) Authenticate(r *http.Request) error {
	// Check local-only exemption.
	if a.localOnly && isLocalRequest(r) {
		return nil
	}

	user, pass, ok := r.BasicAuth()
	if !ok {
		return ErrUnauthorized
	}

	// Constant-time comparison for username.
	if subtle.ConstantTimeCompare([]byte(user), []byte(a.username)) != 1 {
		return ErrUnauthorized
	}

	// Validate password.
	if a.passwordHash != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(a.passwordHash), []byte(pass)); err != nil {
			return ErrUnauthorized
		}
	} else {
		if subtle.ConstantTimeCompare([]byte(pass), []byte(a.password)) != 1 {
			return ErrUnauthorized
		}
	}

	return nil
}

// --- TokenAuth ---

// TokenAuth validates Bearer token authentication.
type TokenAuth struct {
	tokenHash []byte // SHA-256 hash of the token
	localOnly bool
}

func newTokenAuth(cfg types.AuthSettings) (*TokenAuth, error) {
	// Store a hash of the token, not the raw token.
	hash := sha256.Sum256([]byte(cfg.Token))
	return &TokenAuth{
		tokenHash: hash[:],
		localOnly: cfg.LocalOnly,
	}, nil
}

func (a *TokenAuth) Mode() string { return "token" }

func (a *TokenAuth) Authenticate(r *http.Request) error {
	// Check local-only exemption.
	if a.localOnly && isLocalRequest(r) {
		return nil
	}

	var token string

	// Try Authorization header first.
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else if strings.HasPrefix(authHeader, "Token ") {
			token = strings.TrimPrefix(authHeader, "Token ")
		}
	}

	// Also support X-API-Token header.
	if token == "" {
		token = r.Header.Get("X-API-Token")
	}
	if token == "" {
		return ErrUnauthorized
	}

	// Compare hashed token.
	hash := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(hash[:], a.tokenHash) != 1 {
		return ErrUnauthorized
	}

	return nil
}

// --- Helpers ---

// isLocalRequest checks if the request comes from localhost.
func isLocalRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	return ip.IsLoopback()
}

// --- Errors ---

// AuthError represents an authentication failure.
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string { return e.Message }

// ErrUnauthorized is returned when authentication fails.
var ErrUnauthorized = &AuthError{Message: "unauthorized"}

// --- Middleware ---

// Middleware returns an http middleware that enforces authentication.
// It writes 401 Unauthorized without a WWW-Authenticate challenge so browser
// clients do not trigger a native auth prompt.
func Middleware(auth Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := auth.Authenticate(r); err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MiddlewareFunc wraps a single http.HandlerFunc with authentication.
func MiddlewareFunc(auth Authenticator, fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := auth.Authenticate(r); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		fn(w, r)
	}
}

// WebSocketMiddleware is like Middleware but also sets proper headers for WebSocket upgrade.
func WebSocketMiddleware(auth Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := auth.Authenticate(r); err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- Password Hashing ---

// HashPassword generates a bcrypt hash from a plain text password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword checks a plain text password against a bcrypt hash.
func VerifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GenerateToken creates a random token string suitable for API authentication.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
