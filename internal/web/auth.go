package web

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/runixio/runix/internal/auth"
)

const (
	// maxLoginBody is the maximum size of a login request body.
	maxLoginBody = 4096

	// loginRateLimit is the maximum failed login attempts per minute.
	loginRateLimit = 10
)

// loginRequest is the JSON body for login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginResponse is the JSON response for login.
type loginResponse struct {
	Success  bool   `json:"success,omitempty"`
	Error    string `json:"error,omitempty"`
	Username string `json:"username,omitempty"`
}

// authStatusResponse is the JSON response for auth status.
type authStatusResponse struct {
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username,omitempty"`
	Mode          string `json:"mode"`
}

// WebAuthMiddleware returns an http middleware that enforces web-based authentication.
// For browser requests (Accept: text/html), it redirects to /login.
// For API requests, it returns 401 JSON.
// Unauthenticated access is allowed for /login, /api/auth/login, and static assets.
func WebAuthMiddleware(sessions *auth.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if sessions == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Always allow login page and login API.
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Check for valid session cookie.
			token := auth.SessionFromRequest(r)
			if _, ok := sessions.ValidateSession(token); ok {
				next.ServeHTTP(w, r)
				return
			}

			// No valid session — respond based on request type.
			if isBrowserRequest(r) {
				// Redirect to login with the original URL as redirect param.
				redirect := sanitizeRedirectPath(r.URL.RequestURI())
				http.Redirect(w, r, "/login?redirect="+url.QueryEscape(redirect), http.StatusFound)
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(loginResponse{Error: "authentication required"})
			}
		})
	}
}

// isPublicPath returns true for paths that don't require authentication.
func isPublicPath(path string) bool {
	if path == "/login" || path == "/login.html" {
		return true
	}
	if strings.HasPrefix(path, "/api/auth/") {
		return true
	}
	return false
}

// isBrowserRequest checks if the request is from a browser (expects HTML).
func isBrowserRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html")
}

// handleLoginAPI handles POST /api/auth/login.
func (s *Server) handleLoginAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, loginResponse{Error: "method not allowed"})
		return
	}
	if s.auth.Mode() == "disabled" {
		writeJSON(w, http.StatusBadRequest, loginResponse{Error: "authentication is disabled"})
		return
	}
	if s.auth.Mode() == "token" {
		writeJSON(w, http.StatusBadRequest, loginResponse{Error: "web login is not supported for token authentication"})
		return
	}
	if s.sessions == nil {
		writeJSON(w, http.StatusInternalServerError, loginResponse{Error: "session store unavailable"})
		return
	}
	if !s.allowLoginAttempt(r.RemoteAddr) {
		writeJSON(w, http.StatusTooManyRequests, loginResponse{Error: "too many failed login attempts"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBody)

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, loginResponse{Error: "invalid request body"})
		return
	}

	// Validate credentials using the authenticator.
	// Create a synthetic HTTP request with basic auth to reuse existing auth logic.
	authReq := r.Clone(r.Context())
	authReq.SetBasicAuth(req.Username, req.Password)

	if err := s.auth.Authenticate(authReq); err != nil {
		s.recordFailedLogin(r.RemoteAddr)
		log.Warn().Str("username", req.Username).Str("remote", r.RemoteAddr).Msg("failed login attempt")
		writeJSON(w, http.StatusUnauthorized, loginResponse{Error: "Invalid username or password."})
		return
	}
	s.resetFailedLogins(r.RemoteAddr)

	// Create session.
	sess, err := s.sessions.CreateSession(req.Username)
	if err != nil {
		log.Error().Err(err).Msg("failed to create session")
		writeJSON(w, http.StatusInternalServerError, loginResponse{Error: "internal error"})
		return
	}

	// Set session cookie.
	cookie := auth.SessionCookie(sess.Token, time.Until(sess.ExpiresAt))
	cookie.Secure = isSecureRequest(r)
	http.SetCookie(w, cookie)

	log.Info().Str("username", req.Username).Str("remote", r.RemoteAddr).Msg("login successful")
	writeJSON(w, http.StatusOK, loginResponse{Success: true, Username: req.Username})
}

