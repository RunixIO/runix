package types

import (
	"testing"
	"time"
)

func TestProcessConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ProcessConfig
		wantErr bool
	}{
		{
			name:    "missing name",
			cfg:     ProcessConfig{Entrypoint: "app.py"},
			wantErr: true,
		},
		{
			name:    "missing entrypoint",
			cfg:     ProcessConfig{Name: "app"},
			wantErr: true,
		},
		{
			name:    "valid config",
			cfg:     ProcessConfig{Name: "app", Entrypoint: "app.py"},
			wantErr: false,
		},
		{
			name: "valid wait ready config",
			cfg: ProcessConfig{
				Name:          "app",
				Entrypoint:    "app.py",
				WaitReady:     true,
				ListenTimeout: time.Second,
			},
			wantErr: false,
		},
		{
			name:    "invalid restart policy",
			cfg:     ProcessConfig{Name: "app", Entrypoint: "app.py", RestartPolicy: "bad"},
			wantErr: true,
		},
		{
			name:    "valid restart policy always",
			cfg:     ProcessConfig{Name: "app", Entrypoint: "app.py", RestartPolicy: RestartAlways},
			wantErr: false,
		},
		{
			name:    "negative restart delay",
			cfg:     ProcessConfig{Name: "app", Entrypoint: "app.py", RestartDelay: -time.Second},
			wantErr: true,
		},
		{
			name:    "negative min uptime",
			cfg:     ProcessConfig{Name: "app", Entrypoint: "app.py", MinUptime: -time.Second},
			wantErr: true,
		},
		{
			name:    "invalid max memory restart",
			cfg:     ProcessConfig{Name: "app", Entrypoint: "app.py", MaxMemoryRestart: "bad"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseMemorySize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "mb", input: "512MB", want: 512 * 1024 * 1024},
		{name: "gb", input: "1GB", want: 1024 * 1024 * 1024},
		{name: "kb", input: "1024KB", want: 1024 * 1024},
		{name: "bytes", input: "2048", want: 2048},
		{name: "lowercase", input: "256mb", want: 256 * 1024 * 1024},
		{name: "invalid", input: "bad", wantErr: true},
		{name: "zero", input: "0MB", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMemorySize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseMemorySize(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("ParseMemorySize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestCronJobConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     CronJobConfig
		wantErr bool
	}{
		{
			name:    "missing name",
			cfg:     CronJobConfig{Schedule: "* * * * *", Command: "echo hi"},
			wantErr: true,
		},
		{
			name:    "missing schedule",
			cfg:     CronJobConfig{Name: "test", Command: "echo hi"},
			wantErr: true,
		},
		{
			name:    "missing command",
			cfg:     CronJobConfig{Name: "test", Schedule: "* * * * *"},
			wantErr: true,
		},
		{
			name:    "valid",
			cfg:     CronJobConfig{Name: "test", Schedule: "* * * * *", Command: "echo hi"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
