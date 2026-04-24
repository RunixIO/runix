package healthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/runixio/runix/pkg/types"
)

func TestNewChecker(t *testing.T) {
	cfg := types.HealthCheckConfig{
		Type:     types.HealthCheckHTTP,
		URL:      "http://localhost:9999/health",
		Interval: "1s",
		Timeout:  "500ms",
		Retries:  3,
	}
	c := NewChecker(cfg, nil)
	if c == nil {
		t.Fatal("expected non-nil checker")
	}
}

func TestHTTPCheckSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := types.HealthCheckConfig{
		Type:    types.HealthCheckHTTP,
		URL:     server.URL,
		Timeout: "5s",
	}
	c := NewChecker(cfg, nil)

	if err := c.Check(context.Background()); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestHTTPCheckFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := types.HealthCheckConfig{
		Type:    types.HealthCheckHTTP,
		URL:     server.URL,
		Timeout: "5s",
	}
	c := NewChecker(cfg, nil)

	if err := c.Check(context.Background()); err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestTCPCheckSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	cfg := types.HealthCheckConfig{
		Type:        types.HealthCheckTCP,
		TCPEndpoint: server.Listener.Addr().String(),
		Timeout:     "5s",
	}
	c := NewChecker(cfg, nil)

	if err := c.Check(context.Background()); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestTCPCheckFailure(t *testing.T) {
	cfg := types.HealthCheckConfig{
		Type:        types.HealthCheckTCP,
		TCPEndpoint: "127.0.0.1:1",
		Timeout:     "100ms",
	}
	c := NewChecker(cfg, nil)

	if err := c.Check(context.Background()); err == nil {
		t.Fatal("expected error for closed port")
	}
}

func TestCommandCheckSuccess(t *testing.T) {
	cfg := types.HealthCheckConfig{
		Type:    types.HealthCheckCommand,
		Command: "true",
		Timeout: "5s",
	}
	c := NewChecker(cfg, nil)

	if err := c.Check(context.Background()); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCommandCheckFailure(t *testing.T) {
	cfg := types.HealthCheckConfig{
		Type:    types.HealthCheckCommand,
		Command: "false",
		Timeout: "5s",
	}
	c := NewChecker(cfg, nil)

	if err := c.Check(context.Background()); err == nil {
		t.Fatal("expected error for failing command")
	}
}

func TestCheckerUnhealthyCallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	var called atomic.Int32
	cfg := types.HealthCheckConfig{
		Type:        types.HealthCheckHTTP,
		URL:         server.URL,
		Interval:    "100ms",
		Timeout:     "50ms",
		Retries:     2,
		GracePeriod: "0s",
	}
	c := NewChecker(cfg, func() {
		called.Add(1)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c.Start(ctx)

	time.Sleep(500 * time.Millisecond)
	c.Stop()

	if called.Load() == 0 {
		t.Fatal("expected unhealthy callback to be called")
	}
}

func TestCheckerIsHealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := types.HealthCheckConfig{
		Type:     types.HealthCheckHTTP,
		URL:      server.URL,
		Interval: "100ms",
		Timeout:  "50ms",
		Retries:  1,
	}
	c := NewChecker(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c.Start(ctx)

	time.Sleep(300 * time.Millisecond)
	c.Stop()

	if !c.IsHealthy() {
		t.Fatal("expected healthy")
	}
}
