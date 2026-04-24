package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/runixio/runix/internal/daemon"
	"github.com/spf13/cobra"
)

const maxLogReadBytes int64 = 1 << 20 // 1 MiB

func newLogsCmd() *cobra.Command {
	var (
		nostream bool
		lines    int
		errOnly  bool
		outOnly  bool
	)

	cmd := &cobra.Command{
		Use:     "logs [id|name]",
		Aliases: []string{"log"},
		Short:   "Stream process logs",
		Long: `Stream logs for managed processes. Follows by default (like tail -f).
Shows both stdout and stderr interleaved by timestamp.
Without arguments, streams combined logs from all applications.
With an app name or ID, streams logs for that specific process.

Use --nostream to print a snapshot without following.
Use --err to show only stderr, --out to show only stdout.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			follow := !nostream

			if len(args) == 0 {
				return showAllLogs(follow, lines, errOnly)
			}

			target := args[0]

			// Non-follow mode: use daemon IPC for a single snapshot.
			if !follow && daemonIsRunning() {
				resp, err := sendIPC(daemon.ActionLogs, daemon.LogsPayload{
					Target: target,
					Lines:  lines,
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed, using direct mode: %v\n", err)
				} else if resp.Success {
					var result struct {
						Logs string `json:"logs"`
					}
					if err := json.Unmarshal(resp.Data, &result); err == nil {
						if result.Logs == "" {
							fmt.Fprintln(os.Stdout, "No logs available")
						} else {
							output := result.Logs
							if errOnly {
								output = filterErrorLines(output)
							}
							fmt.Print(output)
						}
						return nil
					}
				}
			}

			// Resolve log paths from disk.
			logPaths := resolveLogPaths(target)
			if len(logPaths) == 0 {
				fmt.Fprintln(os.Stdout, "No logs available")
				return nil
			}

			// Filter to stdout only or stderr only.
			if outOnly {
				logPaths = filterPaths(logPaths, "stdout.log")
			} else if errOnly {
				logPaths = filterPaths(logPaths, "stderr.log")
			}

			if len(logPaths) == 0 {
				fmt.Fprintln(os.Stdout, "No logs available")
				return nil
			}

			if follow {
				return streamLogs(logPaths)
			}
			return printMergedLogs(logPaths, lines)
		},
	}

	cmd.Flags().BoolVar(&nostream, "nostream", false, "print logs without streaming (snapshot mode)")
	cmd.Flags().IntVarP(&lines, "lines", "n", 50, "number of lines to show")
	cmd.Flags().BoolVar(&errOnly, "err", false, "show only stderr output")
	cmd.Flags().BoolVar(&outOnly, "out", false, "show only stdout output")

	return cmd
}

// showAllLogs displays combined logs from all applications.
func showAllLogs(follow bool, numLines int, errOnly bool) error {
	dd := dataDir()
	appsDir := filepath.Join(dd, "apps")

	entries, err := os.ReadDir(appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(os.Stdout, "No logs available")
			return nil
		}
		return fmt.Errorf("failed to read apps directory: %w", err)
	}

	type logEntry struct {
		appName string
		line    string
	}

	var allLines []logEntry
	var logPaths []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		logPath := filepath.Join(appsDir, entry.Name(), "stdout.log")
		if _, err := os.Stat(logPath); err != nil {
			continue
		}
		logPaths = append(logPaths, logPath)

		if !follow {
			lines := readLogLines(logPath, numLines)
			for _, l := range lines {
				allLines = append(allLines, logEntry{appName: entry.Name(), line: l})
			}
		}
	}

	if !follow {
		if len(allLines) == 0 {
			fmt.Fprintln(os.Stdout, "No logs available")
			return nil
		}

		// Sort by timestamp (lines are prefixed with "YYYY-MM-DD HH:MM:SS").
		sort.SliceStable(allLines, func(i, j int) bool {
			return allLines[i].line < allLines[j].line
		})

		// Show last numLines entries.
		start := 0
		if len(allLines) > numLines {
			start = len(allLines) - numLines
		}

		for _, entry := range allLines[start:] {
			if errOnly && !strings.Contains(entry.line, "[err]") {
				continue
			}
			fmt.Fprintf(os.Stdout, "%-15s | %s\n", entry.appName, entry.line)
		}
		return nil
	}

	// Follow mode: stream all log files.
	return streamAllLogs(logPaths)
}

// readLogLines reads the last n lines from a log file.
func readLogLines(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	if _, err := seekTailWindow(f, maxLogReadBytes); err != nil {
		return nil
	}

	scanner := bufio.NewScanner(f)
	buf := make([]string, n)
	i := 0
	count := 0
	for scanner.Scan() {
		buf[i%n] = scanner.Text()
		i++
		count++
	}

	start := 0
	if count > n {
		start = i % n
	}

	var result []string
	for j := 0; j < n && j < count; j++ {
		line := buf[(start+j)%n]
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

type followState struct {
	path   string
	label  string
	file   *os.File
	reader *bufio.Reader
	offset int64
}

func newFollowState(path, label string, seekEnd bool) (*followState, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	state := &followState{
		path:   path,
		label:  label,
		file:   f,
		reader: bufio.NewReader(f),
	}

	offset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		f.Close()
		return nil, err
	}
	state.offset = offset

	if seekEnd {
		offset, err = f.Seek(0, io.SeekEnd)
		if err != nil {
			f.Close()
			return nil, err
		}
		state.offset = offset
		state.reader.Reset(f)
	}

	return state, nil
}

func (s *followState) Close() {
	if s.file != nil {
		_ = s.file.Close()
		s.file = nil
	}
}

func (s *followState) readNewLines(write func(string)) (bool, error) {
	hadOutput := false

	for {
		line, err := s.reader.ReadString('\n')
		if err == nil {
			s.offset += int64(len(line))
			hadOutput = true
			write(line)
			continue
		}

		if err == io.EOF {
			fi, statErr := s.file.Stat()
			if statErr != nil {
				return hadOutput, statErr
			}
			if fi.Size() < s.offset {
				if _, seekErr := s.file.Seek(0, io.SeekStart); seekErr != nil {
					return hadOutput, seekErr
				}
				s.offset = 0
				s.reader.Reset(s.file)
				continue
			}
			return hadOutput, nil
		}

		return hadOutput, err
	}
}

// streamAllLogs follows multiple log files concurrently.
func streamAllLogs(paths []string) error {
	if len(paths) == 0 {
		fmt.Fprintln(os.Stdout, "No logs available")
		return nil
	}

	states := make([]*followState, 0, len(paths))
	for _, p := range paths {
		state, err := newFollowState(p, filepath.Base(filepath.Dir(p)), true)
		if err != nil {
			continue
		}
		states = append(states, state)
	}
	defer func() {
		for _, state := range states {
			state.Close()
		}
	}()

	idleWait := 200 * time.Millisecond

	for {
		hadOutput := false
		for _, state := range states {
			wrote, err := state.readNewLines(func(line string) {
				if strings.HasSuffix(line, "\n") {
					fmt.Fprintf(os.Stdout, "%-15s | %s", state.label, line)
				}
			})
			if err != nil {
				continue
			}
			hadOutput = hadOutput || wrote
		}
		if hadOutput {
			idleWait = 200 * time.Millisecond
			continue
		}
		time.Sleep(idleWait)
		if idleWait < 2*time.Second {
			idleWait *= 2
			if idleWait > 2*time.Second {
				idleWait = 2 * time.Second
			}
		}
	}
}

func printMergedLogs(paths []string, numLines int) error {
	// Read all lines from all files.
	type tagged struct {
		line string
	}
	var all []tagged
	for _, p := range paths {
		for _, line := range readLogLines(p, max(numLines, 200)) {
			if line != "" {
				all = append(all, tagged{line: line})
			}
		}
	}

	if len(all) == 0 {
		fmt.Fprintln(os.Stdout, "No logs available")
		return nil
	}

	// Lines are timestamp-prefixed "YYYY-MM-DD HH:MM:SS [...]" — stable sort.
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].line < all[j].line
	})

	start := 0
	if len(all) > numLines {
		start = len(all) - numLines
	}
	for _, t := range all[start:] {
		fmt.Fprintln(os.Stdout, t.line)
	}
	return nil
}

// printFilteredLogs prints lines matching a filter substring from all paths.
func printFilteredLogs(paths []string, numLines int, filter string) error {
	var matched []string
	for _, p := range paths {
		for _, line := range readLogLines(p, max(numLines*4, 400)) {
			if strings.Contains(line, filter) {
				matched = append(matched, line)
			}
		}
	}

	if len(matched) == 0 {
		fmt.Fprintln(os.Stdout, "No logs available")
		return nil
	}

	start := 0
	if len(matched) > numLines {
		start = len(matched) - numLines
	}
	for _, line := range matched[start:] {
		fmt.Fprintln(os.Stdout, line)
	}
	return nil
}

func streamLogs(paths []string) error {
	// Print last 20 lines from each file as initial buffer.
	for _, p := range paths {
		printLogs(p, 20)
	}

	states := make([]*followState, 0, len(paths))
	for _, p := range paths {
		state, err := newFollowState(p, "", true)
		if err != nil {
			continue
		}
		states = append(states, state)
	}
	defer func() {
		for _, state := range states {
			state.Close()
		}
	}()

	idleWait := 200 * time.Millisecond

	for {
		hadOutput := false
		for _, state := range states {
			wrote, err := state.readNewLines(func(line string) {
				fmt.Print(line)
			})
			if err != nil {
				continue
			}
			hadOutput = hadOutput || wrote
		}
		if hadOutput {
			idleWait = 200 * time.Millisecond
			continue
		}
		time.Sleep(idleWait)
		if idleWait < 2*time.Second {
			idleWait *= 2
			if idleWait > 2*time.Second {
				idleWait = 2 * time.Second
			}
		}
	}
}

// printLogs reads and prints the last n lines from a single file.
func printLogs(path string, numLines int) error {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	if _, err := seekTailWindow(f, maxLogReadBytes); err != nil {
		return fmt.Errorf("error seeking log file: %w", err)
	}

	scanner := bufio.NewScanner(f)
	buf := make([]string, numLines)
	i := 0
	count := 0
	for scanner.Scan() {
		buf[i%numLines] = scanner.Text()
		i++
		count++
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("error reading log file: %w", err)
	}

	start := 0
	if count > numLines {
		start = i % numLines
	}
	for j := 0; j < numLines && j < count; j++ {
		line := buf[(start+j)%numLines]
		if line != "" {
			fmt.Fprintln(os.Stdout, line)
		}
	}
	return nil
}

func seekTailWindow(f *os.File, maxBytes int64) (int64, error) {
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if fi.Size() <= maxBytes {
		return f.Seek(0, io.SeekStart)
	}
	return f.Seek(-maxBytes, io.SeekEnd)
}

// filterErrorLines returns only lines containing [err].
func filterErrorLines(s string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "[err]") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// resolveLogPaths resolves a process target (id, name, or prefix) to its log files on disk.
// Returns both stdout.log and stderr.log if they exist.
func resolveLogPaths(target string) []string {
	dd := dataDir()
	appsDir := filepath.Join(dd, "apps")

	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil
	}

	// Collect app dirs sorted by name for stable ID ordering.
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}

	var name string
	// Try exact name match first.
	for _, n := range names {
		if n == target {
			name = n
			break
		}
	}

	if name == "" {
		// Try numeric ID.
		var id int
		if _, err := fmt.Sscanf(target, "%d", &id); err == nil && id >= 0 && id < len(names) {
			name = names[id]
		}
	}

	if name == "" {
		// Try unique prefix match.
		var matched []string
		for _, n := range names {
			if strings.HasPrefix(n, target) {
				matched = append(matched, n)
			}
		}
		if len(matched) == 1 {
			name = matched[0]
		}
	}

	if name == "" {
		return nil
	}

	var paths []string
	for _, suffix := range []string{"stdout.log", "stderr.log"} {
		p := filepath.Join(appsDir, name, suffix)
		if _, err := os.Stat(p); err == nil {
			paths = append(paths, p)
		}
	}
	return paths
}

// filterPaths returns only paths ending with the given suffix.
func filterPaths(paths []string, suffix string) []string {
	var filtered []string
	for _, p := range paths {
		if strings.HasSuffix(p, suffix) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
