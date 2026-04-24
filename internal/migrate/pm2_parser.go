package migrate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"go.yaml.in/yaml/v3"
)

// AutoDetect searches dir for PM2 config files in priority order.
// Returns the path of the first match, or empty string if none found.
func AutoDetect(dir string) string {
	candidates := []string{
		"ecosystem.config.js",
		"ecosystem.config.cjs",
		"ecosystem.config.mjs",
		"ecosystem.config.json",
		"ecosystem.config.yml",
		"ecosystem.config.yaml",
	}
	for _, name := range candidates {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// FindDumpFile returns the path to ~/.pm2/dump.pm2 if it exists, empty string otherwise.
func FindDumpFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(home, ".pm2", "dump.pm2")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// ParseFile parses a PM2 config file, auto-detecting the format by extension.
func ParseFile(path string) (*PM2Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	// Detect dump.pm2 by filename
	if base == "dump.pm2" {
		return ParseDump(data)
	}

	switch ext {
	case ".json":
		return parseJSON(data)
	case ".yml", ".yaml":
		return parseYAML(data)
	case ".js", ".cjs", ".mjs":
		return parseJS(path)
	default:
		// Try JSON first, then YAML
		if cfg, err := parseJSON(data); err == nil {
			return cfg, nil
		}
		return parseYAML(data)
	}
}

// ParseDump parses a dump.pm2 JSON array into a PM2Config.
func ParseDump(data []byte) (*PM2Config, error) {
	var entries []PM2DumpEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing dump.pm2: %w", err)
	}

	cfg := &PM2Config{}
	for i := range entries {
		e := &entries[i]
		app := PM2AppConfig{
			Name:             e.Name,
			Script:           coalesceStr(e.Script, e.PmExecPath),
			Cwd:              coalesceStr(e.Cwd, e.PmCwd),
			Args:             e.Args,
			Env:              e.Env,
			ExecInterpreter:  e.ExecInterpreter,
			ExecMode:         e.ExecMode,
			Instances:        e.Instances,
			Watch:            e.PmWatch,
			MaxMemoryRestart: e.MaxMemoryRestart,
			MaxRestarts:      e.MaxRestarts,
			KillTimeout:      e.KillTimeout,
			CronRestart:      e.CronRestart,
			Namespace:        e.Namespace,
			NodeArgs:         e.NodeArgs,
		}
		if e.Autorestart {
			app.Autorestart = boolPtr(true)
		}
		cfg.Apps = append(cfg.Apps, app)
	}
	return cfg, nil
}

func parseJSON(data []byte) (*PM2Config, error) {
	// Try wrapped format: {"apps": [...]}
	var wrapped struct {
		Apps []PM2AppConfig `json:"apps"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Apps) > 0 {
		return &PM2Config{Apps: wrapped.Apps}, nil
	}

	// Try bare array: [...]
	var apps []PM2AppConfig
	if err := json.Unmarshal(data, &apps); err == nil && len(apps) > 0 {
		return &PM2Config{Apps: apps}, nil
	}

	// Try single object (not wrapped): {...}
	var single PM2AppConfig
	if err := json.Unmarshal(data, &single); err == nil && single.Script != "" {
		return &PM2Config{Apps: []PM2AppConfig{single}}, nil
	}

	return nil, fmt.Errorf("no PM2 apps found in JSON config")
}

func parseYAML(data []byte) (*PM2Config, error) {
	// Try wrapped format: apps: [...]
	var wrapped struct {
		Apps []PM2AppConfig `yaml:"apps"`
	}
	if err := yaml.Unmarshal(data, &wrapped); err == nil && len(wrapped.Apps) > 0 {
		return &PM2Config{Apps: wrapped.Apps}, nil
	}

	// Try bare array
	var apps []PM2AppConfig
	if err := yaml.Unmarshal(data, &apps); err == nil && len(apps) > 0 {
		return &PM2Config{Apps: apps}, nil
	}

	// Try single object
	var single PM2AppConfig
	if err := yaml.Unmarshal(data, &single); err == nil && single.Script != "" {
		return &PM2Config{Apps: []PM2AppConfig{single}}, nil
	}

	return nil, fmt.Errorf("no PM2 apps found in YAML config")
}

func parseJS(path string) (*PM2Config, error) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return nil, fmt.Errorf(
			"node is required to parse JavaScript ecosystem files: %w\n"+
				"Convert to ecosystem.config.json or install Node.js",
			err,
		)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}
	dir := filepath.Dir(absPath)
	base := filepath.Base(absPath)

	// For ESM (.mjs), use import() instead of require().
	var script string
	if strings.HasSuffix(base, ".mjs") {
		script = fmt.Sprintf(
			`import('%s').then(m => console.log(JSON.stringify(m.default || m)))`,
			"./"+base,
		)
	} else {
		script = fmt.Sprintf(
			`console.log(JSON.stringify(require('./%s')))`,
			base,
		)
	}

	cmd := exec.Command(nodePath, "-e", script)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("node failed: %s: %s", err, string(ee.Stderr))
		}
		return nil, fmt.Errorf("running node: %w", err)
	}

	return parseJSON(output)
}

// NormalizeArgs converts PM2's args field (string or []string) to []string.
func NormalizeArgs(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch a := v.(type) {
	case string:
		if a == "" {
			return nil
		}
		return splitQuoted(a)
	case []string:
		return a
	case []interface{}:
		result := make([]string, 0, len(a))
		for _, item := range a {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	}
	return nil
}

// splitQuoted splits a string into arguments, respecting single and double quotes.
func splitQuoted(s string) []string {
	var args []string
	var buf strings.Builder
	var quote rune
	escaped := false

	for _, r := range s {
		if escaped {
			buf.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote == 0 {
			if r == '"' || r == '\'' {
				quote = r
				continue
			}
			if r == ' ' || r == '\t' {
				if buf.Len() > 0 {
					args = append(args, buf.String())
					buf.Reset()
				}
				continue
			}
		} else {
			if r == quote {
				quote = 0
				continue
			}
		}
		buf.WriteRune(r)
	}
	if buf.Len() > 0 {
		args = append(args, buf.String())
	}
	return args
}

// RuntimeNumCPU returns the number of CPUs (for PM2's instances: "max" / 0).
func RuntimeNumCPU() int {
	return runtime.NumCPU()
}

func boolPtr(b bool) *bool { return &b }

func coalesceStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
