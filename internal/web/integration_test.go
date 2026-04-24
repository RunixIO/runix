package web

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/runixio/runix/internal/auth"
	"github.com/runixio/runix/internal/supervisor"
	"github.com/runixio/runix/pkg/types"
)

func TestIntegration_LoginFlow(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sup := supervisor.New(supervisor.Options{
		LogDir: logDir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
		},
	})
	defer sup.Shutdown()

	authenticator, err := auth.NewAuthenticator(types.AuthSettings{
		Enabled:  true,
		Mode:     types.AuthModeBasic,
		Username: "admin",
		Password: "secret123",
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServerWithAuth(sup, "127.0.0.1:18923", authenticator)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Start(ctx)
	time.Sleep(500 * time.Millisecond) // wait for server to start

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't follow redirects
		},
	}

	// 1. Browser request to / should redirect to /login (NOT show native auth dialog).
	t.Run("browser redirect to login", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://127.0.0.1:18923/", nil)
		req.Header.Set("Accept", "text/html")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusFound {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		loc := resp.Header.Get("Location")
		if !strings.Contains(loc, "/login") {
			t.Errorf("expected redirect to /login, got %q", loc)
		}
		// MUST NOT have WWW-Authenticate header (that triggers native dialog).
		if wwa := resp.Header.Get("WWW-Authenticate"); wwa != "" {
			t.Errorf("MUST NOT have WWW-Authenticate header, got %q (this triggers browser native auth dialog)", wwa)
		}
	})

	// 2. API request to / should return 401 JSON (NOT show native auth dialog).
	t.Run("api 401 without native dialog", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://127.0.0.1:18923/api/v1/processes", nil)
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
		// MUST NOT have WWW-Authenticate header.
		if wwa := resp.Header.Get("WWW-Authenticate"); wwa != "" {
			t.Errorf("MUST NOT have WWW-Authenticate header, got %q (this triggers browser native auth dialog)", wwa)
		}
	})

	// 3. Login page should be accessible without auth.
	t.Run("login page accessible", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://127.0.0.1:18923/login", nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Sign in") {
			t.Error("login page should contain 'Sign in'")
		}
	})

	// 4. Login API should return session cookie.
	t.Run("login returns session cookie", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "http://127.0.0.1:18923/api/auth/login",
			strings.NewReader(`{"username":"admin","password":"secret123"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
		var found bool
		for _, c := range resp.Cookies() {
			if c.Name == auth.SessionCookieName {
				found = true
				if !c.HttpOnly {
					t.Error("session cookie should be HttpOnly")
				}
			}
		}
		if !found {
			t.Error("expected session cookie")
		}
	})

	// 5. Full flow: login -> access dashboard -> logout.
	t.Run("full login flow", func(t *testing.T) {
		// Login.
		loginReq, _ := http.NewRequest("POST", "http://127.0.0.1:18923/api/auth/login",
			strings.NewReader(`{"username":"admin","password":"secret123"}`))
		loginReq.Header.Set("Content-Type", "application/json")
		loginResp, err := client.Do(loginReq)
		if err != nil {
			t.Fatal(err)
		}
		loginResp.Body.Close()

		if loginResp.StatusCode != http.StatusOK {
			t.Fatalf("login failed: %d", loginResp.StatusCode)
		}

		// Access dashboard with cookie.
		dashReq, _ := http.NewRequest("GET", "http://127.0.0.1:18923/", nil)
		dashReq.Header.Set("Accept", "text/html")
		for _, c := range loginResp.Cookies() {
			dashReq.AddCookie(c)
		}
		dashResp, err := client.Do(dashReq)
		if err != nil {
			t.Fatal(err)
		}
		dashResp.Body.Close()

		if dashResp.StatusCode != http.StatusOK {
			t.Errorf("dashboard should be 200 after login, got %d", dashResp.StatusCode)
		}

		// Logout.
		logoutReq, _ := http.NewRequest("POST", "http://127.0.0.1:18923/api/auth/logout", nil)
		for _, c := range loginResp.Cookies() {
			logoutReq.AddCookie(c)
		}
		logoutResp, err := client.Do(logoutReq)
		if err != nil {
			t.Fatal(err)
		}
		logoutResp.Body.Close()

		if logoutResp.StatusCode != http.StatusOK {
			t.Errorf("logout: expected 200, got %d", logoutResp.StatusCode)
		}

		// After logout, dashboard should redirect again.
		dashReq2, _ := http.NewRequest("GET", "http://127.0.0.1:18923/", nil)
		dashReq2.Header.Set("Accept", "text/html")
		dashResp2, err := client.Do(dashReq2)
		if err != nil {
			t.Fatal(err)
		}
		dashResp2.Body.Close()

		if dashResp2.StatusCode != http.StatusFound {
			t.Errorf("after logout, dashboard should redirect (302), got %d", dashResp2.StatusCode)
		}
	})
}

func TestStart_ReturnsErrorWhenPortInUse(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sup := supervisor.New(supervisor.Options{
		LogDir: logDir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
		},
	})
	defer sup.Shutdown()

	authenticator, err := auth.NewAuthenticator(types.AuthSettings{
		Enabled:  true,
		Mode:     types.AuthModeBasic,
		Username: "admin",
		Password: "secret123",
	})
	if err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	srv := NewServerWithAuth(sup, ln.Addr().String(), authenticator)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := srv.Start(ctx); err == nil {
		t.Fatal("expected bind error, got nil")
	}
}
