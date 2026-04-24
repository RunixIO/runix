package sdk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/runixio/runix/internal/metrics"
	"github.com/runixio/runix/internal/supervisor"
	"github.com/runixio/runix/pkg/types"
)

// Manager wraps the Runix supervisor for embedded use. It provides a
// developer-friendly API for managing process lifecycles without requiring
// the CLI binary or a separate daemon.
type Manager struct {
	sup  *supervisor.Supervisor
	mcol *metrics.Collector
}

// New creates a new Manager with the given configuration. It initializes the
// underlying supervisor and optionally restores previously saved processes.
func New(cfg Config) (*Manager, error) {
	logDir := cfg.LogDir
	if logDir == "" {
		logDir = filepath.Join(os.TempDir(), "runix")
	}

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("sdk: failed to create log directory: %w", err)
	}

	col := metrics.NewCollector()
	col.Start(5 * time.Second)

	sup := supervisor.New(supervisor.Options{
		LogDir:           logDir,
		Defaults:         toTypesDefaults(cfg.Defaults),
		MetricsCollector: col,
		MetricsInterval:  5 * time.Second,
	})

	return &Manager{sup: sup, mcol: col}, nil
}

// AddProcess creates, registers, and starts a new managed process. It resolves
// the runtime adapter if Runtime is set, derives the correct interpreter and
// entrypoint, and starts the process. Returns the unique process ID.
func (m *Manager) AddProcess(ctx context.Context, cfg ProcessConfig) (string, error) {
	internal, err := toTypesConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("sdk: invalid config: %w", err)
	}

	if internal.Instances < 1 {
		internal.Instances = 1
	}

	// For multi-instance, start all copies. Return the first process ID.
	var firstID string
	var startedIDs []string
	for i := 0; i < internal.Instances; i++ {
		instanceCfg := internal
		instanceCfg.InstanceIndex = i
		if internal.Instances > 1 {
			instanceCfg.Name = fmt.Sprintf("%s:%d", internal.Name, i)
		}

		proc, err := m.sup.AddProcess(ctx, instanceCfg)
		if err != nil {
			// Clean up all previously started instances.
			for _, id := range startedIDs {
				_ = m.sup.RemoveProcess(id)
			}
			return "", fmt.Errorf("sdk: failed to add process %q: %w", instanceCfg.Name, err)
		}
		startedIDs = append(startedIDs, proc.ID)
		if firstID == "" {
			firstID = proc.ID
		}
	}

	return firstID, nil
}

// Stop gracefully stops a process by ID or name. It sends the configured stop
// signal and waits up to timeout for the process to exit before force-killing.
func (m *Manager) Stop(id string, timeout time.Duration) error {
	if err := m.sup.StopProcess(id, false, timeout); err != nil {
		return fmt.Errorf("sdk: stop %q: %w", id, err)
	}
	return nil
}

// ForceStop immediately sends SIGKILL to the process group.
func (m *Manager) ForceStop(id string) error {
	if err := m.sup.StopProcess(id, true, 0); err != nil {
		return fmt.Errorf("sdk: force stop %q: %w", id, err)
	}
	return nil
}

// Restart stops and restarts a process. It fires pre/post restart hooks.
func (m *Manager) Restart(ctx context.Context, id string) error {
	if err := m.sup.RestartProcess(ctx, id); err != nil {
		return fmt.Errorf("sdk: restart %q: %w", id, err)
	}
	return nil
}

// Reload performs a graceful reload (distinct from restart). It fires
// pre/post reload hooks instead of restart hooks.
func (m *Manager) Reload(ctx context.Context, id string) error {
	if err := m.sup.ReloadProcess(ctx, id); err != nil {
		return fmt.Errorf("sdk: reload %q: %w", id, err)
	}
	return nil
}

// Remove stops the process if running and removes it from the manager.
func (m *Manager) Remove(id string) error {
	if err := m.sup.RemoveProcess(id); err != nil {
		return fmt.Errorf("sdk: remove %q: %w", id, err)
	}
	return nil
}

// List returns information for all managed processes, sorted by numeric ID.
func (m *Manager) List() []ProcessInfo {
	infos := m.sup.List()
	result := make([]ProcessInfo, len(infos))
	for i, info := range infos {
		result[i] = toSDKProcessInfo(info)
	}
	return result
}

// Inspect returns detailed information about a single process.
// Accepts process ID (UUID), numeric ID, name, or unique ID prefix.
func (m *Manager) Inspect(id string) (*ProcessInfo, error) {
	proc, err := m.sup.Get(id)
	if err != nil {
		return nil, fmt.Errorf("sdk: inspect %q: %w", id, err)
	}
	info := toSDKProcessInfo(proc.Info())
	return &info, nil
}

// Save persists the current process table to disk for later restoration.
func (m *Manager) Save() error {
	if err := m.sup.Save(); err != nil {
		return fmt.Errorf("sdk: save: %w", err)
	}
	return nil
}

// Resurrect restores previously saved processes from disk and starts them.
// Call this after New() to recover from a previous session.
func (m *Manager) Resurrect() error {
	if err := m.sup.Resurrect(); err != nil {
		return fmt.Errorf("sdk: resurrect: %w", err)
	}
	return nil
}

// Close gracefully stops all managed processes and cleans up resources.
// It is safe to call Close() multiple times.
func (m *Manager) Close() error {
	err := m.sup.Shutdown()
	if m.mcol != nil {
		m.mcol.Stop()
	}
	if err != nil {
		return fmt.Errorf("sdk: close: %w", err)
	}
	return nil
}

// LogPath returns the stdout log file path for a process.
func (m *Manager) LogPath(id string) string {
	return m.sup.LogPath(id)
}

// LogPathStderr returns the stderr log file path for a process.
func (m *Manager) LogPathStderr(id string) string {
	return m.sup.LogPathStderr(id)
}

// state converts a string state to types.ProcessState for internal use.
func state(s string) types.ProcessState {
	return types.ProcessState(s)
}
