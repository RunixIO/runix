package migrate

// PM2AppConfig represents a single PM2 process definition.
// Fields mirror the PM2 ecosystem.config schema.
type PM2AppConfig struct {
	Name             string            `json:"name" yaml:"name"`
	Script           string            `json:"script" yaml:"script"`
	Exec             string            `json:"exec" yaml:"exec"`
	Cwd              string            `json:"cwd" yaml:"cwd"`
	Args             interface{}       `json:"args" yaml:"args"`
	Env              map[string]string `json:"env" yaml:"env"`
	ExecInterpreter  string            `json:"exec_interpreter" yaml:"exec_interpreter"`
	Interpreter      string            `json:"interpreter" yaml:"interpreter"`
	Instances        int               `json:"instances" yaml:"instances"`
	ExecMode         string            `json:"exec_mode" yaml:"exec_mode"`
	Watch            interface{}       `json:"watch" yaml:"watch"`
	IgnoreWatch      []string          `json:"ignore_watch" yaml:"ignore_watch"`
	WatchDelay       int               `json:"watch_delay" yaml:"watch_delay"`
	Autorestart      *bool             `json:"autorestart" yaml:"autorestart"`
	MaxRestarts      int               `json:"max_restarts" yaml:"max_restarts"`
	MaxMemoryRestart string            `json:"max_memory_restart" yaml:"max_memory_restart"`
	RestartDelay     int               `json:"restart_delay" yaml:"restart_delay"`
	ExpBackoffDelay  int               `json:"exp_backoff_restart_delay" yaml:"exp_backoff_restart_delay"`
	KillTimeout      int               `json:"kill_timeout" yaml:"kill_timeout"`
	CronRestart      string            `json:"cron_restart" yaml:"cron_restart"`
	Namespace        string            `json:"namespace" yaml:"namespace"`
	NodeArgs         interface{}       `json:"node_args" yaml:"node_args"`
	UID              string            `json:"uid" yaml:"uid"`
	GID              string            `json:"gid" yaml:"gid"`
	LogDateFormat    string            `json:"log_date_format" yaml:"log_date_format"`
	OutFile          string            `json:"out_file" yaml:"out_file"`
	ErrorFile        string            `json:"error_file" yaml:"error_file"`
	LogFile          string            `json:"log_file" yaml:"log_file"`
	MergeLogs        bool              `json:"merge_logs" yaml:"merge_logs"`
	MinUptime        interface{}       `json:"min_uptime" yaml:"min_uptime"`
	ListenTimeout    int               `json:"listen_timeout" yaml:"listen_timeout"`
	WaitReady        bool              `json:"wait_ready" yaml:"wait_ready"`
	StopExitCodes    []int             `json:"stop_exit_codes" yaml:"stop_exit_codes"`
	Autostart        *bool             `json:"autostart" yaml:"autostart"`
	// Env variants — PM2 supports arbitrary env_<name> keys.
	// We handle known ones explicitly and capture others from raw JSON/YAML.
	EnvProduction map[string]string `json:"env_production" yaml:"env_production"`
	EnvStaging    map[string]string `json:"env_staging" yaml:"env_staging"`
}

// PM2Config represents a top-level PM2 ecosystem configuration.
type PM2Config struct {
	Apps []PM2AppConfig `json:"apps" yaml:"apps"`
	// Deploy is the PM2 deployment section (no Runix equivalent).
	Deploy interface{} `json:"deploy,omitempty" yaml:"deploy,omitempty"`
}

// PM2DumpEntry represents a single entry from ~/.pm2/dump.pm2.
type PM2DumpEntry struct {
	Name             string            `json:"name"`
	PmExecPath       string            `json:"pm_exec_path"`
	PmCwd            string            `json:"pm_cwd"`
	Env              map[string]string `json:"env"`
	ExecInterpreter  string            `json:"exec_interpreter"`
	ExecMode         string            `json:"exec_mode"`
	Instances        int               `json:"instances"`
	PmWatch          interface{}       `json:"pm_watch"`
	Autorestart      bool              `json:"autorestart"`
	NodeArgs         interface{}       `json:"node_args"`
	CronRestart      string            `json:"pm_cron_restart"`
	Namespace        string            `json:"namespace"`
	Args             interface{}       `json:"args"`
	Script           string            `json:"script"`
	Cwd              string            `json:"cwd"`
	MaxMemoryRestart string            `json:"max_memory_restart"`
	MaxRestarts      int               `json:"max_restarts"`
	KillTimeout      int               `json:"kill_timeout"`
}
