package updater

import (
	"os"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v1.0.0", "1.0.0"},
		{"1.0.0", "1.0.0"},
		{"dev", "dev"},
		{"v0.1.0-rc1", "0.1.0-rc1"},
	}

	for _, tt := range tests {
		got := normalizeVersion(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFindChecksum(t *testing.T) {
	checksums := `a1b2c3  runix_linux_amd64
d4e5f6  runix_darwin_arm64
g7h8i9  runix_windows_amd64.exe
`

	hash, err := findChecksum(checksums, "runix_darwin_arm64")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "d4e5f6" {
		t.Errorf("expected d4e5f6, got %q", hash)
	}

	_, err = findChecksum(checksums, "runix_freebsd_amd64")
	if err == nil {
		t.Error("expected error for missing filename")
	}
}

func TestFindChecksumEmpty(t *testing.T) {
	_, err := findChecksum("", "runix_linux_amd64")
	if err == nil {
		t.Error("expected error for empty checksums")
	}
}

func TestSHA256File(t *testing.T) {
	// Create a temp file with known content.
	tmp := t.TempDir()
	path := tmp + "/test.txt"
	content := "hello world"
	if err := writeTestFile(path, content); err != nil {
		t.Fatal(err)
	}

	hash, err := sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hash) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars", len(hash))
	}
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
