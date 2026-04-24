package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/runixio/runix/pkg/types"
	"github.com/spf13/viper"
)

// loadFromFile reads a specific file using viper and returns the parsed config.
func loadFromFile(path string) (*types.RunixConfig, error) {
	v := viper.New()
	v.SetConfigFile(path)

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		v.SetConfigType("yaml")
	case ".json":
		v.SetConfigType("json")
	case ".toml":
		v.SetConfigType("toml")
	default:
		return nil, fmt.Errorf("unsupported config file extension: %s", ext)
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	var cfg types.RunixConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}

	return &cfg, nil
}
