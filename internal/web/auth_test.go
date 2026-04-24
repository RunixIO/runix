package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/runixio/runix/internal/auth"
	"github.com/runixio/runix/internal/supervisor"
	"github.com/runixio/runix/pkg/types"
)

func setupAuthTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sup := supervisor.New(supervisor.Options{
		LogDir: logDir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
			MaxRestarts:   0,
		},
	})
	authenticator, err := auth.NewAuthenticator(types.AuthSettings{
		Enabled:  true,
		Mode:     types.AuthModeBasic,
		Username: "admin",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("NewAuthenticator: %v", err)
	}
	return NewServerWithAuth(sup, "127.0.0.1:0", authenticator)
}

func setupNoAuthTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sup := supervisor.New(supervisor.Options{
		LogDir: logDir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
			MaxRestarts:   0,
		},
	})
	return NewServerWithAuth(sup, "127.0.0.1:0", nil)
}

func setupTokenAuthTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sup := supervisor.New(supervisor.Options{
		LogDir: logDir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
			MaxRestarts:   0,
		},
	})
	authenticator, err := auth.NewAuthenticator(types.AuthSettings{
		Enabled: true,
		Mode:    types.AuthModeToken,
		Token:   "my-secret-token-1234567890abcdef",
	})
	if err != nil {
		t.Fatalf("NewAuthenticator: %v", err)
	}
	return NewServerWithAuth(sup, "127.0.0.1:0", authenticator)
}

func TestLoginPage(t *testing.T) {
	srv := setupAuthTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	srv.handleLoginPage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content type, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Sign in") {
		t.Error("login page should contain 'Sign in'")
	}
	if !strings.Contains(body, "Username") {
		t.Error("login page should contain 'Username'")
	}
	if !strings.Contains(body, "Password") {
		t.Error("login page should contain 'Password'")
	}
}

func TestLoginAPI_Success(t *testing.T) {
	srv := setupAuthTestServer(t)

	body := `{"username":"admin","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleLoginAPI(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp loginResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.Username != "admin" {
		t.Errorf("expected username 'admin', got %q", resp.Username)
	}

	// Should have session cookie.
	cookies := w.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == auth.SessionCookieName {
			found = true
			if c.Value == "" {
				t.Error("session cookie should not be empty")
			}
			if !c.HttpOnly {
				t.Error("session cookie should be HttpOnly")
			}
			break
		}
	}
	if !found {
		t.Error("expected session cookie in response")
	}
}

func TestLoginAPI_WrongPassword(t *testing.T) {
	srv := setupAuthTestServer(t)

	body := `{"username":"admin","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleLoginAPI(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	var resp loginResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == "" {
		t.Error("expected error message in response")
	}
}

func TestLoginAPI_WrongUsername(t *testing.T) {
	srv := setupAuthTestServer(t)

	body := `{"username":"root","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleLoginAPI(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestLoginAPI_InvalidJSON(t *testing.T) {
	srv := setupAuthTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleLoginAPI(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestLoginAPI_MethodNotAllowed(t *testing.T) {
	srv := setupAuthTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	w := httptest.NewRecorder()
	srv.handleLoginAPI(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestLoginAPI_AuthDisabled(t *testing.T) {
	srv := setupNoAuthTestServer(t)

	body := `{"username":"admin","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleLoginAPI(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLoginAPI_TokenModeUnsupported(t *testing.T) {
	srv := setupTokenAuthTestServer(t)

	body := `{"username":"admin","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleLoginAPI(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLoginAPI_RateLimited(t *testing.T) {
	srv := setupAuthTestServer(t)

	for i := 0; i < loginRateLimit; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:12345"
		w := httptest.NewRecorder()
		srv.handleLoginAPI(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, w.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	srv.handleLoginAPI(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestAuthStatusAPI_Unauthenticated(t *testing.T) {
	srv := setupAuthTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	w := httptest.NewRecorder()
	srv.handleAuthStatusAPI(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp authStatusResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Authenticated {
		t.Error("expected authenticated=false")
	}
	if resp.Mode != "basic" {
		t.Errorf("expected mode 'basic', got %q", resp.Mode)
	}
}

func TestAuthStatusAPI_Authenticated(t *testing.T) {
	srv := setupAuthTestServer(t)

	// Create a session first.
	sess, err := srv.sessions.CreateSession("admin")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	req.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: sess.Token,
	})
	w := httptest.NewRecorder()
	srv.handleAuthStatusAPI(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp authStatusResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Authenticated {
		t.Error("expected authenticated=true")
	}
	if resp.Username != "admin" {
		t.Errorf("expected username 'admin', got %q", resp.Username)
	}
}

func TestLogoutAPI(t *testing.T) {
	srv := setupAuthTestServer(t)

	// Create a session first.
	sess, err := srv.sessions.CreateSession("admin")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Verify session is valid.
	if _, ok := srv.sessions.ValidateSession(sess.Token); !ok {
		t.Fatal("session should be valid before logout")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: sess.Token,
	})
	w := httptest.NewRecorder()
	srv.handleLogoutAPI(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// Session should be invalidated.
	if _, ok := srv.sessions.ValidateSession(sess.Token); ok {
		t.Error("session should be invalid after logout")
	}

	// Should have clear cookie.
	cookies := w.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == auth.SessionCookieName && c.MaxAge < 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cleared session cookie in logout response")
	}
}

func TestLoginAPI_SetsSecureCookieForForwardedHTTPS(t *testing.T) {
	srv := setupAuthTestServer(t)

	body := `{"username":"admin","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	srv.handleLoginAPI(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var found bool
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			found = true
			if !c.Secure {
				t.Fatal("expected secure session cookie")
			}
		}
	}
	if !found {
		t.Fatal("expected session cookie")
	}
}

