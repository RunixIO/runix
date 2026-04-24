package output

import "testing"

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0b"},
		{512, "512b"},
		{1023, "1023b"},
		{1024, "1.0kb"},
		{1536, "1.5kb"},
		{1048576, "1.0mb"},
		{4734976, "4.5mb"},
		{1073741824, "1.0gb"},
		{1610612736, "1.5gb"},
	}
	for _, tt := range tests {
		got := FormatBytes(tt.input)
		if got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatCPU(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{0, "0.0%"},
		{2.3, "2.3%"},
		{100.0, "100.0%"},
		{0.1, "0.1%"},
	}
	for _, tt := range tests {
		got := FormatCPU(tt.input)
		if got != tt.want {
			t.Errorf("FormatCPU(%f) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
