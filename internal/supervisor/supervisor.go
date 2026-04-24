package supervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/internal/hooks"
	"github.com/runixio/runix/internal/metrics"
	"github.com/runixio/runix/pkg/types"
)

// MetricsCollector is the interface the supervisor uses to track process metrics.
// Only the Track/Untrack/Get methods are needed — the caller is responsible for
// creating and starting the underlying collector.
type MetricsCollector interface {
	Track(pid int)
	Untrack(pid int)
	Get(pid int) (metrics.ProcessMetrics, bool)
}

// Options configures a Supervisor.
type Options struct {
	LogDir           string
	Defaults         types.DefaultsConfig
	MetricsCollector MetricsCollector
	MetricsInterval  time.Duration
}

// Supervisor manages a set of processes, handling start, stop, restart, and
// lifecycle monitoring.
type Supervisor struct {
	mu            sync.RWMutex
	procs         map[string]*ManagedProcess    // keyed by UUID
	byName        map[string]string             // name -> UUID
	byNumeric     map[int]string                // numeric ID -> UUID
	monitors      map[string]context.CancelFunc // UUID -> monitor cancel
	restartTimers map[string]*time.Timer        // UUID -> pending restart timer
	nextID        int                           // auto-increment counter for numeric IDs
	defaults      types.DefaultsConfig
	logDir        string
	metrics       MetricsCollector
	metricsStop   chan struct{}
	metricsDone   chan struct{}
}

// New creates a new Supervisor with the given options.
func New(opts Options) *Supervisor {
	s := &Supervisor{
		procs:         make(map[string]*ManagedProcess),
		byName:        make(map[string]string),
		byNumeric:     make(map[int]string),
		monitors:      make(map[string]context.CancelFunc),
		restartTimers: make(map[string]*time.Timer),
		nextID:        0,
		defaults:      opts.Defaults,
		logDir:        opts.LogDir,
		metrics:       opts.MetricsCollector,
		metricsStop:   make(chan struct{}),
		metricsDone:   make(chan struct{}),
	}

	if opts.MetricsCollector != nil && opts.MetricsInterval > 0 {
		go s.runMetricsLoop(opts.MetricsInterval)
	} else {
		close(s.metricsDone)
	}

	return s
}

// AddProcess creates, registers, and starts a new managed process.
func (s *Supervisor) AddProcess(ctx context.Context, cfg types.ProcessConfig) (*ManagedProcess, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid process config: %w", err)
	}

	proc := NewManagedProcess(cfg)
	proc.ApplyDefaults(s.defaults)
	proc.metricsCol = s.metrics

	// Set up log file if logDir is configured.
	if s.logDir != "" {
		if err := s.setupLogWriters(proc); err != nil {
			return nil, fmt.Errorf("failed to set up log for process %q: %w", cfg.Name, err)
		}
	}

	// Register before starting so shutdown can find it.
	s.mu.Lock()
	if _, exists := s.byName[cfg.Name]; exists {
		s.mu.Unlock()
		proc.CloseLogFile()
		return nil, fmt.Errorf("process with name %q already exists", cfg.Name)
	}
	proc.NumericID = s.nextID
	s.nextID++
	s.procs[proc.ID] = proc
	s.byName[cfg.Name] = proc.ID
	s.byNumeric[proc.NumericID] = proc.ID
	s.mu.Unlock()

	if err := s.startManagedProcess(ctx, proc); err != nil {
		// Unregister on failure.
		s.mu.Lock()
		delete(s.procs, proc.ID)
		delete(s.byName, cfg.Name)
		delete(s.byNumeric, proc.NumericID)
		s.mu.Unlock()
		proc.CloseLogFile()
		return nil, err
	}

	log.Info().
		Str("process", cfg.Name).
		Str("id", proc.ID).
		Msg("process added and started")

	return proc, nil
}

// StopProcess stops a managed process by ID or name.
func (s *Supervisor) StopProcess(id string, force bool, timeout time.Duration) error {
	proc, err := s.Get(id)
	if err != nil {
		return err
	}

	// If the process is already in a terminal state, nothing to do.
	state := proc.GetState()
	if state != types.StateRunning && state != types.StateStarting {
		s.cancelMonitor(proc.ID)
		return nil
	}

	if force {
		err := proc.ForceStop()
		s.cancelMonitor(proc.ID)
		return err
	}
	err = proc.Stop(timeout)
	s.cancelMonitor(proc.ID)
	return err
}

