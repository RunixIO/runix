package supervisor

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/runixio/runix/pkg/types"
)

// BenchmarkBuildEnv measures the performance of environment overlay construction.
func BenchmarkBuildEnv(b *testing.B) {
	overlay := make(map[string]string, 20)
	for i := 0; i < 20; i++ {
		overlay["RUNIX_VAR_"+strconv.Itoa(i)] = "value_" + strconv.Itoa(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildEnv(overlay)
	}
}

// BenchmarkBuildEnvEmpty measures with no overlay (common case).
func BenchmarkBuildEnvEmpty(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildEnv(nil)
	}
}

// BenchmarkList measures concurrent-safe process listing.
func BenchmarkList(b *testing.B) {
	sup := setupBenchSupervisor(b, 50)
	defer sup.Shutdown()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sup.List()
	}
}

// BenchmarkGet measures process lookup by ID.
func BenchmarkGet(b *testing.B) {
	sup := setupBenchSupervisor(b, 50)
	defer sup.Shutdown()

	procs := sup.List()
	if len(procs) == 0 {
		b.Fatal("no processes")
	}
	id := procs[0].ID

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sup.Get(id)
	}
}

// BenchmarkGetByName measures process lookup by name.
func BenchmarkGetByName(b *testing.B) {
	sup := setupBenchSupervisor(b, 50)
	defer sup.Shutdown()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sup.Get("bench-proc-25")
	}
}

// BenchmarkProcessInfo measures the Info() snapshot method.
func BenchmarkProcessInfo(b *testing.B) {
	sup := setupBenchSupervisor(b, 1)
	defer sup.Shutdown()

	procs := sup.List()
	if len(procs) == 0 {
		b.Fatal("no processes")
	}

	proc, err := sup.Get(procs[0].ID)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proc.Info()
	}
}

// BenchmarkBuildArgs measures command argument construction.
func BenchmarkBuildArgs(b *testing.B) {
	cfg := types.ProcessConfig{
		Entrypoint: "python3",
		Args:       []string{"-u", "server.py", "--port", "8080"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildArgs(cfg)
	}
}

// BenchmarkStateTransition measures the CAS-based state transition.
func BenchmarkStateTransition(b *testing.B) {
	proc := NewManagedProcess(types.ProcessConfig{
		Name:       "bench",
		Entrypoint: "sleep",
		Runtime:    "unknown",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proc.SetStateDirect(types.StateStopped)
		_ = proc.GetState()
	}
}

// BenchmarkShouldRestart measures restart decision logic.
func BenchmarkShouldRestart(b *testing.B) {
	proc := NewManagedProcess(types.ProcessConfig{
		Name:          "bench",
		Entrypoint:    "sleep",
		Runtime:       "unknown",
		RestartPolicy: types.RestartAlways,
	})
	proc.ApplyDefaults(types.DefaultsConfig{
		RestartPolicy: types.RestartAlways,
		MaxRestarts:   10,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proc.ShouldRestart(1)
	}
}

// ── Helpers ──

func setupBenchSupervisor(b *testing.B, n int) *Supervisor {
	dir := b.TempDir()
	logDir := dir + "/logs"
	os.MkdirAll(logDir, 0o755)

	sup := New(Options{
		LogDir: logDir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
			MaxRestarts:   0,
		},
	})

	for i := 0; i < n; i++ {
		name := "bench-proc-" + strconv.Itoa(i)
		_, err := sup.AddProcess(context.Background(), types.ProcessConfig{
			Name:          name,
			Entrypoint:    "sleep",
			Args:          []string{"300"},
			Runtime:       "unknown",
			RestartPolicy: types.RestartNever,
		})
		if err != nil {
			// May fail if sleep is not available; skip.
			if strings.Contains(err.Error(), "executable file not found") {
				b.Skip("sleep not available")
			}
			b.Fatal(err)
		}
	}

	return sup
}
