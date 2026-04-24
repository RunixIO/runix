package daemon

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/runixio/runix/internal/supervisor"
	"github.com/runixio/runix/pkg/types"
)

func TestStartConfigProcessesExpandsInstances(t *testing.T) {
	sup := supervisor.New(supervisor.Options{
		LogDir: filepath.Join(t.TempDir(), "logs"),
	})
	defer func() { _ = sup.Shutdown() }()

	startConfigProcesses(sup, &types.RunixConfig{
		Processes: []types.ProcessConfig{
			{Name: "api", Entrypoint: "sleep", Args: []string{"60"}, Autostart: true, Instances: 3, RestartPolicy: types.RestartNever},
		},
	})

	procs := sup.List()
	if len(procs) != 3 {
		t.Fatalf("expected 3 started processes, got %d", len(procs))
	}
	if procs[0].Config.Name != "api:0" || procs[1].Config.Name != "api:1" || procs[2].Config.Name != "api:2" {
		t.Fatalf("unexpected started names: %q, %q, %q", procs[0].Config.Name, procs[1].Config.Name, procs[2].Config.Name)
	}

	for _, proc := range procs {
		if err := sup.StopProcess(proc.ID, false, 5*time.Second); err != nil {
			t.Fatalf("stop %q: %v", proc.Config.Name, err)
		}
	}
}
