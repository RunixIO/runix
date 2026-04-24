package main

import (
	"testing"

	"github.com/runixio/runix/pkg/types"
)

func TestRuntimeProcessConfigsRequiresAutostart(t *testing.T) {
	_, err := runtimeProcessConfigs(&types.RunixConfig{
		Processes: []types.ProcessConfig{
			{Name: "api", Entrypoint: "/bin/api", Autostart: false},
		},
	})
	if err == nil {
		t.Fatal("expected error when no autostart processes are defined")
	}
}

func TestRuntimeProcessConfigsSortsByPriorityAndDependencies(t *testing.T) {
	cfgs, err := runtimeProcessConfigs(&types.RunixConfig{
		Processes: []types.ProcessConfig{
			{Name: "worker", Entrypoint: "/bin/worker", Autostart: true, Priority: 10, DependsOn: []string{"api"}},
			{Name: "api", Entrypoint: "/bin/api", Autostart: true, Priority: 5},
			{Name: "metrics", Entrypoint: "/bin/metrics", Autostart: false, Priority: 1},
		},
	})
	if err != nil {
		t.Fatalf("runtimeProcessConfigs returned error: %v", err)
	}

	if len(cfgs) != 2 {
		t.Fatalf("expected 2 autostart processes, got %d", len(cfgs))
	}
	if cfgs[0].Name != "api" || cfgs[1].Name != "worker" {
		t.Fatalf("unexpected start order: %q then %q", cfgs[0].Name, cfgs[1].Name)
	}
}

func TestRuntimeProcessConfigsExpandsInstancesAfterSorting(t *testing.T) {
	cfgs, err := runtimeProcessConfigs(&types.RunixConfig{
		Processes: []types.ProcessConfig{
			{Name: "worker", Entrypoint: "/bin/worker", Autostart: true, Priority: 10, DependsOn: []string{"api"}},
			{Name: "api", Entrypoint: "/bin/api", Autostart: true, Priority: 5, Instances: 2},
		},
	})
	if err != nil {
		t.Fatalf("runtimeProcessConfigs returned error: %v", err)
	}

	if len(cfgs) != 3 {
		t.Fatalf("expected 3 process configs, got %d", len(cfgs))
	}
	if cfgs[0].Name != "api:0" || cfgs[1].Name != "api:1" || cfgs[2].Name != "worker" {
		t.Fatalf("unexpected expanded start order: %q, %q, %q", cfgs[0].Name, cfgs[1].Name, cfgs[2].Name)
	}
}