// RestartProcess stops and restarts a managed process.
func (s *Supervisor) RestartProcess(ctx context.Context, id string) error {
	proc, err := s.Get(id)
	if err != nil {
		return err
	}

	// Pre-restart hook: can block the restart.
	if proc.Config.Hooks != nil && proc.Config.Hooks.PreRestart != nil {
		hookExec := hooks.NewExecutor()
		if err := hookExec.RunPre(ctx, proc.Config.Hooks.PreRestart, "pre_restart", proc.Config); err != nil {
			return fmt.Errorf("pre_restart hook blocked restart of %q: %w", proc.Config.Name, err)
		}
	}

	// Cancel the old monitor and any pending restart timer before stop.
	s.cancelRestartTimer(proc.ID)
	s.cancelMonitor(proc.ID)

	// Untrack the old PID from metrics before stopping.
	s.untrackMetrics(proc)

	// Stop with default timeout.
	if err := proc.Stop(proc.stopTimeout); err != nil {
		// Already stopped is fine.
		if proc.GetState() != types.StateStopped &&
			proc.GetState() != types.StateCrashed &&
			proc.GetState() != types.StateErrored {
			return fmt.Errorf("failed to stop process %q for restart: %w", proc.Config.Name, err)
		}
	}

	// Restart with backoff delay if there were previous restarts.
	if proc.Restarts > 0 {
		delay := proc.backoff.Next(proc.Restarts - 1)
		log.Info().
			Str("process", proc.Config.Name).
			Str("id", proc.ID).
			Dur("backoff", delay).
			Msg("waiting before restart")
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if err := s.startManagedProcess(ctx, proc); err != nil {
		proc.RecordObservation("restart_failed", "manual restart failed")
		return fmt.Errorf("failed to restart process %q: %w", proc.Config.Name, err)
	}
	proc.RecordObservation("restarted", "manual restart")

	log.Info().
		Str("process", proc.Config.Name).
		Str("id", proc.ID).
		Msg("process restarted")

	// Post-restart hook: non-blocking.
	if proc.Config.Hooks != nil && proc.Config.Hooks.PostRestart != nil {
		hookExec := hooks.NewExecutor()
		hookExec.RunPost(ctx, proc.Config.Hooks.PostRestart, "post_restart", proc.Config)
	}

	return nil
}

// ReloadProcess performs a graceful reload, distinct from a restart.
// It fires reload-specific hooks (pre_reload/post_reload) instead of restart hooks.
func (s *Supervisor) ReloadProcess(ctx context.Context, id string) error {
	proc, err := s.Get(id)
	if err != nil {
		return err
	}

	// Pre-reload hook: can block the reload.
	if proc.Config.Hooks != nil && proc.Config.Hooks.PreReload != nil {
		hookExec := hooks.NewExecutor()
		if err := hookExec.RunPre(ctx, proc.Config.Hooks.PreReload, "pre_reload", proc.Config); err != nil {
			return fmt.Errorf("pre_reload hook blocked reload of %q: %w", proc.Config.Name, err)
		}
	}

	// Cancel the old monitor and any pending restart timer before stop.
	s.cancelRestartTimer(proc.ID)
	s.cancelMonitor(proc.ID)

	// Untrack the old PID from metrics before stopping.
	s.untrackMetrics(proc)

	// Stop the current process.
	if err := proc.Stop(proc.stopTimeout); err != nil {
		if proc.GetState() != types.StateStopped &&
			proc.GetState() != types.StateCrashed &&
			proc.GetState() != types.StateErrored {
			return fmt.Errorf("failed to stop process %q for reload: %w", proc.Config.Name, err)
		}
	}

	if err := s.startManagedProcess(ctx, proc); err != nil {
		proc.RecordObservation("reload_failed", "manual reload failed")
		return fmt.Errorf("failed to reload process %q: %w", proc.Config.Name, err)
	}
	proc.RecordObservation("reloaded", "manual reload")

	log.Info().
		Str("process", proc.Config.Name).
		Str("id", proc.ID).
		Msg("process reloaded")

	// Post-reload hook: non-blocking.
	if proc.Config.Hooks != nil && proc.Config.Hooks.PostReload != nil {
		hookExec := hooks.NewExecutor()
		hookExec.RunPost(ctx, proc.Config.Hooks.PostReload, "post_reload", proc.Config)
	}

	return nil
}

// RemoveProcess stops the process if running and removes it from the registry.
func (s *Supervisor) RemoveProcess(id string) error {
	proc, err := s.Get(id)
	if err != nil {
		return err
	}

	state := proc.GetState()
	if state == types.StateRunning || state == types.StateStarting {
		if err := proc.Stop(proc.stopTimeout); err != nil {
			log.Warn().
				Str("process", proc.Config.Name).
				Str("id", proc.ID).
				Err(err).
				Msg("error stopping process during removal, force-killing")
			_ = proc.ForceStop()
		}
	}

	// Cancel the monitor and any pending restart timer after the process has fully stopped.
	s.cancelRestartTimer(proc.ID)
	s.cancelMonitor(proc.ID)

	// Stop tracking metrics for this process.
	s.untrackMetrics(proc)

	s.mu.Lock()
	delete(s.procs, proc.ID)
	delete(s.byName, proc.Config.Name)
	delete(s.byNumeric, proc.NumericID)
	delete(s.monitors, proc.ID)
	s.mu.Unlock()

	proc.CloseLogFile()

	log.Info().
		Str("process", proc.Config.Name).
		Str("id", proc.ID).
		Msg("process removed")

	return nil
}

// List returns info for all managed processes sorted by numeric ID.
func (s *Supervisor) List() []types.ProcessInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]types.ProcessInfo, 0, len(s.procs))
	for _, proc := range s.procs {
		result = append(result, proc.Info())
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].NumericID < result[j].NumericID
	})
	return result
}

