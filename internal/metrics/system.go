package metrics

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// GetSystemMetrics returns current system-wide resource metrics (Linux only).
func GetSystemMetrics() SystemMetrics {
	if runtime.GOOS != "linux" {
		return SystemMetrics{}
	}

	var sm SystemMetrics

	// Read /proc/meminfo for memory stats.
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			key := strings.TrimSuffix(parts[0], ":")
			val, _ := strconv.ParseInt(parts[1], 10, 64)
			// Convert from kB to bytes.
			val *= 1024

			switch key {
			case "MemTotal":
				sm.TotalMemory = val
			case "MemFree":
				sm.FreeMemory = val
			case "MemAvailable":
				// Available is more accurate than Free.
				sm.FreeMemory = val
			}
		}
		sm.UsedMemory = sm.TotalMemory - sm.FreeMemory
	}

	// Read /proc/loadavg for CPU load.
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			sm.CPULoad1, _ = strconv.ParseFloat(fields[0], 64)
			sm.CPULoad5, _ = strconv.ParseFloat(fields[1], 64)
			sm.CPULoad15, _ = strconv.ParseFloat(fields[2], 64)
			// Running/total processes from field 3 (format: "running/total").
			if len(fields) >= 4 {
				procParts := strings.Split(fields[3], "/")
				if len(procParts) == 2 {
					sm.ProcessCount, _ = strconv.Atoi(procParts[1])
				}
			}
		}
	}

	// Read /proc/uptime for system uptime.
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 1 {
			upSecs, _ := strconv.ParseFloat(fields[0], 64)
			sm.Uptime = int64(upSecs)
		}
	}

	return sm
}
