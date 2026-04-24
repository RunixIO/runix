package events

import (
	"sync"
	"time"
)

type Bus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]chan Event
	store       *Store
}

func NewBus(storeDir string) *Bus {
	return &Bus{
		subscribers: make(map[EventType][]chan Event),
		store:       NewStore(storeDir),
	}
}

func (b *Bus) Emit(eventType EventType, processID, processName string, payload map[string]interface{}) {
	evt := newEvent(eventType, processID, processName, payload)

	// Persist to store.
	if b.store != nil {
		b.store.Append(evt)
	}

	// Notify subscribers.
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Notify subscribers of this specific event type.
	if chans, ok := b.subscribers[eventType]; ok {
		for _, ch := range chans {
			select {
			case ch <- evt:
			default:
				// Drop if channel full.
			}
		}
	}

	// Notify wildcard subscribers (empty string type).
	if chans, ok := b.subscribers[""]; ok {
		for _, ch := range chans {
			select {
			case ch <- evt:
			default:
			}
		}
	}
}

func (b *Bus) Subscribe(eventTypes ...EventType) <-chan Event {
	ch := make(chan Event, 256)
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(eventTypes) == 0 {
		// Subscribe to all events.
		b.subscribers[""] = append(b.subscribers[""], ch)
	} else {
		for _, et := range eventTypes {
			b.subscribers[et] = append(b.subscribers[et], ch)
		}
	}
	return ch
}

func (b *Bus) Unsubscribe(ch <-chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for et, chans := range b.subscribers {
		for i, c := range chans {
			if c == ch {
				b.subscribers[et] = append(chans[:i], chans[i+1:]...)
				break
			}
		}
	}
}

func (b *Bus) History(since time.Time, eventTypes ...EventType) []Event {
	if b.store == nil {
		return nil
	}
	return b.store.Query(since, eventTypes...)
}

// Close closes the underlying store, releasing the file handle.
func (b *Bus) Close() error {
	if b.store == nil {
		return nil
	}
	return b.store.Close()
}
