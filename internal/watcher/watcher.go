package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

// EventHandler is called when a file change is detected after debouncing.
type EventHandler func(paths []string)

// Watcher wraps fsnotify with debounce and ignore pattern support.
type Watcher struct {
	fsw      *fsnotify.Watcher
	paths    []string
	ignore   []string
	debounce time.Duration

	mu       sync.Mutex
	events   chan string
	done     chan struct{}
	handler  EventHandler
	running  bool
	stopOnce sync.Once
}

// New creates a new Watcher with the given options.
func New(paths []string, ignore []string, debounce time.Duration) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	return &Watcher{
		fsw:      fsw,
		paths:    paths,
		ignore:   ignore,
		debounce: debounce,
		events:   make(chan string, 256),
		done:     make(chan struct{}),
	}, nil
}

// Start begins watching and calls handler when changes are detected.
func (w *Watcher) Start(handler EventHandler) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("watcher already running")
	}
	w.handler = handler
	w.running = true
	w.mu.Unlock()

	// Add paths to the fsnotify watcher.
	for _, p := range w.paths {
		if err := w.addRecursively(p); err != nil {
			log.Warn().Str("path", p).Err(err).Msg("failed to watch path")
		}
	}

	go w.coalesceLoop()
	go w.eventLoop()

	log.Info().Strs("paths", w.paths).Msg("file watcher started")
	return nil
}

// Stop stops the watcher. Safe to call multiple times.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.running = false
	w.stopOnce.Do(func() { close(w.done) })
	_ = w.fsw.Close()

	log.Info().Msg("file watcher stopped")
}

// addRecursively adds a directory and all subdirectories to the watcher.
func (w *Watcher) addRecursively(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if !info.IsDir() {
			return nil
		}
		if w.isIgnored(path) {
			return filepath.SkipDir
		}
		return w.fsw.Add(path)
	})
}

// eventLoop reads raw fsnotify events and forwards them to the coalescer.
func (w *Watcher) eventLoop() {
	for {
		select {
		case <-w.done:
			return
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if w.isIgnored(event.Name) {
				continue
			}
			// Only forward Write, Create, Remove, Rename events.
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			select {
			case w.events <- event.Name:
			default:
				// Drop event if channel is full.
			}
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			log.Error().Err(err).Msg("watcher error")
		}
	}
}

// coalesceLoop collects events during the debounce window and calls the handler
// once with the deduplicated set of changed paths.
func (w *Watcher) coalesceLoop() {
	var pending map[string]bool
	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return

		case path := <-w.events:
			if pending == nil {
				pending = make(map[string]bool)
				timer = time.NewTimer(w.debounce)
				timerC = timer.C
			}
			pending[path] = true

		case <-timerC:
			if len(pending) > 0 {
				paths := make([]string, 0, len(pending))
				for p := range pending {
					paths = append(paths, p)
				}
				w.mu.Lock()
				handler := w.handler
				w.mu.Unlock()
				if handler != nil {
					handler(paths)
				}
			}
			pending = nil
			timer = nil
			timerC = nil
		}
	}
}

// isIgnored checks if a path matches any ignore pattern.
func (w *Watcher) isIgnored(path string) bool {
	for _, pattern := range w.ignore {
		// Support simple glob patterns and directory prefixes.
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		// Check if the path contains a segment matching the pattern.
		for _, segment := range strings.Split(path, string(filepath.Separator)) {
			if matched, _ := filepath.Match(pattern, segment); matched {
				return true
			}
		}
	}
	return false
}
