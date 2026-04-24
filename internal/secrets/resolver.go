package secrets

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/runixio/runix/pkg/types"
)

var vaultHTTPClient = &http.Client{Timeout: 5 * time.Second}

// Resolve resolves all secret references into concrete values.
func Resolve(cfg map[string]types.SecretRef) (map[string]string, error) {
	result := make(map[string]string)
	for name, ref := range cfg {
		val, err := resolveOne(ref)
		if err != nil {
			return nil, fmt.Errorf("secret %q: %w", name, err)
		}
		result[name] = val
	}
	return result, nil
}

func resolveOne(ref types.SecretRef) (string, error) {
	switch ref.Type {
	case "env":
		val := os.Getenv(ref.Value)
		if val == "" {
			return "", fmt.Errorf("environment variable %q not set", ref.Value)
		}
		return val, nil
	case "file":
		data, err := os.ReadFile(ref.Value)
		if err != nil {
			return "", fmt.Errorf("failed to read file %q: %w", ref.Value, err)
		}
		return strings.TrimSpace(string(data)), nil
	case "vault":
		return resolveVault(ref.Value)
	default:
		return "", fmt.Errorf("unknown secret type %q", ref.Type)
	}
}

func resolveVault(ref string) (string, error) {
	pathPart, key, ok := strings.Cut(ref, "#")
	if !ok || pathPart == "" || key == "" {
		return "", fmt.Errorf("vault value must be in the form path#key")
	}

	addr := strings.TrimRight(os.Getenv("VAULT_ADDR"), "/")
	if addr == "" {
		return "", fmt.Errorf("VAULT_ADDR not set")
	}
	token := os.Getenv("VAULT_TOKEN")
	if token == "" {
		return "", fmt.Errorf("VAULT_TOKEN not set")
	}

	baseURL, err := url.Parse(addr)
	if err != nil {
		return "", fmt.Errorf("invalid VAULT_ADDR %q: %w", addr, err)
	}

	// Try the common KV v2 API shape first, then fall back to KV v1.
	if val, err := readVaultSecret(baseURL, token, vaultV2Path(pathPart), key, true); err == nil {
		return val, nil
	}

	return readVaultSecret(baseURL, token, pathPart, key, false)
}

func vaultV2Path(secretPath string) string {
	parts := strings.SplitN(strings.Trim(secretPath, "/"), "/", 2)
	if len(parts) == 1 {
		return parts[0] + "/data"
	}
	if strings.HasPrefix(parts[1], "data/") || parts[1] == "data" {
		return parts[0] + "/" + parts[1]
	}
	return parts[0] + "/data/" + parts[1]
}

func readVaultSecret(baseURL *url.URL, token, secretPath, key string, expectNestedData bool) (string, error) {
	u := *baseURL
	u.Path = path.Join(baseURL.Path, "/v1", secretPath)

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("build vault request: %w", err)
	}
	req.Header.Set("X-Vault-Token", token)

	resp, err := vaultHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read vault response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vault returned %s", resp.Status)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("decode vault response: %w", err)
	}

	value, err := extractVaultValue(payload, key, expectNestedData)
	if err != nil {
		return "", err
	}
	return value, nil
}

func extractVaultValue(payload map[string]any, key string, expectNestedData bool) (string, error) {
	data, ok := payload["data"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("vault response missing data object")
	}

	if expectNestedData {
		if nested, ok := data["data"].(map[string]any); ok {
			data = nested
		} else {
			return "", fmt.Errorf("vault response missing nested data object")
		}
	}

	raw, ok := data[key]
	if !ok {
		return "", fmt.Errorf("vault secret key %q not found", key)
	}
	return stringifySecretValue(raw)
}

func stringifySecretValue(v any) (string, error) {
	switch val := v.(type) {
	case string:
		return val, nil
	case nil:
		return "", fmt.Errorf("vault secret value is null")
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return "", fmt.Errorf("marshal vault secret value: %w", err)
		}
		return string(data), nil
	}
}

// MaskValue returns a masked representation of a value.
func MaskValue(val string) string {
	if len(val) <= 4 {
		return "****"
	}
	return val[:2] + strings.Repeat("*", len(val)-4) + val[len(val)-2:]
}

// IsSecretEnv checks if an env var name corresponds to a secret.
func IsSecretEnv(key string, secretKeys map[string]bool) bool {
	return secretKeys[key]
}
