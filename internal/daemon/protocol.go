package daemon

import "encoding/json"

// Action constants
const (
	ActionStart         = "start"
	ActionStartAll      = "start_all"
	ActionStop          = "stop"
	ActionRestart       = "restart"
	ActionReload        = "reload"
	ActionDelete        = "delete"
	ActionList          = "list"
	ActionStatus        = "status"
	ActionLogs          = "logs"
	ActionSave          = "save"
	ActionResurrect     = "resurrect"
	ActionPing          = "ping"
	ActionCronList      = "cron_list"
	ActionCronStart     = "cron_start"
	ActionCronStop      = "cron_stop"
	ActionCronRun       = "cron_run"
	ActionRollingReload = "rolling_reload"
	ActionConfigReload  = "config_reload"
	ActionWebStart      = "web_start"
)

// Request is an IPC request from CLI to daemon.
type Request struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Response is an IPC response from daemon to CLI.
type Response struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// ErrorResponse creates a failed response.
func ErrorResponse(err error) Response {
	return Response{
		Success: false,
		Error:   err.Error(),
	}
}

// DataResponse creates a successful response with data.
func DataResponse(data interface{}) (Response, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Response{}, err
	}
	return Response{
		Success: true,
		Data:    raw,
	}, nil
}

// StartPayload is the payload for the "start" action.
type StartPayload struct {
	Name          string            `json:"name,omitempty"`
	Runtime       string            `json:"runtime,omitempty"`
	Entrypoint    string            `json:"entrypoint"`
	Args          []string          `json:"args,omitempty"`
	Cwd           string            `json:"cwd,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	RestartPolicy string            `json:"restart_policy,omitempty"`
	MaxRestarts   int               `json:"max_restarts,omitempty"`
}

// StartAllPayload is the payload for the "start_all" action.
type StartAllPayload struct {
	Only   string `json:"only,omitempty"`   // comma-separated process names to start
	Config string `json:"config,omitempty"` // config file path (loads on the fly)
}

// StopPayload is the payload for the "stop" action.
type StopPayload struct {
	Target   string `json:"target"`
	Force    bool   `json:"force,omitempty"`
	Timeout  string `json:"timeout,omitempty"` // duration string like "5s"
	Parallel bool   `json:"parallel,omitempty"`
	Graceful bool   `json:"graceful,omitempty"`
}

// LogsPayload is the payload for the "logs" action.
type LogsPayload struct {
	Target string `json:"target"`
	Lines  int    `json:"lines,omitempty"`
}

// CronPayload is the payload for cron actions.
type CronPayload struct {
	Name string `json:"name,omitempty"`
}

// RollingReloadPayload is the payload for the "rolling_reload" action.
type RollingReloadPayload struct {
	Target            string `json:"target"`
	BatchSize         int    `json:"batch_size,omitempty"`
	WaitReady         bool   `json:"wait_ready,omitempty"`
	RollbackOnFailure bool   `json:"rollback_on_failure,omitempty"`
}

// WebStartPayload is the payload for the "web_start" action.
type WebStartPayload struct {
	Listen string `json:"listen,omitempty"`
}

// ConfigReloadPayload is the payload for the "config_reload" action.
type ConfigReloadPayload struct {
	ConfigPath string `json:"config_path,omitempty"`
}
