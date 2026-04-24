package types

// HealthCheckType defines the type of health check.
type HealthCheckType string

const (
	HealthCheckHTTP    HealthCheckType = "http"
	HealthCheckTCP     HealthCheckType = "tcp"
	HealthCheckCommand HealthCheckType = "command"
)

// HealthCheckConfig configures health checking for a process.
type HealthCheckConfig struct {
	Type        HealthCheckType `json:"type" yaml:"type"`
	URL         string          `json:"url,omitempty" yaml:"url,omitempty"`
	TCPEndpoint string          `json:"tcp_endpoint,omitempty" yaml:"tcp_endpoint,omitempty"`
	Command     string          `json:"command,omitempty" yaml:"command,omitempty"`
	Interval    string          `json:"interval,omitempty" yaml:"interval,omitempty"`
	Timeout     string          `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Retries     int             `json:"retries,omitempty" yaml:"retries,omitempty"`
	GracePeriod string          `json:"grace_period,omitempty" yaml:"grace_period,omitempty"`
}
