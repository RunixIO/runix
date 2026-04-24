package supervisor

import (
	"fmt"
	"sort"

	"github.com/runixio/runix/pkg/types"
)

// ResolveStartOrder returns process configs sorted by dependency order (topological sort)
// with priority as secondary sort (lower priority = starts first).
// Returns error if there's a cycle or a missing dependency reference.
func ResolveStartOrder(configs []types.ProcessConfig) ([]types.ProcessConfig, error) {
	// Build name -> config map.
	byName := make(map[string]*types.ProcessConfig)
	for i := range configs {
		byName[configs[i].Name] = &configs[i]
	}

	// Validate all depends_on references exist.
	for _, cfg := range configs {
		for _, dep := range cfg.DependsOn {
			if _, ok := byName[dep]; !ok {
				return nil, fmt.Errorf("process %q depends on %q which does not exist", cfg.Name, dep)
			}
		}
	}

	// Build adjacency list: dep -> dependents.
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	for _, cfg := range configs {
		inDegree[cfg.Name] = len(cfg.DependsOn)
		for _, dep := range cfg.DependsOn {
			graph[dep] = append(graph[dep], cfg.Name)
		}
	}

	// Kahn's algorithm for topological sort.
	var queue []types.ProcessConfig
	for _, cfg := range configs {
		if inDegree[cfg.Name] == 0 {
			queue = append(queue, cfg)
		}
	}

	// Sort initial queue by priority.
	sort.Slice(queue, func(i, j int) bool {
		return queue[i].Priority < queue[j].Priority
	})

	var result []types.ProcessConfig
	for len(queue) > 0 {
		// Take first element.
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Process dependents.
		var nextBatch []types.ProcessConfig
		for _, dependent := range graph[current.Name] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				nextBatch = append(nextBatch, *byName[dependent])
			}
		}
		// Sort by priority.
		sort.Slice(nextBatch, func(i, j int) bool {
			return nextBatch[i].Priority < nextBatch[j].Priority
		})
		queue = append(queue, nextBatch...)
	}

	if len(result) != len(configs) {
		// Find which nodes are in the cycle.
		var cycleNodes []string
		for _, cfg := range configs {
			if inDegree[cfg.Name] > 0 {
				cycleNodes = append(cycleNodes, cfg.Name)
			}
		}
		return nil, fmt.Errorf("circular dependency detected among: %v", cycleNodes)
	}

	return result, nil
}
