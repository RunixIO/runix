package scheduler

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/runixio/runix/pkg/types"
)

// mockRunner records job executions for testing.
type mockRunner struct {
	mu    sync.Mutex
	calls []string
}

func (m *mockRunner) RunJob(cfg types.CronJobConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, cfg.Name)
	return nil
}

func (m *mockRunner) getCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.calls))
	copy(result, m.calls)
	return result
}

func TestSchedulerAddAndRun(t *testing.T) {
	runner := &mockRunner{}
	s := New(runner)
	s.Start()
	defer s.Stop()

	cfg := types.CronJobConfig{
		Name:     "test-job",
		Schedule: "*/1 * * * * *", // every second (with seconds field)
		Command:  "echo hello",
		Enabled:  true,
	}

	if err := s.AddJob(cfg); err != nil {
		t.Fatalf("failed to add job: %v", err)
	}

	// Verify job is listed.
	jobs := s.ListJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "test-job" {
		t.Errorf("expected job name 'test-job', got %q", jobs[0].Name)
	}
}

func TestSchedulerDisableEnable(t *testing.T) {
	runner := &mockRunner{}
	s := New(runner)
	s.Start()
	defer s.Stop()

	cfg := types.CronJobConfig{
		Name:     "toggle-job",
		Schedule: "*/1 * * * * *",
		Command:  "echo toggle",
		Enabled:  true,
	}

	if err := s.AddJob(cfg); err != nil {
		t.Fatalf("failed to add job: %v", err)
	}

	// Disable.
	if err := s.DisableJob("toggle-job"); err != nil {
		t.Fatalf("failed to disable job: %v", err)
	}

	job, err := s.GetJob("toggle-job")
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}
	if job.Enabled {
		t.Error("expected job to be disabled")
	}

	// Enable.
	if err := s.EnableJob("toggle-job"); err != nil {
		t.Fatalf("failed to enable job: %v", err)
	}

	job, _ = s.GetJob("toggle-job")
	if !job.Enabled {
		t.Error("expected job to be enabled")
	}
}

func TestSchedulerRemove(t *testing.T) {
	runner := &mockRunner{}
	s := New(runner)
	s.Start()
	defer s.Stop()

	cfg := types.CronJobConfig{
		Name:     "removable",
		Schedule: "*/1 * * * * *",
		Command:  "echo remove",
		Enabled:  true,
	}

	if err := s.AddJob(cfg); err != nil {
		t.Fatalf("failed to add job: %v", err)
	}

	if err := s.RemoveJob("removable"); err != nil {
		t.Fatalf("failed to remove job: %v", err)
	}

	jobs := s.ListJobs()
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs after removal, got %d", len(jobs))
	}
}

func TestSchedulerRunJob(t *testing.T) {
	runner := &mockRunner{}
	s := New(runner)

	cfg := types.CronJobConfig{
		Name:     "manual-job",
		Schedule: "0 0 31 2 *", // Feb 31 (never fires)
		Command:  "echo manual",
		Enabled:  true,
	}

	if err := s.AddJob(cfg); err != nil {
		t.Fatalf("failed to add job: %v", err)
	}

	if err := s.RunJob("manual-job"); err != nil {
		t.Fatalf("failed to run job: %v", err)
	}

	// Give the goroutine time to execute.
	time.Sleep(100 * time.Millisecond)

	calls := runner.getCalls()
	if len(calls) != 1 || calls[0] != "manual-job" {
		t.Errorf("expected 1 call to 'manual-job', got %v", calls)
	}
}

func TestSchedulerNotFound(t *testing.T) {
	runner := &mockRunner{}
	s := New(runner)

	if err := s.RemoveJob("nonexistent"); err == nil {
		t.Error("expected error removing nonexistent job")
	}

	if err := s.EnableJob("nonexistent"); err == nil {
		t.Error("expected error enabling nonexistent job")
	}

	if err := s.DisableJob("nonexistent"); err == nil {
		t.Error("expected error disabling nonexistent job")
	}

	if err := s.RunJob("nonexistent"); err == nil {
		t.Error("expected error running nonexistent job")
	}
}

func TestSchedulerInvalidSchedule(t *testing.T) {
	runner := &mockRunner{}
	s := New(runner)

	cfg := types.CronJobConfig{
		Name:     "bad-schedule",
		Schedule: "not-a-cron",
		Command:  "echo bad",
		Enabled:  true,
	}

	if err := s.AddJob(cfg); err == nil {
		t.Error("expected error for invalid cron schedule")
	}
}

func TestHasSecondsField(t *testing.T) {
	tests := []struct {
		schedule string
		want     bool
	}{
		{"* * * * *", false},    // 5 fields, standard
		{"*/1 * * * * *", true}, // 6 fields, with seconds
		{"0 0 * * *", false},    // 5 fields
		{"0 0 0 * * *", true},   // 6 fields
		{"0 0 0 1 1 *", true},   // 6 fields
	}

	for _, tt := range tests {
		got := hasSecondsField(tt.schedule)
		if got != tt.want {
			t.Errorf("hasSecondsField(%q) = %v, want %v", tt.schedule, got, tt.want)
		}
	}
}

// Ensure atomic import is available for tests.
var _ = atomic.Int32{}