// WaitUntilReady waits until the named process is considered ready.
func (s *Supervisor) WaitUntilReady(ctx context.Context, id string, timeout time.Duration) error {
	proc, err := s.Get(id)
	if err != nil {
		return err
	}
	return proc.WaitUntilReady(ctx, timeout)
}

// Get looks up a process by exact UUID, numeric ID, name, or UUID prefix.
func (s *Supervisor) Get(idOrName string) (*ManagedProcess, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Exact UUID match.
	if proc, ok := s.procs[idOrName]; ok {
		return proc, nil
	}

	// Numeric ID match.
	if numID, err := strconv.Atoi(idOrName); err == nil {
		if id, ok := s.byNumeric[numID]; ok {
			return s.procs[id], nil
		}
	}

	// Name match.
	if id, ok := s.byName[idOrName]; ok {
		return s.procs[id], nil
	}

	// UUID prefix match.
	matches := make([]*ManagedProcess, 0)
	for id, proc := range s.procs {
		if strings.HasPrefix(id, idOrName) {
			matches = append(matches, proc)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return nil, fmt.Errorf("process %q not found", idOrName)
	default:
		return nil, fmt.Errorf("ambiguous ID prefix %q matches %d processes", idOrName, len(matches))
	}
}

// GetGroup resolves a target to one or more managed processes. Exact UUID,
// numeric ID, exact name, and UUID prefix keep their existing single-process
// semantics. If none match, a base service name like "api" resolves all
// instances named "api:<n>".
func (s *Supervisor) GetGroup(idOrName string) ([]*ManagedProcess, error) {
	if proc, err := s.Get(idOrName); err == nil {
		return []*ManagedProcess{proc}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var matches []*ManagedProcess
	for _, proc := range s.procs {
		if serviceGroupName(proc.Config.Name) == idOrName {
			matches = append(matches, proc)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("process or service group %q not found", idOrName)
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Config.InstanceIndex != matches[j].Config.InstanceIndex {
			return matches[i].Config.InstanceIndex < matches[j].Config.InstanceIndex
		}
		return matches[i].NumericID < matches[j].NumericID
	})

	return matches, nil
}

// GetGroupNames resolves a target to one or more process names.
func (s *Supervisor) GetGroupNames(idOrName string) ([]string, error) {
	procs, err := s.GetGroup(idOrName)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(procs))
	for i, proc := range procs {
		names[i] = proc.Config.Name
	}
	return names, nil
}

func serviceGroupName(name string) string {
	idx := strings.LastIndexByte(name, ':')
	if idx <= 0 || idx == len(name)-1 {
		return name
	}
	for _, r := range name[idx+1:] {
		if !unicode.IsDigit(r) {
			return name
		}
	}
	return name[:idx]
}

// Shutdown stops all managed processes and cleans up.
func (s *Supervisor) Shutdown() error {
	select {
	case <-s.metricsDone:
	default:
		close(s.metricsStop)
		<-s.metricsDone
	}

	s.mu.RLock()
	ids := make([]string, 0, len(s.procs))
	for id := range s.procs {
		ids = append(ids, id)
	}
	s.mu.RUnlock()

	var firstErr error
	for _, id := range ids {
		if err := s.RemoveProcess(id); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *Supervisor) runMetricsLoop(interval time.Duration) {
	defer close(s.metricsDone)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.metricsStop:
			return
		case <-ticker.C:
			s.enforceMemoryLimits()
		}
	}
}

