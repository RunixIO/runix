package secrets

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/runixio/runix/pkg/types"
)

func TestResolveEnv(t *testing.T) {
	os.Setenv("TEST_SECRET_VAL", "mysecret")
	defer os.Unsetenv("TEST_SECRET_VAL")

	cfg := map[string]types.SecretRef{
		"db_password": {Type: "env", Value: "TEST_SECRET_VAL"},
	}
	result, err := Resolve(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result["db_password"] != "mysecret" {
		t.Fatalf("expected mysecret, got %s", result["db_password"])
	}
}

func TestResolveEnvMissing(t *testing.T) {
	cfg := map[string]types.SecretRef{
		"missing": {Type: "env", Value: "NONEXISTENT_VAR_12345"},
	}
	_, err := Resolve(cfg)
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
}

func TestResolveFile(t *testing.T) {
	f, err := os.CreateTemp("", "secret-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("file_secret_value\n")
	f.Close()

	cfg := map[string]types.SecretRef{
		"api_key": {Type: "file", Value: f.Name()},
	}
	result, err := Resolve(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result["api_key"] != "file_secret_value" {
		t.Fatalf("expected file_secret_value, got %s", result["api_key"])
	}
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"short", "sh*rt"},
		{"ab", "****"},
		{"abcdefgh", "ab****gh"},
		{"secret123456", "se********56"},
	}
	for _, tt := range tests {
		got := MaskValue(tt.input)
		if got != tt.want {
			t.Errorf("MaskValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveVaultKVv2(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/secret/data/path" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Vault-Token"); got != "test-token" {
			t.Fatalf("unexpected token: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"data":{"api_key":"vault-secret"}}}`))
	}))
	defer srv.Close()

	t.Setenv("VAULT_ADDR", srv.URL)
	t.Setenv("VAULT_TOKEN", "test-token")

	cfg := map[string]types.SecretRef{
		"api_key": {Type: "vault", Value: "secret/path#api_key"},
	}
	result, err := Resolve(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result["api_key"] != "vault-secret" {
		t.Fatalf("expected vault-secret, got %q", result["api_key"])
	}
}

func TestResolveVaultKVv2ExplicitDataPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/secret/data/myapp" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"data":{"password":"from-data-path"}}}`))
	}))
	defer srv.Close()

	t.Setenv("VAULT_ADDR", srv.URL)
	t.Setenv("VAULT_TOKEN", "test-token")

	cfg := map[string]types.SecretRef{
		"password": {Type: "vault", Value: "secret/data/myapp#password"},
	}
	result, err := Resolve(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result["password"] != "from-data-path" {
		t.Fatalf("expected from-data-path, got %q", result["password"])
	}
}

func TestResolveVaultKVv1Fallback(t *testing.T) {
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		switch r.URL.Path {
		case "/v1/secret/data/legacy":
			http.Error(w, "not found", http.StatusNotFound)
		case "/v1/secret/legacy":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"password":"legacy-secret"}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	t.Setenv("VAULT_ADDR", srv.URL)
	t.Setenv("VAULT_TOKEN", "test-token")

	cfg := map[string]types.SecretRef{
		"password": {Type: "vault", Value: "secret/legacy#password"},
	}
	result, err := Resolve(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result["password"] != "legacy-secret" {
		t.Fatalf("expected legacy-secret, got %q", result["password"])
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 vault calls, got %d", len(calls))
	}
}

func TestResolveVaultMissingEnv(t *testing.T) {
	t.Setenv("VAULT_ADDR", "")
	t.Setenv("VAULT_TOKEN", "")

	cfg := map[string]types.SecretRef{
		"password": {Type: "vault", Value: "secret/path#password"},
	}
	_, err := Resolve(cfg)
	if err == nil {
		t.Fatal("expected error for missing vault environment")
	}
}
