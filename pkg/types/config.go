package types

import (
	"fmt"
	"time"
)

// RunixConfig is the top-level configuration for Runix.
type RunixConfig struct {
	Daemon    DaemonConfig                            `json:"daemon" yaml:"daemon"`
	Defaults  DefaultsConfig                          `json:"defaults" yaml:"defaults"`
	Processes []ProcessConfig                         `json:"processes" yaml:"processes"`
	Cron      []CronJobConfig                         `json:"cron" yaml:"cron"`
	Deploy    map[string]DeployTarget                 `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	Web       WebConfig                               `json:"web" yaml:"web"`
	MCP       MCPConfig                               `json:"mcp" yaml:"mcp"`
	Security  SecurityConfig                          `json:"security,omitempty" yaml:"security,omitempty"`
	Metrics   MetricsConfig                           `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	Secrets   map[string]SecretRef                    `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Profiles  map[string]map[string]map[string]string `json:"profiles,omitempty" yaml:"profiles,omitempty"`
}

// DaemonConfig holds daemon-specific settings.
type DaemonConfig struct {
	SocketPath string `json:"socket_path,omitempty" yaml:"socket_path,omitempty"`
	PidDir     string `json:"pid_dir,omitempty" yaml:"pid_dir,omitempty"`
	DataDir    string `json:"data_dir,omitempty" yaml:"data_dir,omitempty"`
	LogLevel   string `json:"log_level,omitempty" yaml:"log_level,omitempty"`
}

// DeployTarget holds remote deployment settings for `runix deploy`.
type DeployTarget struct {
	Host          string `json:"host,omitempty" yaml:"host,omitempty"`
	User          string `json:"user,omitempty" yaml:"user,omitempty"`
	Port          int    `json:"port,omitempty" yaml:"port,omitempty"`
	Path          string `json:"path,omitempty" yaml:"path,omitempty"`
	PreDeploy     string `json:"pre_deploy,omitempty" yaml:"pre_deploy,omitempty"`
	PostDeploy    string `json:"post_deploy,omitempty" yaml:"post_deploy,omitempty"`
	ReloadCommand string `json:"reload_command,omitempty" yaml:"reload_command,omitempty"`
}

// DefaultsConfig holds default values for process configs.
type DefaultsConfig struct {
	RestartPolicy    RestartPolicy `json:"restart_policy,omitempty" yaml:"restart_policy,omitempty"`
	AutoRestart      *bool         `json:"autorestart,omitempty" yaml:"autorestart,omitempty"`
	MaxRestarts      int           `json:"max_restarts,omitempty" yaml:"max_restarts,omitempty"`
	RestartWindow    time.Duration `json:"restart_window,omitempty" yaml:"restart_window,omitempty"`
	RestartDelay     time.Duration `json:"restart_delay,omitempty" yaml:"restart_delay,omitempty"`
	MinUptime        time.Duration `json:"min_uptime,omitempty" yaml:"min_uptime,omitempty"`
	MaxMemoryRestart string        `json:"max_memory_restart,omitempty" yaml:"max_memory_restart,omitempty"`
	BackoffBase      time.Duration `json:"backoff_base,omitempty" yaml:"backoff_base,omitempty"`
	BackoffMax       time.Duration `json:"backoff_max,omitempty" yaml:"backoff_max,omitempty"`
	LogMaxSize       int64         `json:"log_max_size,omitempty" yaml:"log_max_size,omitempty"`
	LogMaxAge        time.Duration `json:"log_max_age,omitempty" yaml:"log_max_age,omitempty"`
	WatchDebounce    time.Duration `json:"watch_debounce,omitempty" yaml:"watch_debounce,omitempty"`
}

// WebConfig holds Web UI settings.
type WebConfig struct {
	Enabled bool       `json:"enabled" yaml:"enabled"`
	Listen  string     `json:"listen,omitempty" yaml:"listen,omitempty"`
	Auth    AuthConfig `json:"auth" yaml:"auth"`
}

// AuthConfig holds authentication settings for the Web UI (legacy, use SecurityConfig.Auth instead).
type AuthConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
}

// SecurityConfig holds global security and authentication settings.
type SecurityConfig struct {
	Auth AuthSettings `json:"auth" yaml:"auth"`
}

// AuthMode represents the authentication mode.
type AuthMode string

const (
	AuthModeDisabled AuthMode = "disabled" // No authentication required
	AuthModeBasic    AuthMode = "basic"    // HTTP Basic Auth (username/password)
	AuthModeToken    AuthMode = "token"    // Bearer token or API key
)

// AuthSettings holds authentication configuration for all Runix interfaces.
type AuthSettings struct {
	// Enabled controls whether authentication is active.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Mode specifies the authentication mode: "disabled", "basic", or "token".
	Mode AuthMode `json:"mode,omitempty" yaml:"mode,omitempty"`

	// Username for basic auth.
	Username string `json:"username,omitempty" yaml:"username,omitempty"`

	// Password for basic auth (plain text, for development only).
	Password string `json:"password,omitempty" yaml:"password,omitempty"`

	// PasswordHash is a bcrypt hash of the password (preferred over plain text).
	PasswordHash string `json:"password_hash,omitempty" yaml:"password_hash,omitempty"`

	// Token is the bearer token for token auth.
	Token string `json:"token,omitempty" yaml:"token,omitempty"`

	// LocalOnly allows unauthenticated access from localhost (127.0.0.1, ::1).
	// When true, remote connections still require authentication.
	LocalOnly bool `json:"local_only,omitempty" yaml:"local_only,omitempty"`
}

// Validate checks the auth settings for errors.
func (a *AuthSettings) Validate() error {
	if !a.Enabled {
		return nil
	}

	mode := a.Mode
	if mode == "" {
		mode = AuthModeBasic
	}

	switch mode {
	case AuthModeDisabled:
		// No validation needed.
	case AuthModeBasic:
		if a.Username == "" {
			return fmt.Errorf("security.auth.username is required for basic auth mode")
		}
		if a.Password == "" && a.PasswordHash == "" {
			return fmt.Errorf("security.auth.password or security.auth.password_hash is required for basic auth mode")
		}
		if a.Password != "" && a.PasswordHash != "" {
			return fmt.Errorf("security.auth.password and security.auth.password_hash are mutually exclusive")
		}
	case AuthModeToken:
		if a.Token == "" {
			return fmt.Errorf("security.auth.token is required for token auth mode")
		}
		if len(a.Token) < 16 {
			return fmt.Errorf("security.auth.token must be at least 16 characters")
		}
	default:
		return fmt.Errorf("invalid security.auth.mode %q, must be \"disabled\", \"basic\", or \"token\"", mode)
	}

	return nil
}

// EffectiveMode returns the effective auth mode, defaulting to basic if enabled but not specified.
func (a *AuthSettings) EffectiveMode() AuthMode {
	if !a.Enabled {
		return AuthModeDisabled
	}
	if a.Mode == "" {
		return AuthModeBasic
	}
	return a.Mode
}

// IsEnabled returns true if authentication is enabled and the mode is not disabled.
func (a *AuthSettings) IsEnabled() bool {
	return a.Enabled && a.EffectiveMode() != AuthModeDisabled
}

// MCPConfig holds MCP server settings.
type MCPConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	Transport string `json:"transport,omitempty" yaml:"transport,omitempty"`
	Listen    string `json:"listen,omitempty" yaml:"listen,omitempty"`
}

// MetricsConfig holds metrics collection settings.
type MetricsConfig struct {
	Enabled  bool          `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Interval time.Duration `json:"interval,omitempty" yaml:"interval,omitempty"`
}

