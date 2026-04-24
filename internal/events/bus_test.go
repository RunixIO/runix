package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewBus(t *testing.T) {
	dir := t.TempDir()
	bus := NewBus(dir)
	if bus == nil {
		t.Fatal("expected non-nil bus")
	}
	if bus.store == nil {
		t.Fatal("expected non-nil store")
	}
	if bus.subscribers == nil {
		t.Fatal("expected non-nil subscribers map")
	}
}

func TestNewBusEmptyDir(t *testing.T) {
	bus := NewBus("")
	if bus == nil {
		t.Fatal("expected non-nil bus")
	}
	if bus.store != nil {
		t.Fatal("expected nil store for empty dir")
	}
}

func TestEmitAndSubscribe(t *testing.T) {
	bus := NewBus("")
	ch := bus.Subscribe(ProcessStarted)

	bus.Emit(ProcessStarted, "p1", "web", map[string]interface{}{"port": 8080})

	select {
	case evt := <-ch:
		if evt.Type != ProcessStarted {
			t.Fatalf("expected %s, got %s", ProcessStarted, evt.Type)
		}
		if evt.ProcessID != "p1" {
			t.Fatalf("expected process_id p1, got %s", evt.ProcessID)
		}
		if evt.ProcessName != "web" {
			t.Fatalf("expected process_name web, got %s", evt.ProcessName)
		}
		if evt.ID == "" {
			t.Fatal("expected non-empty event ID")
		}
		if evt.Timestamp.IsZero() {
			t.Fatal("expected non-zero timestamp")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEmitWildcardSubscribe(t *testing.T) {
	bus := NewBus("")
	ch := bus.Subscribe() // subscribe to all

	bus.Emit(ProcessStarted, "p1", "web", nil)
	bus.Emit(ProcessCrashed, "p2", "api", nil)

	count := 0
	for count < 2 {
		select {
		case evt := <-ch:
			count++
			if evt.ProcessID == "" {
				t.Fatal("expected non-empty process_id")
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out, received %d events", count)
		}
	}
}

func TestMultipleEventTypes(t *testing.T) {
	bus := NewBus("")
	startedCh := bus.Subscribe(ProcessStarted)
	crashedCh := bus.Subscribe(ProcessCrashed)

	bus.Emit(ProcessStarted, "p1", "web", nil)
	bus.Emit(ProcessCrashed, "p2", "api", nil)
	bus.Emit(ProcessStopped, "p3", "worker", nil)

	// startedCh should get only ProcessStarted
	select {
	case evt := <-startedCh:
		if evt.Type != ProcessStarted {
			t.Fatalf("expected %s, got %s", ProcessStarted, evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for started event")
	}

	// crashedCh should get only ProcessCrashed
	select {
	case evt := <-crashedCh:
		if evt.Type != ProcessCrashed {
			t.Fatalf("expected %s, got %s", ProcessCrashed, evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for crashed event")
	}

	// Neither channel should have the ProcessStopped event.
	select {
	case evt := <-startedCh:
		t.Fatalf("unexpected event on started channel: %s", evt.Type)
	case evt := <-crashedCh:
		t.Fatalf("unexpected event on crashed channel: %s", evt.Type)
	default:
		// Expected: no more events.
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := NewBus("")
	ch := bus.Subscribe(ProcessStarted)

	bus.Unsubscribe(ch)

	bus.Emit(ProcessStarted, "p1", "web", nil)

	select {
	case <-ch:
		t.Fatal("expected no event after unsubscribe")
	case <-time.After(50 * time.Millisecond):
		// Expected: no event received.
	}
}

func TestUnsubscribeRemovesFromMultipleTypes(t *testing.T) {
	bus := NewBus("")
	ch := bus.Subscribe(ProcessStarted, ProcessStopped)

	bus.Unsubscribe(ch)

	bus.Emit(ProcessStarted, "p1", "web", nil)
	bus.Emit(ProcessStopped, "p2", "api", nil)

	select {
	case <-ch:
		t.Fatal("expected no event after unsubscribe")
	case <-time.After(50 * time.Millisecond):
		// Expected: no event received.
	}
}

func TestStoreAppendAndQuery(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if store == nil {
		t.Fatal("expected non-nil store")
	}

	evt := newEvent(ProcessStarted, "p1", "web", map[string]interface{}{"port": 8080})
	store.Append(evt)
	evt2 := newEvent(ProcessCrashed, "p2", "api", map[string]interface{}{"exit_code": 1})
	store.Append(evt2)

	// Query all events since the zero time.
	results := store.Query(time.Time{})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Query with type filter.
	results = store.Query(time.Time{}, ProcessStarted)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Type != ProcessStarted {
		t.Fatalf("expected %s, got %s", ProcessStarted, results[0].Type)
	}

	// Query with time filter (future time should return nothing).
	results = store.Query(time.Now().Add(time.Hour))
	if len(results) != 0 {
		t.Fatalf("expected 0 results for future time, got %d", len(results))
	}
}

func TestStoreFileFormat(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	evt := newEvent(ProcessStarted, "p1", "web", nil)
	store.Append(evt)

	data, err := os.ReadFile(filepath.Join(dir, "events.log"))
	if err != nil {
		t.Fatalf("failed to read events log: %v", err)
	}

	var parsed Event
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if parsed.ID != evt.ID {
		t.Fatalf("expected ID %s, got %s", evt.ID, parsed.ID)
	}
}

func TestStoreNilReceiver(t *testing.T) {
	var store *Store
	store.Append(Event{})
	results := store.Query(time.Time{})
	if results != nil {
		t.Fatal("expected nil results from nil store")
	}
}

func TestHistoryWithStore(t *testing.T) {
	dir := t.TempDir()
	bus := NewBus(dir)

	bus.Emit(ProcessStarted, "p1", "web", nil)
	bus.Emit(ProcessCrashed, "p2", "api", nil)
	bus.Emit(ProcessStopped, "p3", "worker", nil)

	// Query all events.
	history := bus.History(time.Time{})
	if len(history) != 3 {
		t.Fatalf("expected 3 history events, got %d", len(history))
	}

	// Query with type filter.
	history = bus.History(time.Time{}, ProcessCrashed)
	if len(history) != 1 {
		t.Fatalf("expected 1 history event, got %d", len(history))
	}
	if history[0].Type != ProcessCrashed {
		t.Fatalf("expected %s, got %s", ProcessCrashed, history[0].Type)
	}
}

func TestHistoryWithoutStore(t *testing.T) {
	bus := NewBus("")
	history := bus.History(time.Time{})
	if history != nil {
		t.Fatal("expected nil history without store")
	}
}

func TestConcurrentEmit(t *testing.T) {
	bus := NewBus("")
	ch := bus.Subscribe()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Emit(ProcessStarted, "p1", "web", nil)
		}()
	}
	wg.Wait()

	count := 0
	timeout := time.After(2 * time.Second)
	for count < 100 {
		select {
		case <-ch:
			count++
		case <-timeout:
			t.Fatalf("timed out, received %d/100 events", count)
		}
	}
}
