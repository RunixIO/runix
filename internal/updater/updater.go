package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/internal/version"
)

const (
	githubAPI = "https://api.github.com/repos/runixio/runix/releases/latest"
	userAgent = "runix-self-update"
)

// Release represents a GitHub release.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset represents a release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// CheckResult holds the outcome of a version check.
type CheckResult struct {
	CurrentVersion string
	LatestVersion  string
	HasUpdate      bool
	ReleaseURL     string
}

// httpClient is the HTTP client used for requests (testable).
var httpClient = &http.Client{Timeout: 30 * time.Second}

// CheckForUpdate queries the GitHub API for the latest release.
func CheckForUpdate(ctx context.Context) (*CheckResult, error) {
	current := normalizeVersion(version.Version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("checking for updates: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parsing release: %w", err)
	}

	latest := normalizeVersion(release.TagName)

	return &CheckResult{
		CurrentVersion: current,
		LatestVersion:  latest,
		HasUpdate:      current != latest && current != "dev",
		ReleaseURL:     fmt.Sprintf("https://github.com/runixio/runix/releases/tag/%s", release.TagName),
	}, nil
}

// SelfUpdate downloads the latest release and replaces the current binary.
func SelfUpdate(ctx context.Context, targetVersion string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	// Determine the asset name for this platform.
	assetName := fmt.Sprintf("runix_%s_%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}

	var downloadURL string
	var assetSize int64

	if targetVersion != "" {
		// Specific version requested.
		downloadURL = fmt.Sprintf(
			"https://github.com/runixio/runix/releases/download/%s/%s",
			targetVersion, assetName,
		)
	} else {
		// Find latest release asset.
		url, size, err := findLatestAsset(ctx, assetName)
		if err != nil {
			return err
		}
		downloadURL = url
		assetSize = size
	}

	log.Info().Str("url", downloadURL).Msg("downloading update")

	tmpFile, err := os.CreateTemp("", "runix-update-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := downloadFile(ctx, tmpFile, downloadURL, assetSize); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("downloading: %w", err)
	}
	_ = tmpFile.Close()

	// Verify checksum if available.
	if targetVersion == "" {
		if err := verifyChecksum(ctx, tmpPath, assetName); err != nil {
			log.Warn().Err(err).Msg("checksum verification skipped or failed")
		}
	}

	// Make binary executable.
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}

	// Replace the binary.
	if err := os.Rename(tmpPath, exePath); err != nil {
		// Cross-device rename won't work; fall back to copy.
		if err := copyFile(tmpPath, exePath); err != nil {
			return fmt.Errorf("replacing binary: %w", err)
		}
		_ = os.Remove(tmpPath)
	}

	log.Info().Str("path", exePath).Msg("binary updated successfully")
	return nil
}

// findLatestAsset finds the download URL for the matching platform asset.
func findLatestAsset(ctx context.Context, assetName string) (string, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPI, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", 0, fmt.Errorf("parsing release: %w", err)
	}

	for _, a := range release.Assets {
		if a.Name == assetName {
			return a.BrowserDownloadURL, a.Size, nil
		}
	}

	return "", 0, fmt.Errorf("no matching asset found for %s in release %s", assetName, release.TagName)
}

// downloadFile downloads a file from url into dst.
func downloadFile(ctx context.Context, dst *os.File, url string, expectedSize int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	written, err := io.Copy(dst, resp.Body)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	if expectedSize > 0 && written != expectedSize {
		return fmt.Errorf("download incomplete: got %d bytes, expected %d", written, expectedSize)
	}

	return nil
}

// copyFile copies src to dst, replacing dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// normalizeVersion strips the leading "v" from version strings.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}
