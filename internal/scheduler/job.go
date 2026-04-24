package scheduler

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/pkg/types"
)

const maxCapturedCronOutput = 64 * 1024 // 64 KiB tail across stdout/stderr

// Job represents a scheduled cron job.
type Job struct {
	Config  types.CronJobConfig
	entryID cron.EntryID
	enabled bool

	mu       sync.Mutex
	lastRun  time.Time
	runCount int
	runner   JobRunner
}

// Run executes the job command.
func (j *Job) Run() {
	j.mu.Lock()
	j.lastRun = time.Now()
	j.runCount++
	count := j.runCount
	j.mu.Unlock()

	log.Info().
		Str("name", j.Config.Name).
		Str("command", j.Config.Command).
		Int("run_count", count).
		Msg("executing cron job")

	if j.runner != nil {
		if err := j.runner.RunJob(j.Config); err != nil {
			log.Error().
				Str("name", j.Config.Name).
				Err(err).
				Msg("cron job execution failed")
		}
		return
	}

	// Fallback: direct execution.
	j.runDirect()
}

// runDirect executes the command via exec.CommandContext.
func (j *Job) runDirect() {
	ctx := context.Background()
	timeout := j.Config.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", j.Config.Command)
	setProcessGroup(cmd)
	if j.Config.Cwd != "" {
		cmd.Dir = j.Config.Cwd
	}
	if len(j.Config.Env) > 0 {
		cmd.Env = buildCronEnv(j.Config.Env)
	}

	output := newBoundedTailBuffer(maxCapturedCronOutput)
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()
	outputStr := strings.TrimSpace(output.String())
	if err != nil {
		event := log.Error().
			Str("name", j.Config.Name).
			Str("command", j.Config.Command).
			Err(err)
		if outputStr != "" {
			event = event.Str("output", outputStr)
		}
		event.Msg("cron job command failed")
		return
	}

	event := log.Info().
		Str("name", j.Config.Name).
		Int("output_bytes", output.total)
	if output.total > maxCapturedCronOutput {
		event = event.Bool("output_truncated", true)
	}
	event.Msg("cron job completed")
}

// buildCronEnv builds an environment list from the host env plus overlay.
func buildCronEnv(overlay map[string]string) []string {
	env := os.Environ()
	if len(overlay) == 0 {
		return env
	}

	prefixes := make(map[string]string, len(overlay))
	for k, v := range overlay {
		prefixes[k+"="] = k + "=" + v
	}

	result := make([]string, 0, len(env)+len(overlay))
	for _, e := range env {
		replaced := false
		for prefix := range prefixes {
			if strings.HasPrefix(e, prefix) {
				result = append(result, prefixes[prefix])
				delete(prefixes, prefix)
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, e)
		}
	}

	for _, v := range prefixes {
		result = append(result, v)
	}

	return result
}

type boundedTailBuffer struct {
	max   int
	buf   []byte
	total int
}

func newBoundedTailBuffer(max int) boundedTailBuffer {
	return boundedTailBuffer{max: max}
}

func (b *boundedTailBuffer) Write(p []byte) (int, error) {
	b.total += len(p)
	if b.max <= 0 {
		return len(p), nil
	}
	if len(p) >= b.max {
		b.buf = append(b.buf[:0], p[len(p)-b.max:]...)
		return len(p), nil
	}

	needed := len(b.buf) + len(p) - b.max
	if needed > 0 {
		copy(b.buf, b.buf[needed:])
		b.buf = b.buf[:len(b.buf)-needed]
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *boundedTailBuffer) String() string {
	return string(b.buf)
}

// JobInfo holds runtime information about a cron job.
type JobInfo struct {
	Name     string    `json:"name"`
	Schedule string    `json:"schedule"`
	Command  string    `json:"command"`
	Enabled  bool      `json:"enabled"`
	LastRun  time.Time `json:"last_run,omitempty"`
	RunCount int       `json:"run_count"`
}
