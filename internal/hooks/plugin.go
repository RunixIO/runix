package hooks

import (
	"context"
	"sync"

	"github.com/runixio/runix/pkg/types"
)

// Plugin is an extension point for external integrations.
// Plugins are notified of lifecycle events but cannot block them.
type Plugin interface {
	Name() string
	Handle(ctx context.Context, event string, cfg types.ProcessConfig) error
}

// pluginRegistry holds globally registered plugins.
var pluginRegistry struct {
	mu      sync.RWMutex
	plugins []Plugin
}

// RegisterPlugin adds a plugin to the global registry.
func RegisterPlugin(p Plugin) {
	pluginRegistry.mu.Lock()
	defer pluginRegistry.mu.Unlock()
	pluginRegistry.plugins = append(pluginRegistry.plugins, p)
}

// NotifyPlugins sends an event to all registered plugins.
// Errors from plugins are logged but do not block the caller.
func NotifyPlugins(ctx context.Context, event string, cfg types.ProcessConfig) {
	pluginRegistry.mu.RLock()
	plugins := make([]Plugin, len(pluginRegistry.plugins))
	copy(plugins, pluginRegistry.plugins)
	pluginRegistry.mu.RUnlock()

	for _, p := range plugins {
		if err := p.Handle(ctx, event, cfg); err != nil {
			// Plugin errors are non-blocking; logged by the plugin itself.
			_ = err
		}
	}
}

// RegisteredPlugins returns the names of all registered plugins.
func RegisteredPlugins() []string {
	pluginRegistry.mu.RLock()
	defer pluginRegistry.mu.RUnlock()
	names := make([]string, len(pluginRegistry.plugins))
	for i, p := range pluginRegistry.plugins {
		names[i] = p.Name()
	}
	return names
}
