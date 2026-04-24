package hooks

import (
	"context"
	"fmt"
	"sync"

	"github.com/runixio/runix/pkg/types"
)

// HookChain executes a sequence of hook commands in order.
// If any hook fails (and is not configured to ignore failure),
// execution stops and the error is returned.
type HookChain struct {
	mu    sync.Mutex
	hooks []*types.HookConfig
}

// NewChain creates a hook chain from the given hook configs.
// Nil entries are skipped.
func NewChain(hooks ...*types.HookConfig) *HookChain {
	var active []*types.HookConfig
	for _, h := range hooks {
		if h != nil && h.Command != "" {
			active = append(active, h)
		}
	}
	return &HookChain{hooks: active}
}

// Execute runs all hooks in the chain sequentially.
// The executor's Run method is used for each hook.
// On failure (unless IgnoreFailure is set), execution stops.
func (c *HookChain) Execute(ctx context.Context, executor *Executor, event string, cfg types.ProcessConfig) error {
	c.mu.Lock()
	hooks := make([]*types.HookConfig, len(c.hooks))
	copy(hooks, c.hooks)
	c.mu.Unlock()

	for i, hook := range hooks {
		if err := executor.Run(ctx, hook, fmt.Sprintf("%s[%d]", event, i), cfg); err != nil {
			return err
		}
	}
	return nil
}

// Len returns the number of active hooks in the chain.
func (c *HookChain) Len() int {
	return len(c.hooks)
}
