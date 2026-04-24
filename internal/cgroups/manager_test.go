package cgroups

import (
	"testing"
)

func TestParseCPUQuota(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"50%", 50000},
		{"50", 50000},
		{"100%", 100000},
		{"200%", 200000},
		{"0.5", 500},
	}
	for _, tt := range tests {
		got := parseCPUQuota(tt.input)
		if got != tt.expected {
			t.Errorf("parseCPUQuota(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestParseMemoryLimit(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"512MB", 512 * 1024 * 1024},
		{"1GB", 1 * 1024 * 1024 * 1024},
		{"256MB", 256 * 1024 * 1024},
		{"1024KB", 1024 * 1024},
	}
	for _, tt := range tests {
		got := parseMemoryLimit(tt.input)
		if got != tt.expected {
			t.Errorf("parseMemoryLimit(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}
