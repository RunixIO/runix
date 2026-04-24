package events

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const maxEventLogBytes int64 = 4 << 20 // 4 MiB

type Store struct {
	mu      sync.Mutex
	path    string
	file    *os.File
	written atomic.Int64 // bytes appended since last compact
}

func NewStore(dir string) *Store {
	if dir == "" {
		return nil
	}
	return &Store{
		path: filepath.Join(dir, "events.log"),
	}
}

func (s *Store) Append(evt Event) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file == nil {
		f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return
		}
		s.file = f
	}

	data, _ := json.Marshal(evt)
	n, _ := s.file.Write(data)
	_, _ = s.file.Write([]byte("\n"))
	s.written.Add(int64(n + 1))

	// Only compact when we've appended enough since the last compact.
	if s.written.Load() >= maxEventLogBytes {
		_ = s.compactLocked()
		s.written.Store(0)
	}
}

func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil {
		err := s.file.Close()
		s.file = nil
		return err
	}
	return nil
}

func (s *Store) Query(since time.Time, eventTypes ...EventType) []Event {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Open(s.path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	typeSet := make(map[EventType]bool)
	for _, et := range eventTypes {
		typeSet[et] = true
	}

	var results []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var evt Event
		if err := json.Unmarshal(scanner.Bytes(), &evt); err != nil {
			continue
		}
		if !evt.Timestamp.After(since) && !evt.Timestamp.Equal(since) {
			continue
		}
		if len(typeSet) > 0 && !typeSet[evt.Type] {
			continue
		}
		results = append(results, evt)
	}
	return results
}

func (s *Store) compactLocked() error {
	f, err := os.OpenFile(s.path, os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if fi.Size() <= maxEventLogBytes {
		return nil
	}

	if _, err := f.Seek(-maxEventLogBytes, io.SeekEnd); err != nil {
		return err
	}

	buf := make([]byte, maxEventLogBytes)
	n, err := io.ReadFull(f, buf)
	if n > 0 {
		data := buf[:n]
		// Drop a partial first line so the retained file remains valid NDJSON.
		if idx := bytes.IndexByte(data, '\n'); idx >= 0 && idx+1 < len(data) {
			data = data[idx+1:]
		}

		if err := f.Truncate(0); err != nil {
			return err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	}
	return err
}
