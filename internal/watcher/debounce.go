package watcher

import (
	"sync"
	"time"
)

// Debouncer coalesces events within a time window, calling the handler
// once per window with all accumulated events.
type Debouncer struct {
	window  time.Duration
	handler func(events []string)
	mu      sync.Mutex
	pending map[string]bool
	timer   *time.Timer
}

// NewDebouncer creates a debouncer with the given window duration.
func NewDebouncer(window time.Duration, handler func(events []string)) *Debouncer {
	return &Debouncer{
		window:  window,
		handler: handler,
		pending: make(map[string]bool),
	}
}

// Add registers an event. The first event starts the debounce timer.
// Subsequent events reset or accumulate within the window.
func (d *Debouncer) Add(event string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.pending[event] = true

	// Reset or start the timer.
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.window, d.flush)
}

// flush sends all accumulated events to the handler.
func (d *Debouncer) flush() {
	d.mu.Lock()
	events := make([]string, 0, len(d.pending))
	for e := range d.pending {
		events = append(events, e)
	}
	d.pending = make(map[string]bool)
	d.timer = nil
	d.mu.Unlock()

	if len(events) > 0 && d.handler != nil {
		d.handler(events)
	}
}

// Stop cancels any pending flush.
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}
