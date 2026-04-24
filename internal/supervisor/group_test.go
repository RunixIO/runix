package supervisor

import (
	"context"
	"testing"

	"github.com/runixio/runix/pkg/types"
)

func TestGetGroupResolvesServiceInstances(t *testing.T) {
	sup := New(Options{})
	defer func() { _ = sup.Shutdown() }()

	for _, name := range []string{"api:0", "api:1"} {
		_, err := sup.AddProcess(context.Background(), types.ProcessConfig{
			Name:          name,
			Entrypoint:    "sleep",
			Args:          []string{"30"},
			Runtime:       "unknown",
			RestartPolicy: types.RestartNever,
		})
		if err != nil {
			t.Fatalf("AddProcess(%q) failed: %v", name, err)
		}
	}

	procs, err := sup.GetGroup("api")
	if err != nil {
		t.Fatalf("GetGroup returned error: %v", err)
	}
	if len(procs) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(procs))
	}
	if procs[0].Config.Name != "api:0" || procs[1].Config.Name != "api:1" {
		t.Fatalf("unexpected group order: %q then %q", procs[0].Config.Name, procs[1].Config.Name)
	}
}

func TestGetGroupPrefersExactProcessName(t *testing.T) {
	sup := New(Options{})
	defer func() { _ = sup.Shutdown() }()

	for _, name := range []string{"api", "api:0", "api:1"} {
		_, err := sup.AddProcess(context.Background(), types.ProcessConfig{
			Name:          name,
			Entrypoint:    "sleep",
			Args:          []string{"30"},
			Runtime:       "unknown",
			RestartPolicy: types.RestartNever,
		})
		if err != nil {
			t.Fatalf("AddProcess(%q) failed: %v", name, err)
		}
	}

	procs, err := sup.GetGroup("api")
	if err != nil {
		t.Fatalf("GetGroup returned error: %v", err)
	}
	if len(procs) != 1 || procs[0].Config.Name != "api" {
		t.Fatalf("expected exact process match, got %#v", procs)
	}
}
