package types

// WatchConfig holds file watching configuration.
type WatchConfig struct {
	Enabled  bool     `json:"enabled" yaml:"enabled"`
	Paths    []string `json:"paths,omitempty" yaml:"paths,omitempty"`
	Ignore   []string `json:"ignore,omitempty" yaml:"ignore,omitempty"`
	Debounce string   `json:"debounce,omitempty" yaml:"debounce,omitempty"`
}
