package logrot

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Rotator wraps a log file with size-based rotation.
type Rotator struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	maxSize  int64
	maxFiles int
	maxAge   time.Duration
	written  int64 // bytes written since last rotation
}

// NewRotator creates a new Rotator. The file is opened in append mode.
// maxSize is in bytes (0 = unlimited). maxFiles is count (0 = unlimited). maxAge is duration (0 = unlimited).
func NewRotator(path string, maxSize int64, maxFiles int, maxAge time.Duration) (*Rotator, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &Rotator{
		file:     f,
		path:     path,
		maxSize:  maxSize,
		maxFiles: maxFiles,
		maxAge:   maxAge,
	}, nil
}

// Write writes data to the log file, rotating if needed.
func (r *Rotator) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.maxSize > 0 && r.written+int64(len(p)) >= r.maxSize {
		if rotErr := r.rotate(); rotErr != nil {
			log.Warn().Err(rotErr).Str("path", r.path).Msg("log rotation failed")
		}
	}

	n, err := r.file.Write(p)
	r.written += int64(n)
	return n, err
}

// Close closes the underlying file.
func (r *Rotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Close()
}

// rotate performs the rotation: close current, rename, create new, prune old.
func (r *Rotator) rotate() error {
	if err := r.file.Close(); err != nil {
		return err
	}

	// Rename current file to timestamped backup.
	ts := time.Now().Format("2006-01-02T15-04-05")
	rotated := r.path + "." + ts
	if err := os.Rename(r.path, rotated); err != nil {
		// Try to reopen the original file.
		r.file, _ = os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		return err
	}

	// Open new file.
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	r.file = f
	r.written = 0

	// Prune old files.
	r.prune()

	return nil
}

// prune removes old rotated log files based on maxFiles and maxAge.
func (r *Rotator) prune() {
	dir := filepath.Dir(r.path)
	base := filepath.Base(r.path)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	// Collect rotated files.
	var rotated []os.DirEntry
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), base+".") && entry.Name() != base {
			rotated = append(rotated, entry)
		}
	}

	// Sort by name (which includes timestamp), oldest first.
	sort.Slice(rotated, func(i, j int) bool {
		return rotated[i].Name() < rotated[j].Name()
	})

	// Prune by count.
	if r.maxFiles > 0 && len(rotated) > r.maxFiles {
		for i := 0; i < len(rotated)-r.maxFiles; i++ {
			_ = os.Remove(filepath.Join(dir, rotated[i].Name()))
		}
		rotated = rotated[len(rotated)-r.maxFiles:]
	}

	// Prune by age.
	if r.maxAge > 0 {
		cutoff := time.Now().Add(-r.maxAge)
		for _, entry := range rotated {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				_ = os.Remove(filepath.Join(dir, entry.Name()))
			}
		}
	}
}

// File returns the underlying file (for Sync etc.).
func (r *Rotator) File() *os.File {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file
}
