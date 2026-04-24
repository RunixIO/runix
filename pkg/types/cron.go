package types

import (
	"fmt"
	"time"
)

// CronJobConfig defines a scheduled job.
type CronJobConfig struct {
	Name     string            `json:"name" yaml:"name"`
	Schedule string            `json:"schedule" yaml:"schedule"`
	Runtime  string            `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	Command  string            `json:"command" yaml:"command"`
	Cwd      string            `json:"cwd,omitempty" yaml:"cwd,omitempty"`
	Env      map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	Timeout  time.Duration     `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Enabled  bool              `json:"enabled" yaml:"enabled"`
}

// Validate checks the cron job config for errors.
func (c *CronJobConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("cron job name is required")
	}
	if c.Schedule == "" {
		return fmt.Errorf("schedule is required for cron job %q", c.Name)
	}
	if c.Command == "" {
		return fmt.Errorf("command is required for cron job %q", c.Name)
	}
	return nil
}