func TestWebAuthMiddleware_BrowserRedirect(t *testing.T) {
	srv := setupAuthTestServer(t)

	handler := WebAuthMiddleware(srv.sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/processes", nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "/login") {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
	if strings.Contains(loc, "//evil.example") {
		t.Fatalf("unexpected unsafe redirect location %q", loc)
	}
}

func TestWebAuthMiddleware_BrowserRedirectEscapesRedirect(t *testing.T) {
	srv := setupAuthTestServer(t)

	handler := WebAuthMiddleware(srv.sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "//evil.example", nil)
	req.URL.Path = "/"
	req.URL.RawQuery = "next=1"
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Location"); got != "/login?redirect=%2F%3Fnext%3D1" {
		t.Fatalf("unexpected redirect location %q", got)
	}
}

func TestWebAuthMiddleware_APIUnauthorized(t *testing.T) {
	srv := setupAuthTestServer(t)

	handler := WebAuthMiddleware(srv.sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/processes", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	var resp loginResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == "" {
		t.Error("expected error in 401 response")
	}
}

func TestWebAuthMiddleware_AuthenticatedPassThrough(t *testing.T) {
	srv := setupAuthTestServer(t)

	sess, err := srv.sessions.CreateSession("admin")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	handler := WebAuthMiddleware(srv.sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/processes", nil)
	req.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: sess.Token,
	})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWebAuthMiddleware_PublicPaths(t *testing.T) {
	srv := setupAuthTestServer(t)

	handler := WebAuthMiddleware(srv.sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	paths := []string{"/login", "/login.html", "/api/auth/login", "/api/auth/logout", "/api/auth/status"}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("path %q: expected 200, got %d", path, w.Code)
		}
	}
}

func TestLoginAPI_FullFlow(t *testing.T) {
	srv := setupAuthTestServer(t)

	// Step 1: Login.
	loginBody := `{"username":"admin","password":"secret123"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	srv.handleLoginAPI(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", loginW.Code, loginW.Body.String())
	}

	// Extract session cookie.
	var sessionCookie *http.Cookie
	for _, c := range loginW.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("login: expected session cookie")
	}

	// Step 2: Check auth status with cookie.
	statusReq := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	statusReq.AddCookie(sessionCookie)
	statusW := httptest.NewRecorder()
	srv.handleAuthStatusAPI(statusW, statusReq)

	var statusResp authStatusResponse
	json.NewDecoder(statusW.Body).Decode(&statusResp)
	if !statusResp.Authenticated {
		t.Error("status: expected authenticated=true after login")
	}
	if statusResp.Username != "admin" {
		t.Errorf("status: expected 'admin', got %q", statusResp.Username)
	}

	// Step 3: Logout.
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutW := httptest.NewRecorder()
	srv.handleLogoutAPI(logoutW, logoutReq)

	if logoutW.Code != http.StatusOK {
		t.Errorf("logout: expected 200, got %d", logoutW.Code)
	}

	// Step 4: Verify session is invalid.
	statusReq2 := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	statusReq2.AddCookie(sessionCookie)
	statusW2 := httptest.NewRecorder()
	srv.handleAuthStatusAPI(statusW2, statusReq2)

	var statusResp2 authStatusResponse
	json.NewDecoder(statusW2.Body).Decode(&statusResp2)
	if statusResp2.Authenticated {
		t.Error("status: expected authenticated=false after logout")
	}
}

func TestSessionStore_Expiry(t *testing.T) {
	store := auth.NewSessionStore(100 * time.Millisecond)
	defer store.Stop()

	sess, err := store.CreateSession("test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Should be valid immediately.
	if _, ok := store.ValidateSession(sess.Token); !ok {
		t.Error("session should be valid immediately after creation")
	}

	// Wait for expiry.
	time.Sleep(200 * time.Millisecond)

	// Should be invalid after expiry.
	if _, ok := store.ValidateSession(sess.Token); ok {
		t.Error("session should be invalid after TTL")
	}
}

func TestHandleLoginPage_DisabledAuthRedirectsHome(t *testing.T) {
	srv := setupNoAuthTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	srv.handleLoginPage(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if got := w.Header().Get("Location"); got != "/" {
		t.Fatalf("expected redirect to /, got %q", got)
	}
}

func TestSanitizeRedirectPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: "/"},
		{name: "relative path", in: "/api/v1/processes?x=1", want: "/api/v1/processes?x=1"},
		{name: "double slash", in: "//evil.example", want: "/"},
		{name: "absolute url", in: "https://evil.example", want: "/"},
		{name: "path traversal", in: "/../../admin", want: "/admin"},
		{name: "no leading slash", in: "dashboard", want: "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeRedirectPath(tt.in); got != tt.want {
				t.Fatalf("sanitizeRedirectPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
