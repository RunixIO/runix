package healthcheck

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/pkg/types"
)

// Checker runs periodic health checks on a process.
type Checker struct {
	config              types.HealthCheckConfig
	onUnhealthy         func()
	cancel              context.CancelFunc
	healthy             atomic.Bool
	consecutiveFailures int
	mu                  sync.Mutex
}

// NewChecker creates a new health checker.
// onUnhealthy is called when the process is deemed unhealthy after retries.
func NewChecker(cfg types.HealthCheckConfig, onUnhealthy func()) *Checker {
	return &Checker{
		config:      cfg,
		onUnhealthy: onUnhealthy,
	}
}

// Start begins the health check loop in a goroutine.
func (c *Checker) Start(ctx context.Context) {
	interval := 10 * time.Second
	if c.config.Interval != "" {
		if d, err := time.ParseDuration(c.config.Interval); err == nil {
			interval = d
		}
	}

	// Wait for grace period before starting checks.
	if c.config.GracePeriod != "" {
		if d, err := time.ParseDuration(c.config.GracePeriod); err == nil {
			log.Debug().Dur("grace_period", d).Msg("health check waiting for grace period")
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return
			}
		}
	}

	checkCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.healthy.Store(true)

	go c.runLoop(checkCtx, interval)
}

func (c *Checker) runLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.Check(ctx); err != nil {
				c.mu.Lock()
				c.consecutiveFailures++
				failures := c.consecutiveFailures
				c.mu.Unlock()

				retries := c.config.Retries
				if retries == 0 {
					retries = 3
				}

				if failures >= retries {
					c.healthy.Store(false)
					log.Warn().
						Int("failures", failures).
						Msg("process marked unhealthy")
					if c.onUnhealthy != nil {
						c.onUnhealthy()
					}
				}
			} else {
				c.mu.Lock()
				c.consecutiveFailures = 0
				c.mu.Unlock()
				c.healthy.Store(true)
			}
		}
	}
}

// Stop cancels the health check loop.
func (c *Checker) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

// IsHealthy returns whether the last check was successful.
func (c *Checker) IsHealthy() bool {
	return c.healthy.Load()
}

// Check performs a single health check.
func (c *Checker) Check(ctx context.Context) error {
	timeout := 5 * time.Second
	if c.config.Timeout != "" {
		if d, err := time.ParseDuration(c.config.Timeout); err == nil {
			timeout = d
		}
	}

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch c.config.Type {
	case types.HealthCheckHTTP:
		return c.checkHTTP(checkCtx)
	case types.HealthCheckTCP:
		return c.checkTCP(checkCtx)
	case types.HealthCheckCommand:
		return c.checkCommand(checkCtx)
	default:
		return nil
	}
}
