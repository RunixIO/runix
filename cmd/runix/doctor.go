package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/internal/version"
	"github.com/spf13/cobra"
)

type doctorCheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type doctorResult struct {
	Version string              `json:"version"`
	OS      string              `json:"os"`
	Arch    string              `json:"arch"`
	Checks  []doctorCheckResult `json:"checks"`
	Passed  int                 `json:"passed"`
	Warned  int                 `json:"warned"`
	Failed  int                 `json:"failed"`
}

func newDoctorCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run diagnostic checks",
		Long:  `Run a series of diagnostic checks to verify your Runix environment is healthy.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			passed := 0
			failed := 0
			warned := 0
			var results []doctorCheckResult

			printHeader := func(title string) {
				fmt.Fprintf(os.Stdout, "\n── %s ──\n", title)
			}

			check := func(name string, fn func() (status, string)) {
				s, detail := fn()
				var statusStr string
				switch s {
				case statusOK:
					fmt.Fprintf(os.Stdout, "  [OK]   %-20s %s\n", name, detail)
					passed++
					statusStr = "ok"
				case statusWarn:
					fmt.Fprintf(os.Stdout, "  [WARN] %-20s %s\n", name, detail)
					warned++
					statusStr = "warn"
				case statusFail:
					fmt.Fprintf(os.Stdout, "  [FAIL] %-20s %s\n", name, detail)
					failed++
					statusStr = "fail"
				}
				results = append(results, doctorCheckResult{
					Name:   name,
					Status: statusStr,
					Detail: detail,
				})
			}

			fmt.Fprintln(os.Stdout, "Runix Doctor — Environment Diagnostics")
			fmt.Fprintf(os.Stdout, "Version: %s  OS: %s/%s\n", version.Version, runtime.GOOS, runtime.GOARCH)

			// Runtime checks.
			printHeader("Runtimes")
			for _, rt := range []struct {
				name string
				cmd  string
			}{
				{"Go", "go"},
				{"Python", "python3"},
				{"Node.js", "node"},
				{"Bun", "bun"},
			} {
				check(rt.name, makeLookPathCheck(rt.name, rt.cmd))
			}

			// Directory checks.
			printHeader("Directories")
			dd := dataDir()
			check("Data dir", makeDirCheck(dd))
			check("Apps dir", makeDirCheck(filepath.Join(dd, "apps")))
			check("State dir", makeDirCheck(filepath.Join(dd, "state")))
			check("Tmp dir", makeDirCheck(filepath.Join(dd, "tmp")))

			// Daemon checks.
			printHeader("Daemon")
			check("Socket", makeSocketCheck(daemon.DefaultSocketPath()))
			check("Daemon", func() (status, string) {
				client := daemonClient()
				if client.IsAlive() {
					return statusOK, "running"
				}
				return statusWarn, "not running"
			})

			// Permissions.
			printHeader("Permissions")
			check("Write executable", makeWriteCheck(dataDir()))

			// Summary.
			fmt.Fprintln(os.Stdout)
			total := passed + warned + failed
			fmt.Fprintf(os.Stdout, "Results: %d/%d passed", passed, total)
			if warned > 0 {
				fmt.Fprintf(os.Stdout, ", %d warnings", warned)
			}
			if failed > 0 {
				fmt.Fprintf(os.Stdout, ", %d failures", failed)
			}
			fmt.Fprintln(os.Stdout)

			if format == "json" {
				outputResult("json", doctorResult{
					Version: version.Version,
					OS:      runtime.GOOS,
					Arch:    runtime.GOARCH,
					Checks:  results,
					Passed:  passed,
					Warned:  warned,
					Failed:  failed,
				}, func() {})
			}

			if failed > 0 {
				return fmt.Errorf("%d check(s) failed", failed)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format: text, json")

	return cmd
}

type status int

const (
	statusOK status = iota
	statusWarn
	statusFail
)

func makeLookPathCheck(name, cmd string) func() (status, string) {
	return func() (status, string) {
		path, err := exec.LookPath(cmd)
		if err != nil {
			return statusWarn, "not found"
		}
		return statusOK, path
	}
}

func makeDirCheck(dir string) func() (status, string) {
	return func() (status, string) {
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				// Try to create it.
				if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
					return statusFail, fmt.Sprintf("%s (create failed: %v)", dir, mkErr)
				}
				return statusOK, fmt.Sprintf("%s (created)", dir)
			}
			return statusFail, fmt.Sprintf("%s (%v)", dir, err)
		}
		if !info.IsDir() {
			return statusFail, fmt.Sprintf("%s (not a directory)", dir)
		}
		return statusOK, dir
	}
}

func makeSocketCheck(socketPath string) func() (status, string) {
	return func() (status, string) {
		dir := filepath.Dir(socketPath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return statusWarn, fmt.Sprintf("%s (dir does not exist)", socketPath)
		}
		if _, err := os.Stat(socketPath); os.IsNotExist(err) {
			return statusWarn, fmt.Sprintf("%s (not created)", socketPath)
		}
		return statusOK, socketPath
	}
}

func makeWriteCheck(dir string) func() (status, string) {
	return func() (status, string) {
		testFile := filepath.Join(dir, ".runix-doctor-write-test")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return statusFail, fmt.Sprintf("cannot create %s: %v", dir, err)
		}
		if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
			return statusFail, fmt.Sprintf("cannot write to %s: %v", dir, err)
		}
		os.Remove(testFile)
		return statusOK, "writable"
	}
}

// Unused import guard.
var _ = strings.TrimSpace
