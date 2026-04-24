package sdk

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const maxTailReadBytes int64 = 1 << 20 // 1 MiB

// Logs returns a reader for the process log output. The reader supports tailing
// (reading the last N lines) and following (streaming new lines as they arrive).
//
// The caller must close the returned reader when done. If Follow is true, the
// reader blocks until the context is cancelled or the log file becomes
// unavailable.
func (m *Manager) Logs(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error) {
	var logPath string
	if opts.Stderr {
		logPath = m.sup.LogPathStderr(id)
	} else {
		logPath = m.sup.LogPath(id)
	}

	if logPath == "" {
		return nil, fmt.Errorf("sdk: no log path for process %q", id)
	}

	if _, err := os.Stat(logPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("sdk: log file not found for process %q", id)
		}
		return nil, fmt.Errorf("sdk: failed to access log for %q: %w", id, err)
	}

	// Create a cancellable context so Close() on the reader unblocks the goroutine.
	logCtx, cancel := context.WithCancel(ctx)

	r, w := io.Pipe()

	go func() {
		defer w.Close()

		f, err := os.Open(logPath)
		if err != nil {
			return
		}
		defer f.Close()

		// Tail: read and output the last N lines.
		if opts.Tail > 0 {
			lines := readLastLines(f, opts.Tail)
			for _, line := range lines {
				select {
				case <-logCtx.Done():
					return
				default:
				}
				if _, err := fmt.Fprintln(w, line); err != nil {
					return
				}
			}
		}

		if !opts.Follow {
			return
		}

		// Follow: seek to end and stream new content.
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			return
		}

		reader := bufio.NewReader(f)
		idleWait := 200 * time.Millisecond
		for {
			select {
			case <-logCtx.Done():
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					if fi, statErr := f.Stat(); statErr == nil && fi.Size() < currentOffset(f) {
						if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
							return
						}
						reader.Reset(f)
						idleWait = 200 * time.Millisecond
						continue
					}
					select {
					case <-logCtx.Done():
						return
					case <-time.After(idleWait):
						if idleWait < 2*time.Second {
							idleWait *= 2
							if idleWait > 2*time.Second {
								idleWait = 2 * time.Second
							}
						}
						continue
					}
				}
				return
			}
			idleWait = 200 * time.Millisecond

			// Write the line. If the pipe reader has been closed, this returns
			// an error and we exit the goroutine.
			if _, err := w.Write([]byte(line)); err != nil {
				return
			}
		}
	}()

	return &logStreamReader{
		ReadCloser: r,
		cancel:     cancel,
	}, nil
}

// logStreamReader wraps an io.ReadCloser with context cancellation support.
type logStreamReader struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

func (l *logStreamReader) Close() error {
	l.once.Do(l.cancel)
	return l.ReadCloser.Close()
}

// readLastLines reads the last n lines from the current file position.
// It uses a circular buffer to avoid loading the entire file into memory.
func readLastLines(f *os.File, n int) []string {
	if _, err := seekTailWindow(f, maxTailReadBytes); err != nil {
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

	result := make([]string, 0, min(count, n))
	for j := 0; j < n && j < count; j++ {
		line := buf[(start+j)%n]
		if line != "" {
			result = append(result, line)
		}
	}
	return result
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

func currentOffset(f *os.File) int64 {
	offset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0
	}
	return offset
}
