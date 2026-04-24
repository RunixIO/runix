package main

import (
	"reflect"
	"testing"

	"github.com/runixio/runix/pkg/types"
)

func TestRunDeployExecutesExpectedCommands(t *testing.T) {
	cfg := &types.RunixConfig{
		Deploy: map[string]types.DeployTarget{
			"prod": {
				Host:          "example.com",
				User:          "deploy",
				Port:          2222,
				Path:          "/srv/runix",
				PreDeploy:     "echo pre",
				PostDeploy:    "echo post",
				ReloadCommand: "runix --config runix.yaml daemon reload",
			},
		},
	}

	var calls [][]string
	runner := func(name string, args ...string) error {
		call := append([]string{name}, args...)
		calls = append(calls, call)
		return nil
	}

	err := runDeploy("prod", "/tmp/prod.runix.toml", cfg, runner)
	if err != nil {
		t.Fatalf("runDeploy returned error: %v", err)
	}

	want := [][]string{
		{"sh", "-c", "echo pre"},
		{"ssh", "-p", "2222", "deploy@example.com", "mkdir -p '/srv/runix'"},
		{"scp", "-P", "2222", "/tmp/prod.runix.toml", "deploy@example.com:/srv/runix/prod.runix.toml"},
		{"ssh", "-p", "2222", "deploy@example.com", "cd '/srv/runix' && echo post"},
		{"ssh", "-p", "2222", "deploy@example.com", "cd '/srv/runix' && runix --config runix.yaml daemon reload"},
	}

	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected calls:\n got: %#v\nwant: %#v", calls, want)
	}
}

func TestRunDeployMissingTarget(t *testing.T) {
	err := runDeploy("prod", "/tmp/runix.yaml", &types.RunixConfig{}, func(name string, args ...string) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected missing target error")
	}
}
