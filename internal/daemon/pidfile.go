package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// PIDFile manages a PID file on disk.
type PIDFile struct {
	Path string
}

// NewPIDFile creates a PIDFile reference.
func NewPIDFile(dir string) *PIDFile {
	return &PIDFile{
		Path: filepath.Join(dir, "runix.pid"),
	}
}

// Write writes the current PID to the file. Creates parent dirs.
func (p *PIDFile) Write() error {
	if err := os.MkdirAll(filepath.Dir(p.Path), 0o755); err != nil {
		return fmt.Errorf("failed to create PID directory: %w", err)
	}

	pid := os.Getpid()
	data := strconv.Itoa(pid)
	if err := os.WriteFile(p.Path, []byte(data), 0o644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	return nil
}

// Read reads the PID from the file. Returns 0 if file doesn't exist.
func (p *PIDFile) Read() (int, error) {
	data, err := os.ReadFile(p.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("failed to parse PID: %w", err)
	}
	return pid, nil
}

// Remove deletes the PID file.
func (p *PIDFile) Remove() error {
	err := os.Remove(p.Path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}
	return nil
}

// IsRunning checks if the process with the stored PID is still alive.
func (p *PIDFile) IsRunning() bool {
	pid, err := p.Read()
	if err != nil || pid == 0 {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
