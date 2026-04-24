package config

import (
	"testing"

	"github.com/runixio/runix/pkg/types"
)

func TestDiffConfigs_NoChanges(t *testing.T) {
	old := &types.RunixConfig{
		Processes: []types.ProcessConfig{
			{Name: "api", Entrypoint: "./api"},
		},
	}
	diff := DiffConfigs(old, old)
	if len(diff.Added) != 0 || len(diff.Removed) != 0 || len(diff.Modified) != 0 {
		t.Fatal("expected no changes")
	}
}

func TestDiffConfigs_Added(t *testing.T) {
	old := &types.RunixConfig{}
	new := &types.RunixConfig{
		Processes: []types.ProcessConfig{
			{Name: "api", Entrypoint: "./api"},
		},
	}
	diff := DiffConfigs(old, new)
	if len(diff.Added) != 1 || diff.Added[0].Name != "api" {
		t.Fatalf("expected 1 added, got %v", diff.Added)
	}
}

func TestDiffConfigs_Removed(t *testing.T) {
	old := &types.RunixConfig{
		Processes: []types.ProcessConfig{
			{Name: "api", Entrypoint: "./api"},
		},
	}
	new := &types.RunixConfig{}
	diff := DiffConfigs(old, new)
	if len(diff.Removed) != 1 || diff.Removed[0] != "api" {
		t.Fatalf("expected 1 removed, got %v", diff.Removed)
	}
}

func TestDiffConfigs_Modified(t *testing.T) {
	old := &types.RunixConfig{
		Processes: []types.ProcessConfig{
			{Name: "api", Entrypoint: "./api", Runtime: "go"},
		},
	}
	new := &types.RunixConfig{
		Processes: []types.ProcessConfig{
			{Name: "api", Entrypoint: "./api-new", Runtime: "go"},
		},
	}
	diff := DiffConfigs(old, new)
	if len(diff.Modified) != 1 {
		t.Fatalf("expected 1 modified, got %d", len(diff.Modified))
	}
	if diff.Modified[0].Name != "api" {
		t.Fatalf("expected api, got %s", diff.Modified[0].Name)
	}
	found := false
	for _, c := range diff.Modified[0].Changed {
		if c == "entrypoint" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected entrypoint in changed fields")
	}
}

func TestDiffConfigs_String(t *testing.T) {
	old := &types.RunixConfig{
		Processes: []types.ProcessConfig{
			{Name: "old", Entrypoint: "./old"},
		},
	}
	new := &types.RunixConfig{
		Processes: []types.ProcessConfig{
			{Name: "new", Entrypoint: "./new"},
		},
	}
	diff := DiffConfigs(old, new)
	s := diff.String()
	if s == "no changes" {
		t.Fatal("expected changes in string")
	}
}
