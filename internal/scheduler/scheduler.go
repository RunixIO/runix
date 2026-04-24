package scheduler

import (
	"fmt"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/pkg/types"
)

// Scheduler manages cron jobs using robfig/cron.
type Scheduler struct {
	cron   *cron.Cron
	jobs   map[string]*Job
	mu     sync.RWMutex
	runner JobRunner
	wg     sync.WaitGroup // tracks in-flight manual job runs
}

// JobRunner is the interface for executing job commands.
type JobRunner interface {
	RunJob(cfg types.CronJobConfig) error
}

// New creates a new Scheduler with the given job runner.
func New(runner JobRunner) *Scheduler {
	return &Scheduler{
		cron:   cron.New(cron.WithSeconds()),
		jobs:   make(map[string]*Job),
		runner: runner,
	}
}

// Start starts the cron scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
	log.Info().Msg("cron scheduler started")
}

// Stop stops the cron scheduler, waits for in-flight jobs, and removes all jobs.
func (s *Scheduler) Stop() {
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("cron scheduler stopped")
}

// AddJob registers a cron job. If the job is enabled, it's scheduled immediately.
func (s *Scheduler) AddJob(cfg types.CronJobConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing job with the same name.
	if existing, ok := s.jobs[cfg.Name]; ok {
		s.cron.Remove(existing.entryID)
		delete(s.jobs, cfg.Name)
	}

	job := &Job{
		Config: cfg,
		runner: s.runner,
	}

	// robfig/cron with seconds: "sec min hour dom month dow"
	// If the schedule has 5 fields (standard cron), prepend "0 " for seconds.
	schedule := cfg.Schedule
	if !hasSecondsField(schedule) {
		schedule = "0 " + schedule
	}

	entryID, err := s.cron.AddFunc(schedule, job.Run)
	if err != nil {
		return fmt.Errorf("invalid cron schedule %q: %w", cfg.Schedule, err)
	}

	job.entryID = entryID
	s.jobs[cfg.Name] = job

	if !cfg.Enabled {
		s.cron.Remove(entryID)
		job.enabled = false
	}

	log.Info().
		Str("name", cfg.Name).
		Str("schedule", cfg.Schedule).
		Bool("enabled", cfg.Enabled).
		Msg("cron job registered")

	return nil
}

// RemoveJob removes a cron job by name.
func (s *Scheduler) RemoveJob(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[name]
	if !ok {
		return fmt.Errorf("cron job %q not found", name)
	}

	s.cron.Remove(job.entryID)
	delete(s.jobs, name)

	log.Info().Str("name", name).Msg("cron job removed")
	return nil
}

// EnableJob enables a disabled cron job.
func (s *Scheduler) EnableJob(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[name]
	if !ok {
		return fmt.Errorf("cron job %q not found", name)
	}

	if !job.enabled {
		schedule := job.Config.Schedule
		if !hasSecondsField(schedule) {
			schedule = "0 " + schedule
		}
		entryID, err := s.cron.AddFunc(schedule, job.Run)
		if err != nil {
			return fmt.Errorf("invalid cron schedule: %w", err)
		}
		job.entryID = entryID
		job.enabled = true
	}

	job.Config.Enabled = true
	log.Info().Str("name", name).Msg("cron job enabled")
	return nil
}

// DisableJob disables a cron job.
func (s *Scheduler) DisableJob(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[name]
	if !ok {
		return fmt.Errorf("cron job %q not found", name)
	}

	if job.enabled {
		s.cron.Remove(job.entryID)
		job.enabled = false
	}

	job.Config.Enabled = false
	log.Info().Str("name", name).Msg("cron job disabled")
	return nil
}

// RunJob manually triggers a cron job by name.
func (s *Scheduler) RunJob(name string) error {
	s.mu.RLock()
	job, ok := s.jobs[name]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("cron job %q not found", name)
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		job.Run()
	}()
	log.Info().Str("name", name).Msg("cron job triggered manually")
	return nil
}

// ListJobs returns info about all registered cron jobs.
func (s *Scheduler) ListJobs() []JobInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]JobInfo, 0, len(s.jobs))
	for _, job := range s.jobs {
		result = append(result, JobInfo{
			Name:     job.Config.Name,
			Schedule: job.Config.Schedule,
			Command:  job.Config.Command,
			Enabled:  job.enabled,
			LastRun:  job.lastRun,
			RunCount: job.runCount,
		})
	}
	return result
}

// GetJob returns info about a specific job.
func (s *Scheduler) GetJob(name string) (JobInfo, error) {
	s.mu.RLock()
	job, ok := s.jobs[name]
	s.mu.RUnlock()

	if !ok {
		return JobInfo{}, fmt.Errorf("cron job %q not found", name)
	}

	return JobInfo{
		Name:     job.Config.Name,
		Schedule: job.Config.Schedule,
		Command:  job.Config.Command,
		Enabled:  job.enabled,
		LastRun:  job.lastRun,
		RunCount: job.runCount,
	}, nil
}

// hasSecondsField checks if the cron expression includes a seconds field
// (i.e., has 6 space-separated fields).
func hasSecondsField(schedule string) bool {
	count := 0
	for _, c := range schedule {
		if c == ' ' {
			count++
		}
	}
	return count >= 5
}
