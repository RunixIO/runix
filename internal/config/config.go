package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/internal/secrets"
	"github.com/runixio/runix/pkg/types"
)

var secretPattern = regexp.MustCompile(`\$\{([A-Za-z0-9_]+)\}`)

// Load reads a config file and returns a validated RunixConfig.
// If path is empty, searches for runix.yaml/runix.json/runix.toml in the
// current directory. Returns an empty config with defaults applied if no file
// is found (not an error).
func Load(path string) (*types.RunixConfig, error) {
	var cfg *types.RunixConfig
	var configDir string

	if path != "" {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolving config path: %w", err)
		}
		cfg, err = loadFromFile(abs)
		if err != nil {
			return nil, err
		}
		configDir = filepath.Dir(abs)
		log.Debug().Str("path", abs).Msg("loaded config file")
	} else {
		candidates := []string{"runix.yaml", "runix.yml", "runix.json", "runix.toml"}
		found := false
		for _, name := range candidates {
			if _, err := os.Stat(name); err == nil {
				abs, _ := filepath.Abs(name)
				cfg, err = loadFromFile(abs)
				if err != nil {
					return nil, err
				}
				configDir = filepath.Dir(abs)
				log.Debug().Str("path", abs).Msg("auto-detected config file")
				found = true
				break
			}
		}
		if !found {
			log.Debug().Msg("no config file found, using defaults")
			cfg = &types.RunixConfig{}
		}
	}

	ApplyDefaults(cfg)

	// Resolve extends (deep merge).
	resolved, err := ResolveExtends(cfg.Processes)
	if err != nil {
		return nil, err
	}
	cfg.Processes = resolved

	// Set process cwd from config file directory when empty.
	if configDir != "" {
		for i := range cfg.Processes {
			if cfg.Processes[i].Cwd == "" {
				cfg.Processes[i].Cwd = configDir
			}
		}
	}

	if err := resolveSecrets(cfg); err != nil {
		return nil, err
	}

	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks the config for errors.
func Validate(cfg *types.RunixConfig) error {
	var errs []string

	for i := range cfg.Processes {
		if err := cfg.Processes[i].Validate(); err != nil {
			errs = append(errs, err.Error())
		}
	}

	for i := range cfg.Cron {
		if err := cfg.Cron[i].Validate(); err != nil {
			errs = append(errs, err.Error())
		}
	}

	for name, d := range cfg.Deploy {
		if d.Host == "" {
			errs = append(errs, fmt.Sprintf("deploy.%s.host is required", name))
		}
		if d.User == "" {
			errs = append(errs, fmt.Sprintf("deploy.%s.user is required", name))
		}
		if d.Path == "" {
			errs = append(errs, fmt.Sprintf("deploy.%s.path is required", name))
		}
	}

	// Check for duplicate process names.
	names := make(map[string]int, len(cfg.Processes))
	for i, p := range cfg.Processes {
		if p.Name == "" {
			continue
		}
		if first, ok := names[p.Name]; ok {
			errs = append(errs, fmt.Sprintf("duplicate process name %q (indices %d and %d)", p.Name, first, i))
		} else {
			names[p.Name] = i
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  %s", strings.Join(errs, "\n  "))
	}

	return nil
}

func resolveSecrets(cfg *types.RunixConfig) error {
	resolved, err := secrets.Resolve(cfg.Secrets)
	if err != nil {
		return err
	}

	for i := range cfg.Processes {
		env, err := interpolateEnv(cfg.Processes[i].Env, resolved)
		if err != nil {
			return fmt.Errorf("process %q env: %w", cfg.Processes[i].Name, err)
		}
		cfg.Processes[i].Env = env
	}

	for i := range cfg.Cron {
		env, err := interpolateEnv(cfg.Cron[i].Env, resolved)
		if err != nil {
			return fmt.Errorf("cron %q env: %w", cfg.Cron[i].Name, err)
		}
		cfg.Cron[i].Env = env
	}

	return nil
}

func interpolateEnv(env map[string]string, resolvedSecrets map[string]string) (map[string]string, error) {
	if len(env) == 0 {
		return env, nil
	}

	out := make(map[string]string, len(env))
	for key, value := range env {
		resolved, err := interpolateSecrets(value, resolvedSecrets)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		out[key] = resolved
	}

	return out, nil
}

func interpolateSecrets(value string, resolvedSecrets map[string]string) (string, error) {
	var unresolved string
	result := secretPattern.ReplaceAllStringFunc(value, func(match string) string {
		name := secretPattern.FindStringSubmatch(match)[1]
		secret, ok := resolvedSecrets[name]
		if !ok {
			unresolved = name
			return match
		}
		return secret
	})

	if unresolved != "" {
		return "", fmt.Errorf("unknown secret %q", unresolved)
	}

	return result, nil
}