func (s *Supervisor) enforceMemoryLimits() {
	s.mu.RLock()
	procs := make([]*ManagedProcess, 0, len(s.procs))
	for _, proc := range s.procs {
		procs = append(procs, proc)
	}
	s.mu.RUnlock()

	for _, proc := range procs {
		if !proc.autoRestart || proc.maxMemoryRestart <= 0 {
			continue
		}
		if proc.GetState() != types.StateRunning {
			continue
		}
		if s.metrics == nil || proc.PID <= 0 {
			continue
		}

		m, ok := s.metrics.Get(proc.PID)
		if !ok || m.MemBytes <= 0 || m.MemBytes <= proc.maxMemoryRestart {
			continue
		}

		proc.RecordObservation("memory_restart", fmt.Sprintf("memory usage %d exceeded limit %d", m.MemBytes, proc.maxMemoryRestart))
		log.Warn().
			Str("process", proc.Config.Name).
			Str("id", proc.ID).
			Int64("memory_bytes", m.MemBytes).
			Int64("max_memory_restart", proc.maxMemoryRestart).
			Msg("process exceeded memory restart threshold")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := s.RestartProcess(ctx, proc.ID); err != nil {
			log.Error().
				Str("process", proc.Config.Name).
				Str("id", proc.ID).
				Err(err).
				Msg("memory-threshold restart failed")
		} else {
			proc.RecordObservation("restarted", fmt.Sprintf("memory usage %d exceeded limit %d", m.MemBytes, proc.maxMemoryRestart))
		}
		cancel()
	}
}

// handleExit is the callback invoked by the monitor when a process exits.
// It decides whether to restart the process.
func (s *Supervisor) handleExit(proc *ManagedProcess, exitCode int) {
	if !proc.ExitedBeforeMinUptime() {
		proc.ResetRestartStreak()
	}

	if !proc.ShouldRestart(exitCode) {
		proc.RecordObservation("restart_skipped", fmt.Sprintf("no restart after exit code %d", exitCode))
		log.Info().
			Str("process", proc.Config.Name).
			Str("id", proc.ID).
			Int("exit_code", exitCode).
			Msg("process will not be restarted")
		return
	}

	attempt := proc.recordRestart()
	delay := proc.RestartBackoff(attempt - 1)

	proc.RecordObservation("restart_scheduled", fmt.Sprintf("restart scheduled after exit code %d in %s", exitCode, delay))
	log.Info().
		Str("process", proc.Config.Name).
		Str("id", proc.ID).
		Int("restart", proc.Restarts).
		Dur("backoff", delay).
		Msg("scheduling process restart")

	timer := time.AfterFunc(delay, func() {
		// Clear the timer reference now that it has fired.
		s.mu.Lock()
		delete(s.restartTimers, proc.ID)
		s.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.startManagedProcess(ctx, proc); err != nil {
			proc.RecordObservation("restart_failed", fmt.Sprintf("automatic restart failed after exit code %d", exitCode))
			log.Error().
				Str("process", proc.Config.Name).
				Str("id", proc.ID).
				Err(err).
				Msg("restart failed")
			return
		}
		proc.RecordObservation("restarted", fmt.Sprintf("automatic restart after exit code %d", exitCode))

		log.Info().
			Str("process", proc.Config.Name).
			Str("id", proc.ID).
			Int("restart", proc.Restarts).
			Msg("process restarted after exit")
	})
	s.mu.Lock()
	s.restartTimers[proc.ID] = timer
	s.mu.Unlock()
}

