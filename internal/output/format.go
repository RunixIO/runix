package output

import "fmt"

// FormatBytes returns a human-readable byte string (e.g. "45.2mb", "1.5gb", "0b").
func FormatBytes(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1fgb", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1fmb", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1fkb", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%db", b)
	}
}

// FormatCPU returns a formatted CPU percentage string (e.g. "2.3%", "0.0%").
func FormatCPU(pct float64) string {
	return fmt.Sprintf("%.1f%%", pct)
}