// MetricsInterval returns the configured polling interval, defaulting to 5 seconds.
func (c *MetricsConfig) MetricsInterval() time.Duration {
	if c.Interval <= 0 {
		return 5 * time.Second
	}
	return c.Interval
}

// Validate checks the top-level config for errors.
func (c *RunixConfig) Validate() error {
	seen := make(map[string]bool)
	for i, p := range c.Processes {
		if err := p.Validate(); err != nil {
			return fmt.Errorf("process[%d]: %w", i, err)
		}
		if seen[p.Name] {
			return fmt.Errorf("duplicate process name: %q", p.Name)
		}
		seen[p.Name] = true
		if p.Runtime != "" {
			validRuntimes := map[string]bool{
				"go": true, "python": true, "node": true, "bun": true, "deno": true, "ruby": true, "php": true,
				"auto": true, "unknown": true,
			}
			if !validRuntimes[p.Runtime] {
				return fmt.Errorf("invalid runtime %q for process %q", p.Runtime, p.Name)
			}
		}
	}
	for i, cr := range c.Cron {
		if cr.Name == "" {
			return fmt.Errorf("cron[%d]: name is required", i)
		}
		if cr.Schedule == "" {
			return fmt.Errorf("cron[%d]: schedule is required", i)
		}
	}
	for name, d := range c.Deploy {
		if d.Host == "" {
			return fmt.Errorf("deploy.%s.host is required", name)
		}
		if d.User == "" {
			return fmt.Errorf("deploy.%s.user is required", name)
		}
		if d.Path == "" {
			return fmt.Errorf("deploy.%s.path is required", name)
		}
	}

	if err := c.Security.Auth.Validate(); err != nil {
		return err
	}

	return nil
}
