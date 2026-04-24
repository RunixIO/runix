package events

import (
	"time"

	"github.com/google/uuid"
)

type EventType string

const (
	ProcessStarted   EventType = "process.started"
	ProcessStopped   EventType = "process.stopped"
	ProcessCrashed   EventType = "process.crashed"
	ProcessHealthy   EventType = "process.healthy"
	ProcessUnhealthy EventType = "process.unhealthy"
	ProcessReloaded  EventType = "process.reloaded"
)

type Event struct {
	ID          string                 `json:"id"`
	Type        EventType              `json:"type"`
	Timestamp   time.Time              `json:"timestamp"`
	ProcessID   string                 `json:"process_id"`
	ProcessName string                 `json:"process_name"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
}

func newEvent(eventType EventType, processID, processName string, payload map[string]interface{}) Event {
	return Event{
		ID:          uuid.New().String(),
		Type:        eventType,
		Timestamp:   time.Now().UTC(),
		ProcessID:   processID,
		ProcessName: processName,
		Payload:     payload,
	}
}