func (s *Supervisor) startManagedProcess(ctx context.Context, proc *ManagedProcess) error {
	if err := proc.Start(ctx); err != nil {
		proc.RecordObservation("start_failed", "process start failed")
		return err
	}

	s.trackMetrics(proc)
	s.startMonitor(ctx, proc)

	if err := proc.WaitForReady(ctx); err != nil {
		log.Error().
			Err(err).
			Str("process", proc.Config.Name).
			Str("id", proc.ID).
			Msg("process failed readiness check during startup")
		if stopErr := proc.Stop(proc.stopTimeout); stopErr != nil {
			log.Warn().
				Err(stopErr).
				Str("process", proc.Config.Name).
				Str("id", proc.ID).
				Msg("failed to stop process after readiness failure, force-killing")
			_ = proc.ForceStop()
		}

		s.cancelRestartTimer(proc.ID)
		s.cancelMonitor(proc.ID)
		s.untrackMetrics(proc)
		proc.SetStateDirect(types.StateErrored)
		proc.RecordObservation("readiness_failed", err.Error())
		return err
	}
	proc.RecordObservation("started", "process ready")

	return nil
}

// trackMetrics registers a process PID with the metrics collector.
func (s *Supervisor) trackMetrics(proc *ManagedProcess) {
	if s.metrics != nil && proc.PID > 0 {
		s.metrics.Track(proc.PID)
	}
}

// untrackMetrics removes a process PID from the metrics collector.
func (s *Supervisor) untrackMetrics(proc *ManagedProcess) {
	if s.metrics != nil && proc.PID > 0 {
		s.metrics.Untrack(proc.PID)
	}
}

// startMonitor launches a goroutine that watches the process for exits.
func (s *Supervisor) startMonitor(ctx context.Context, proc *ManagedProcess) {
	monCtx, monCancel := context.WithCancel(context.Background())

	s.mu.Lock()
	// Cancel any existing monitor for this process.
	if oldCancel, ok := s.monitors[proc.ID]; ok {
		oldCancel()
	}
	s.monitors[proc.ID] = monCancel
	s.mu.Unlock()

	monitor := NewMonitor()
	go monitor.Run(monCtx, proc, s.handleExit)
}

// cancelMonitor cancels the monitor goroutine for a process.
func (s *Supervisor) cancelMonitor(id string) {
	s.mu.Lock()
	cancel, ok := s.monitors[id]
	if ok {
		cancel()
		delete(s.monitors, id)
	}
	s.mu.Unlock()
}

// cancelRestartTimer stops and removes any pending restart timer for a process.
func (s *Supervisor) cancelRestartTimer(id string) {
	s.mu.Lock()
	if t, ok := s.restartTimers[id]; ok {
		t.Stop()
		delete(s.restartTimers, id)
	}
	s.mu.Unlock()
}

// LogPath returns the stdout log file path for a process identified by ID or name.
// Returns empty string if the process is not found.
func (s *Supervisor) LogPath(processID string) string {
	proc, err := s.Get(processID)
	if err != nil {
		return ""
	}
	return s.stdoutLogPath(proc.Config.Name)
}

// LogPathStderr returns the stderr log file path for a process.
func (s *Supervisor) LogPathStderr(processID string) string {
	proc, err := s.Get(processID)
	if err != nil {
		return ""
	}
	return s.stderrLogPath(proc.Config.Name)
}

// AppDir returns the per-app data directory path.
func (s *Supervisor) AppDir(name string) string {
	return filepath.Join(s.logDir, "apps", sanitizeName(name))
}

// sanitizeName strips path traversal sequences from a process name used in file paths.
func sanitizeName(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, name)
}

