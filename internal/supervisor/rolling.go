package supervisor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/pkg/types"
)

// RollingReloadOptions configures a rolling reload.
type RollingReloadOptions struct {
	BatchSize         int
	WaitReady         bool
	ReadyTimeout      string
	RollbackOnFailure bool
}

// RollingReload performs a batched rolling reload of the named processes.
func (s *Supervisor) RollingReload(ctx context.Context, names []string, opts RollingReloadOptions) error {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 1
	}

	timeout := 0 * time.Second
	if opts.ReadyTimeout != "" {
		if d, err := time.ParseDuration(opts.ReadyTimeout); err == nil {
			timeout = d
		}
	}

	if opts.WaitReady && len(names) > 1 && opts.BatchSize >= len(names) {
		return fmt.Errorf("rolling reload with wait_ready requires batch_size smaller than total processes")
	}

	originalConfigs := make(map[string]types.ProcessConfig, len(names))
	for _, name := range names {
		proc, err := s.Get(name)
		if err != nil {
			return fmt.Errorf("failed to find process %q for rolling reload: %w", name, err)
		}
		if opts.WaitReady {
			if _, ok := proc.readinessHealthCheck(); !ok {
				return fmt.Errorf("process %q has no readiness check configured", name)
			}
		}
		originalConfigs[name] = proc.Config
	}

	var reloadedMu sync.Mutex
	reloadedNames := make([]string, 0, len(names))

	for i := 0; i < len(names); i += opts.BatchSize {
		end := i + opts.BatchSize
		if end > len(names) {
			end = len(names)
		}
		batch := names[i:end]

		var wg sync.WaitGroup
		errCh := make(chan error, len(batch))

		for _, name := range batch {
			wg.Add(1)
			go func(n string) {
				defer wg.Done()
				if err := s.ReloadProcess(ctx, n); err != nil {
					errCh <- fmt.Errorf("%s: %w", n, err)
					return
				}
				reloadedMu.Lock()
				reloadedNames = append(reloadedNames, n)
				reloadedMu.Unlock()
				if opts.WaitReady {
					if err := s.WaitUntilReady(ctx, n, timeout); err != nil {
						errCh <- fmt.Errorf("%s: %w", n, err)
					}
				}
			}(name)
		}
		wg.Wait()
		close(errCh)

		var batchErrors []error
		for err := range errCh {
			batchErrors = append(batchErrors, err)
			log.Error().Err(err).Msg("rolling reload batch failure")
		}

		if len(batchErrors) > 0 {
			if opts.RollbackOnFailure {
				// Rollback: restore all processes that were reloaded successfully,
				// including successes in the current batch.
				for j := len(reloadedNames) - 1; j >= 0; j-- {
					name := reloadedNames[j]
					proc, err := s.Get(name)
					if err != nil {
						continue
					}
					proc.Config = originalConfigs[name]
					_ = s.ReloadProcess(ctx, name)
					if opts.WaitReady {
						_ = s.WaitUntilReady(ctx, name, timeout)
					}
				}
				return fmt.Errorf("rolling reload failed at batch starting with %q, rolled back %d processes: %v",
					batch[0], len(reloadedNames), batchErrors[0])
			}
			return fmt.Errorf("rolling reload failed at batch starting with %q: %v", batch[0], batchErrors[0])
		}

		log.Info().
			Strs("batch", batch).
			Int("progress", end).
			Int("total", len(names)).
			Msg("rolling reload batch completed")
	}

	return nil
}
