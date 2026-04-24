package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/runixio/runix/pkg/types"
)

func TestNoAuth(t *testing.T) {
	a := &NoAuth{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := a.Authenticate(req); err != nil {
		t.Errorf("NoAuth should accept all requests, got error: %v", err)
	}
	if a.Mode() != "disabled" {
		t.Errorf("NoAuth mode = %q, want %q", a.Mode(), "disabled")
	}
}

func TestNewAuthenticator_Disabled(t *testing.T) {
	cfg := types.AuthSettings{Enabled: false}
	a, err := NewAuthenticator(cfg)
	if err != nil {
		t.Fatalf("NewAuthenticator: %v", err)
	}
	if a.Mode() != "disabled" {
		t.Errorf("mode = %q, want %q", a.Mode(), "disabled")
	}
}

func TestNewAuthenticator_Basic(t *testing.T) {
	cfg := types.AuthSettings{
		Enabled:  true,
		Mode:     types.AuthModeBasic,
		Username: "admin",
		Password: "secret123",
	}
	a, err := NewAuthenticator(cfg)
	if err != nil {
		t.Fatalf("NewAuthenticator: %v", err)
	}
	if a.Mode() != "basic" {
		t.Errorf("mode = %q, want %q", a.Mode(), "basic")
	}
}

func TestNewAuthenticator_BasicWithHash(t *testing.T) {
	// Generate a real bcrypt hash for testing.
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	cfg := types.AuthSettings{
		Enabled:      true,
		Mode:         types.AuthModeBasic,
		Username:     "admin",
		PasswordHash: hash,
	}
	a, err := NewAuthenticator(cfg)
	if err != nil {
		t.Fatalf("NewAuthenticator: %v", err)
	}

	// Correct credentials should pass.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "secret123")
	if err := a.Authenticate(req); err != nil {
		t.Errorf("correct credentials should pass: %v", err)
	}

	// Wrong password should fail.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.SetBasicAuth("admin", "wrong")
	if err := a.Authenticate(req2); err == nil {
		t.Error("wrong password should fail")
	}
}

func TestBasicAuth_Success(t *testing.T) {
	cfg := types.AuthSettings{
		Enabled:  true,
		Mode:     types.AuthModeBasic,
		Username: "admin",
		Password: "secret",
	}
	a, _ := NewAuthenticator(cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "secret")
	if err := a.Authenticate(req); err != nil {
		t.Errorf("correct credentials should pass: %v", err)
	}
}

func TestBasicAuth_WrongPassword(t *testing.T) {
	cfg := types.AuthSettings{
		Enabled:  true,
		Mode:     types.AuthModeBasic,
		Username: "admin",
		Password: "secret",
	}
	a, _ := NewAuthenticator(cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "wrongpass")
	if err := a.Authenticate(req); err == nil {
		t.Error("wrong password should fail")
	}
}

func TestBasicAuth_WrongUsername(t *testing.T) {
	cfg := types.AuthSettings{
		Enabled:  true,
		Mode:     types.AuthModeBasic,
		Username: "admin",
		Password: "secret",
	}
	a, _ := NewAuthenticator(cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("root", "secret")
	if err := a.Authenticate(req); err == nil {
		t.Error("wrong username should fail")
	}
}

func TestBasicAuth_NoCredentials(t *testing.T) {
	cfg := types.AuthSettings{
		Enabled:  true,
		Mode:     types.AuthModeBasic,
		Username: "admin",
		Password: "secret",
	}
	a, _ := NewAuthenticator(cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := a.Authenticate(req); err == nil {
		t.Error("missing credentials should fail")
	}
}

func TestTokenAuth_Success(t *testing.T) {
	cfg := types.AuthSettings{
		Enabled: true,
		Mode:    types.AuthModeToken,
		Token:   "my-secret-token-1234567890abcdef",
	}
	a, _ := NewAuthenticator(cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer my-secret-token-1234567890abcdef")
	if err := a.Authenticate(req); err != nil {
		t.Errorf("correct token should pass: %v", err)
	}
}

func TestTokenAuth_TokenPrefix(t *testing.T) {
	cfg := types.AuthSettings{
		Enabled: true,
		Mode:    types.AuthModeToken,
		Token:   "my-secret-token-1234567890abcdef",
	}
	a, _ := NewAuthenticator(cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Token my-secret-token-1234567890abcdef")
	if err := a.Authenticate(req); err != nil {
		t.Errorf("Token prefix should work: %v", err)
	}
}

func TestTokenAuth_XAPIToken(t *testing.T) {
	cfg := types.AuthSettings{
		Enabled: true,
		Mode:    types.AuthModeToken,
		Token:   "my-secret-token-1234567890abcdef",
	}
	a, _ := NewAuthenticator(cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Token", "my-secret-token-1234567890abcdef")
	if err := a.Authenticate(req); err != nil {
		t.Errorf("X-API-Token header should work: %v", err)
	}
}

func TestTokenAuth_WrongToken(t *testing.T) {
	cfg := types.AuthSettings{
		Enabled: true,
		Mode:    types.AuthModeToken,
		Token:   "my-secret-token-1234567890abcdef",
	}
	a, _ := NewAuthenticator(cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	if err := a.Authenticate(req); err == nil {
		t.Error("wrong token should fail")
	}
}

func TestTokenAuth_NoToken(t *testing.T) {
	cfg := types.AuthSettings{
		Enabled: true,
		Mode:    types.AuthModeToken,
		Token:   "my-secret-token-1234567890abcdef",
	}
	a, _ := NewAuthenticator(cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := a.Authenticate(req); err == nil {
		t.Error("missing token should fail")
	}
}

func TestBasicAuth_LocalOnly(t *testing.T) {
	cfg := types.AuthSettings{
		Enabled:   true,
		Mode:      types.AuthModeBasic,
		Username:  "admin",
		Password:  "secret",
		LocalOnly: true,
	}
	a, _ := NewAuthenticator(cfg)

	// Local request should pass without credentials.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	if err := a.Authenticate(req); err != nil {
		t.Errorf("local request should pass without auth: %v", err)
	}

	// Remote request with correct credentials should pass.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "192.168.1.100:12345"
	req2.SetBasicAuth("admin", "secret")
	if err := a.Authenticate(req2); err != nil {
		t.Errorf("remote request with correct credentials should pass: %v", err)
	}

	// Remote request without credentials should fail.
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.RemoteAddr = "192.168.1.100:12345"
	if err := a.Authenticate(req3); err == nil {
		t.Error("remote request without credentials should fail")
	}
}

func TestTokenAuth_LocalOnly(t *testing.T) {
	cfg := types.AuthSettings{
		Enabled:   true,
		Mode:      types.AuthModeToken,
		Token:     "my-secret-token-1234567890abcdef",
		LocalOnly: true,
	}
	a, _ := NewAuthenticator(cfg)

	// Local request should pass without token.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	if err := a.Authenticate(req); err != nil {
		t.Errorf("local request should pass without auth: %v", err)
	}
}

func TestMiddleware(t *testing.T) {
	cfg := types.AuthSettings{
		Enabled:  true,
		Mode:     types.AuthModeBasic,
		Username: "admin",
		Password: "secret",
	}
	a, _ := NewAuthenticator(cfg)

	handler := Middleware(a)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Without credentials -> 401.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if got := w.Header().Get("WWW-Authenticate"); got != "" {
		t.Errorf("expected no WWW-Authenticate header, got %q", got)
	}

	// With correct credentials -> 200.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.SetBasicAuth("admin", "secret")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}
}

func TestMiddlewareFunc(t *testing.T) {
	cfg := types.AuthSettings{
		Enabled:  true,
		Mode:     types.AuthModeBasic,
		Username: "admin",
		Password: "secret",
	}
	a, _ := NewAuthenticator(cfg)

	handler := MiddlewareFunc(a, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Without credentials -> 401.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if got := w.Header().Get("WWW-Authenticate"); got != "" {
		t.Errorf("expected no WWW-Authenticate header, got %q", got)
	}
}

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("my-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("hash should not be empty")
	}
	// Should start with bcrypt prefix.
	if hash[:4] != "$2a$" && hash[:4] != "$2b$" {
		t.Errorf("hash should start with $2a$ or $2b$, got %q", hash[:4])
	}
}

func TestVerifyPassword(t *testing.T) {
	hash, _ := HashPassword("my-password")
	if err := VerifyPassword(hash, "my-password"); err != nil {
		t.Errorf("correct password should verify: %v", err)
	}
	if err := VerifyPassword(hash, "wrong-password"); err == nil {
		t.Error("wrong password should not verify")
	}
}

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if len(token) != 64 {
		t.Errorf("token length = %d, want 64", len(token))
	}

	// Generate another token, should be different.
	token2, _ := GenerateToken()
	if token == token2 {
		t.Error("two generated tokens should be different")
	}
}

func TestAuthSettings_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     types.AuthSettings
		wantErr bool
	}{
		{
			name: "disabled is valid",
			cfg:  types.AuthSettings{Enabled: false},
		},
		{
			name: "basic with password is valid",
			cfg: types.AuthSettings{
				Enabled:  true,
				Mode:     types.AuthModeBasic,
				Username: "admin",
				Password: "secret",
			},
		},
		{
			name: "basic with hash is valid",
			cfg: types.AuthSettings{
				Enabled:      true,
				Mode:         types.AuthModeBasic,
				Username:     "admin",
				PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
			},
		},
		{
			name: "token is valid",
			cfg: types.AuthSettings{
				Enabled: true,
				Mode:    types.AuthModeToken,
				Token:   "my-secret-token-1234567890abcdef",
			},
		},
		{
			name: "basic missing username",
			cfg: types.AuthSettings{
				Enabled:  true,
				Mode:     types.AuthModeBasic,
				Password: "secret",
			},
			wantErr: true,
		},
		{
			name: "basic missing password and hash",
			cfg: types.AuthSettings{
				Enabled:  true,
				Mode:     types.AuthModeBasic,
				Username: "admin",
			},
			wantErr: true,
		},
		{
			name: "basic both password and hash",
			cfg: types.AuthSettings{
				Enabled:      true,
				Mode:         types.AuthModeBasic,
				Username:     "admin",
				Password:     "secret",
				PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
			},
			wantErr: true,
		},
		{
			name: "token missing",
			cfg: types.AuthSettings{
				Enabled: true,
				Mode:    types.AuthModeToken,
			},
			wantErr: true,
		},
		{
			name: "token too short",
			cfg: types.AuthSettings{
				Enabled: true,
				Mode:    types.AuthModeToken,
				Token:   "short",
			},
			wantErr: true,
		},
		{
			name: "invalid mode",
			cfg: types.AuthSettings{
				Enabled: true,
				Mode:    "oauth",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