// Save persists the current process table to disk as dump.json with full metadata.
// Also writes per-app metadata.json files.
func (s *Supervisor) Save() error {
	// Copy process references under lock, then snapshot outside the lock
	// to minimize lock hold time.
	s.mu.RLock()
	procs := make([]*ManagedProcess, 0, len(s.procs))
	for _, proc := range s.procs {
		procs = append(procs, proc)
	}
	s.mu.RUnlock()

	snapshots := make([]processSnapshot, 0, len(procs))
	for _, proc := range procs {
		info := proc.Info()
		snapshots = append(snapshots, processSnapshot{
			Config:    proc.Config,
			NumericID: proc.NumericID,
			Restarts:  proc.Restarts,
			CreatedAt: proc.CreatedAt.Format(time.RFC3339),
			PID:       info.PID,
			State:     string(info.State),
			StartedAt: info.StartedAt,
		})
	}

	// Write per-app metadata files.
	for _, snap := range snapshots {
		if err := s.writeMetadata(snap.Config.Name, snap); err != nil {
			log.Warn().Str("name", snap.Config.Name).Err(err).Msg("failed to write metadata")
		}
	}

	if s.logDir == "" {
		return fmt.Errorf("no data directory configured")
	}

	data, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal process state: %w", err)
	}

	dumpFile := filepath.Join(s.logDir, "dump.json")
	if err := os.MkdirAll(filepath.Dir(dumpFile), 0o755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	tmpFile := dumpFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	if err := os.Rename(tmpFile, dumpFile); err != nil {
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	log.Info().Int("count", len(snapshots)).Str("path", dumpFile).Msg("process state saved")
	return nil
}

// Resurrect restores previously saved processes from disk and starts them.
// Supports both new dump.json and legacy state.json formats.
func (s *Supervisor) Resurrect() error {
	data, source, err := s.readStateFile()
	if err != nil {
		if os.IsNotExist(err) {
			log.Info().Msg("no saved state found")
			return nil
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	ctx := context.Background()

	if source == "dump.json" {
		var snapshots []processSnapshot
		if err := json.Unmarshal(data, &snapshots); err != nil {
			return fmt.Errorf("failed to parse dump.json: %w", err)
		}

		// Restore nextID counter from saved state.
		maxID := -1
		for _, snap := range snapshots {
			if snap.NumericID > maxID {
				maxID = snap.NumericID
			}
		}
		s.mu.Lock()
		s.nextID = maxID + 1
		s.mu.Unlock()

		for _, snap := range snapshots {
			if _, err := s.AddProcess(ctx, snap.Config); err != nil {
				log.Warn().Str("name", snap.Config.Name).Err(err).Msg("failed to resurrect process")
			}
		}
		log.Info().Int("count", len(snapshots)).Msg("processes resurrected")
	} else {
		var configs []types.ProcessConfig
		if err := json.Unmarshal(data, &configs); err != nil {
			return fmt.Errorf("failed to parse state.json: %w", err)
		}
		for _, cfg := range configs {
			if _, err := s.AddProcess(ctx, cfg); err != nil {
				log.Warn().Str("name", cfg.Name).Err(err).Msg("failed to resurrect process")
			}
		}
		log.Info().Int("count", len(configs)).Msg("processes resurrected from legacy state")
	}

	return nil
}

// processSnapshot captures everything needed to persist and restore a process.
type processSnapshot struct {
	Config    types.ProcessConfig `json:"config"`
	NumericID int                 `json:"numeric_id"`
	Restarts  int                 `json:"restarts"`
	CreatedAt string              `json:"created_at"`
	PID       int                 `json:"pid"`
	State     string              `json:"state"`
	StartedAt *time.Time          `json:"started_at,omitempty"`
}

// appMetadata is the per-app metadata file format.
type appMetadata struct {
	Name      string              `json:"name"`
	NumericID int                 `json:"numeric_id"`
	PID       int                 `json:"pid"`
	State     string              `json:"state"`
	Restarts  int                 `json:"restarts"`
	CreatedAt string              `json:"created_at"`
	StartedAt string              `json:"started_at,omitempty"`
	Uptime    string              `json:"uptime,omitempty"`
	Config    types.ProcessConfig `json:"config"`
}

// readStateFile reads dump.json, falling back to legacy state.json.
func (s *Supervisor) readStateFile() ([]byte, string, error) {
	dumpPath := filepath.Join(s.logDir, "dump.json")
	data, err := os.ReadFile(dumpPath)
	if err == nil {
		return data, "dump.json", nil
	}
	if !os.IsNotExist(err) {
		return nil, "", err
	}

	statePath := filepath.Join(s.logDir, "..", "state.json")
	data, err = os.ReadFile(statePath)
	if err != nil {
		return nil, "", err
	}
	return data, "state.json", nil
}

// writeMetadata writes the per-app metadata.json file.
func (s *Supervisor) writeMetadata(name string, snap processSnapshot) error {
	appDir := s.AppDir(name)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return err
	}

	meta := appMetadata{
		Name:      name,
		NumericID: snap.NumericID,
		PID:       snap.PID,
		State:     snap.State,
		Restarts:  snap.Restarts,
		CreatedAt: snap.CreatedAt,
		Config:    snap.Config,
	}
	if snap.StartedAt != nil {
		meta.StartedAt = snap.StartedAt.Format(time.RFC3339)
		meta.Uptime = time.Since(*snap.StartedAt).Round(time.Second).String()
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(appDir, "metadata.json"), data, 0o644)
}

// stdoutLogPath returns the stdout log file path for an app.
func (s *Supervisor) stdoutLogPath(name string) string {
	return filepath.Join(s.AppDir(name), "stdout.log")
}

// stderrLogPath returns the stderr log file path for an app.
func (s *Supervisor) stderrLogPath(name string) string {
	return filepath.Join(s.AppDir(name), "stderr.log")
}

// setupLogWriters creates per-app log directory and separate stdout/stderr files.
func (s *Supervisor) setupLogWriters(proc *ManagedProcess) error {
	appDir := s.AppDir(proc.Config.Name)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return fmt.Errorf("failed to create app directory: %w", err)
	}

	stdoutPath := filepath.Join(appDir, "stdout.log")
	stdoutF, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open stdout log: %w", err)
	}

	stderrPath := filepath.Join(appDir, "stderr.log")
	stderrF, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = stdoutF.Close()
		return fmt.Errorf("failed to open stderr log: %w", err)
	}

	proc.SetLogFiles(stdoutF, stderrF)
	proc.SetLogWriters(&prefixWriter{writer: stdoutF, prefix: "[out]"}, &prefixWriter{writer: stderrF, prefix: "[err]"})

	return nil
}

