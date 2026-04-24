package runtime

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

// Detector auto-detects the runtime from project files.
type Detector struct {
	runtimes []Runtime
}

// NewDetector returns a Detector with the built-in runtime adapters registered.
func NewDetector() *Detector {
	d := &Detector{
		runtimes: []Runtime{
			&GoRuntime{},
			&PythonRuntime{},
			&NodeRuntime{},
			&BunRuntime{},
			&DenoRuntime{},
			&RubyRuntime{},
			&PHPRuntime{},
		},
	}
	log.Debug().Int("count", len(d.runtimes)).Msg("detector initialized with runtimes")
	return d
}

// Detect checks each registered runtime in order and returns the first match.
func (d *Detector) Detect(dir string) (Runtime, error) {
	for _, r := range d.runtimes {
		if r.Detect(dir) {
			log.Debug().Str("runtime", r.Name()).Str("dir", dir).Msg("detected runtime")
			return r, nil
		}
	}
	return nil, fmt.Errorf("no runtime detected in %q", dir)
}

// Get looks up a runtime by name (e.g. "go", "python", "node", "bun").
func (d *Detector) Get(name string) (Runtime, error) {
	for _, r := range d.runtimes {
		if r.Name() == name {
			return r, nil
		}
	}
	return nil, fmt.Errorf("unknown runtime %q", name)
}
