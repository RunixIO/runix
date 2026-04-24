package logutil

import (
	"io"
	"os"
)

// ReadFileBounded reads up to maxBytes from a file. If the file exceeds
// maxBytes, only the last maxBytes are returned (tail semantics).
func ReadFileBounded(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if fi.Size() <= maxBytes {
		return io.ReadAll(f)
	}

	if _, err := f.Seek(-maxBytes, io.SeekEnd); err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}

// TailLines returns the last n lines from s. If n <= 0, returns s unchanged.
func TailLines(s string, n int) string {
	if n <= 0 {
		return s
	}
	count := 0
	idx := len(s)
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '\n' {
			count++
			if count == n {
				idx = i + 1
				break
			}
		}
	}
	if idx < len(s) {
		return s[idx:]
	}
	return s
}