// prefixWriter buffers partial lines and writes complete lines with a
// timestamp and prefix.
type prefixWriter struct {
	mu     sync.Mutex
	writer io.Writer
	prefix string
	buf    []byte
}

// maxLineBuffer caps the prefixWriter buffer size. Lines exceeding this are
// force-flushed as partial output.
const maxLineBuffer = 64 * 1024 // 64 KB

func (pw *prefixWriter) Write(p []byte) (int, error) {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	pw.buf = append(pw.buf, p...)
	n := len(p)

	// Force-flush if the buffer exceeds the line limit.
	if len(pw.buf) > maxLineBuffer {
		ts := time.Now().Format("2006-01-02 15:04:05")
		_, _ = fmt.Fprintf(pw.writer, "%s %s %s\n", ts, pw.prefix, pw.buf)
		pw.buf = nil // allow GC to reclaim the oversized slice
		return n, nil
	}

	for {
		idx := bytes.IndexByte(pw.buf, '\n')
		if idx < 0 {
			break
		}
		line := pw.buf[:idx]
		pw.buf = pw.buf[idx+1:]

		ts := time.Now().Format("2006-01-02 15:04:05")
		_, _ = fmt.Fprintf(pw.writer, "%s %s %s\n", ts, pw.prefix, line)
	}

	return n, nil
}

func (pw *prefixWriter) Flush() error {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	if len(pw.buf) == 0 {
		return nil
	}

	ts := time.Now().Format("2006-01-02 15:04:05")
	_, err := fmt.Fprintf(pw.writer, "%s %s %s\n", ts, pw.prefix, pw.buf)
	pw.buf = nil // allow GC to reclaim the oversized slice
	return err
}
