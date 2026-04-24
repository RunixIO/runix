package metrics

import (
	"fmt"
	"io"
	"time"

	"github.com/runixio/runix/pkg/types"
)

// WritePrometheus writes process metrics in Prometheus text exposition format.
func WritePrometheus(w io.Writer, procs []types.ProcessInfo) {
	stateCounts := make(map[string]int)
	for _, p := range procs {
		stateCounts[string(p.State)]++
	}

	// Process count by state.
	for state, count := range stateCounts {
		fmt.Fprintf(w, "# HELP runix_process_count Number of processes by state.\n")
		fmt.Fprintf(w, "# TYPE runix_process_count gauge\n")
		fmt.Fprintf(w, "runix_process_count{state=%q} %d\n", state, count)
	}

	// Per-process metrics.
	for _, p := range procs {
		labels := fmt.Sprintf(`name=%q,namespace=%q`, p.Name, p.Namespace)

		fmt.Fprintf(w, "runix_process_restarts_total{%s} %d\n", labels, p.Restarts)

		if p.StartedAt != nil {
			uptime := time.Since(*p.StartedAt).Seconds()
			fmt.Fprintf(w, "runix_process_uptime_seconds{%s} %.0f\n", labels, uptime)
		}

		if p.MemBytes > 0 {
			fmt.Fprintf(w, "runix_process_memory_bytes{%s} %d\n", labels, p.MemBytes)
		}

		if p.CPUPercent > 0 {
			fmt.Fprintf(w, "runix_process_cpu_percent{%s} %.2f\n", labels, p.CPUPercent)
		}
	}

	// Total process count.
	fmt.Fprintf(w, "runix_process_count_total %d\n", len(procs))
}
