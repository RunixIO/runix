package supervisor

import (
	"math"
	"math/rand/v2"
	"time"
)

// Backoff computes exponential backoff durations with optional jitter.
type Backoff struct {
	Base   time.Duration // base delay (default 1s)
	Max    time.Duration // maximum delay (default 60s)
	Jitter time.Duration // random jitter added (default 500ms)
}

// NewBackoff returns a Backoff initialised with sensible defaults.
func NewBackoff() Backoff {
	return Backoff{
		Base:   time.Second,
		Max:    60 * time.Second,
		Jitter: 500 * time.Millisecond,
	}
}

// Next returns the backoff duration for the given attempt number (zero-based).
// The formula is: min(base * 2^attempt + rand(0, jitter), max).
func (b Backoff) Next(attempt int) time.Duration {
	if b.Base <= 0 {
		b.Base = time.Second
	}
	if b.Max <= 0 {
		b.Max = 60 * time.Second
	}

	exp := time.Duration(float64(b.Base) * math.Pow(2, float64(attempt)))
	delay := exp

	if b.Jitter > 0 {
		delay += time.Duration(rand.Int64N(int64(b.Jitter)))
	}

	if delay > b.Max {
		delay = b.Max
	}
	if delay < 0 {
		delay = b.Max
	}
	return delay
}
