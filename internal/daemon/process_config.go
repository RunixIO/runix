package daemon

import "github.com/runixio/runix/pkg/types"

// ExpandProcessInstances expands each process config into its configured
// instance copies while preserving the incoming order.
func ExpandProcessInstances(processes []types.ProcessConfig) []types.ProcessConfig {
	if len(processes) == 0 {
		return nil
	}

	var expanded []types.ProcessConfig
	for _, cfg := range processes {
		instances := cfg.Instances
		if instances < 1 {
			instances = 1
		}

		for i := 0; i < instances; i++ {
			instanceCfg := cfg
			instanceCfg.InstanceIndex = i
			if instances > 1 {
				instanceCfg.Name = instanceCfg.FullName()
			}
			expanded = append(expanded, instanceCfg)
		}
	}

	return expanded
}