// handleLogoutAPI handles POST /api/auth/logout.
func (s *Server) handleLogoutAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, loginResponse{Error: "method not allowed"})
		return
	}

	// Delete session from store.
	token := auth.SessionFromRequest(r)
	if token != "" && s.sessions != nil {
		s.sessions.DeleteSession(token)
	}

	// Clear cookie.
	cookie := auth.ClearSessionCookie()
	cookie.Secure = isSecureRequest(r)
	http.SetCookie(w, cookie)

	log.Info().Str("remote", r.RemoteAddr).Msg("logout successful")
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

// handleAuthStatusAPI handles GET /api/auth/status.
func (s *Server) handleAuthStatusAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, loginResponse{Error: "method not allowed"})
		return
	}

	resp := authStatusResponse{
		Mode: s.auth.Mode(),
	}

	if s.sessions == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	token := auth.SessionFromRequest(r)
	if sess, ok := s.sessions.ValidateSession(token); ok {
		resp.Authenticated = true
		resp.Username = sess.Username
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleLoginPage handles GET /login.
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.auth.Mode() == "disabled" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// If already authenticated, redirect to dashboard.
	token := auth.SessionFromRequest(r)
	if s.sessions != nil {
		if _, ok := s.sessions.ValidateSession(token); ok {
			redirect := sanitizeRedirectPath(r.URL.Query().Get("redirect"))
			if redirect == "" {
				redirect = "/"
			}
			http.Redirect(w, r, redirect, http.StatusFound)
			return
		}
	}

	// Serve the login page.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	loginHTML, err := frontendFS.ReadFile("frontend/login.html")
	if err != nil {
		log.Error().Err(err).Msg("failed to read login.html")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Write(loginHTML)
}

func sanitizeRedirectPath(raw string) string {
	if raw == "" {
		return "/"
	}
	if strings.HasPrefix(raw, "//") {
		return "/"
	}
	if !strings.HasPrefix(raw, "/") {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "/"
	}
	if u.IsAbs() || u.Host != "" {
		return "/"
	}
	cleanPath := path.Clean("/" + strings.TrimPrefix(u.Path, "/"))
	if cleanPath == "." {
		cleanPath = "/"
	}
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/"
	}
	if u.RawQuery != "" {
		return cleanPath + "?" + u.RawQuery
	}
	return cleanPath
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func loginRateWindowCutoff(now time.Time) time.Time {
	return now.Add(-time.Minute)
}

func normalizeRemoteAddr(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		if remoteAddr == "" {
			return "unknown"
		}
		return remoteAddr
	}
	if host == "" {
		return "unknown"
	}
	return host
}

func (s *Server) pruneLoginFailures(remoteAddr string, now time.Time) []time.Time {
	key := normalizeRemoteAddr(remoteAddr)
	failures := s.loginFailures[key]
	cutoff := loginRateWindowCutoff(now)
	n := 0
	for _, ts := range failures {
		if ts.After(cutoff) {
			failures[n] = ts
			n++
		}
	}
	failures = failures[:n]
	if len(failures) == 0 {
		delete(s.loginFailures, key)
		return nil
	}
	s.loginFailures[key] = failures
	return failures
}

func (s *Server) allowLoginAttempt(remoteAddr string) bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()

	failures := s.pruneLoginFailures(remoteAddr, time.Now())
	return len(failures) < loginRateLimit
}

func (s *Server) recordFailedLogin(remoteAddr string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()

	key := normalizeRemoteAddr(remoteAddr)
	failures := s.pruneLoginFailures(remoteAddr, time.Now())
	failures = append(failures, time.Now())
	s.loginFailures[key] = failures
}

func (s *Server) resetFailedLogins(remoteAddr string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()

	delete(s.loginFailures, normalizeRemoteAddr(remoteAddr))
}
