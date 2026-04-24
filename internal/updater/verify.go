package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// verifyChecksum downloads the SHA256 checksums file and verifies the binary.
func verifyChecksum(ctx context.Context, binaryPath, assetName string) error {
	checksumURL := "https://github.com/runixio/runix/releases/latest/download/checksums.txt"

	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return fmt.Errorf("creating checksum request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("checksums file not available")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checksums download returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return fmt.Errorf("reading checksums: %w", err)
	}

	// Parse checksums file (format: "<hash>  <filename>").
	expectedHash, err := findChecksum(string(body), assetName)
	if err != nil {
		return err
	}

	// Compute the SHA256 of the downloaded file.
	actualHash, err := sha256File(binaryPath)
	if err != nil {
		return fmt.Errorf("hashing binary: %w", err)
	}

	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	log.Info().Str("hash", actualHash[:16]+"...").Msg("checksum verified")
	return nil
}

// findChecksum parses a checksums file and returns the hash for the given filename.
func findChecksum(checksums, filename string) (string, error) {
	for _, line := range strings.Split(checksums, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.TrimSpace(parts[1]) == filename {
			return strings.TrimSpace(parts[0]), nil
		}
	}
	return "", fmt.Errorf("no checksum found for %s", filename)
}

// sha256File computes the SHA256 hex digest of a file.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
