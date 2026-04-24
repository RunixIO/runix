# Logging

Runix captures per-process stdout and stderr to log files with automatic rotation.

## Log File Structure

```
~/.runix/
└── apps/
    └── {process-name}/
        ├── stdout.log       # Standard output
        ├── stderr.log       # Standard error
        └── metadata.json    # Process metadata
```

When log rotation is triggered, archived files are created alongside the active log:

```
~/.runix/
└── apps/
    └── api/
        ├── stdout.log       # Active log
        ├── stdout.log.1     # Most recent archive
        ├── stdout.log.2     # Second archive
        ├── stdout.log.3
        ├── stdout.log.4
        ├── stdout.log.5     # Oldest archive (max 5)
        ├── stderr.log
        ├── stderr.log.1
        └── ...
```

## Log Format

Each line in the log file is prefixed with a timestamp and stream indicator:

```
2026-04-14 10:23:45 [out] Server listening on :8080
2026-04-14 10:23:46 [out] Connected to database
2026-04-14 10:24:01 [err] connection timeout: retrying in 5s
```

- `[out]` — stdout output
- `[err]` — stderr output
- Timestamp format: `YYYY-MM-DD HH:MM:SS`

Lines are buffered and written atomically when a newline is received. Partial lines (output without a trailing newline) are held in a buffer until completed.

## Log Rotation

Logs are rotated automatically based on size and age.

### Rotation Triggers

| Condition | Default         | Description                             |
| --------- | --------------- | --------------------------------------- |
| Size      | `10MB`          | Rotate when log file exceeds this size  |
| Age       | `168h` (7 days) | Rotate when log file is older than this |

Configure these values in `defaults` or per-process:

```yaml
defaults:
  log_max_size: "10MB"
  log_max_age: "168h"
```

### Rotation Process

1. Check if the active log file exceeds `log_max_size` or is older than `log_max_age`
2. Shift existing archives: `.4` → `.5`, `.3` → `.4`, `.2` → `.3`, `.1` → `.2`
3. Rename the active log to `.log.1`
4. Create a new empty log file
5. Maximum of 5 archived copies; the oldest (`.5`) is discarded

The maximum disk usage per process is approximately `log_max_size × 6` (one active log plus five archives).

## Reading Logs

### CLI

```bash
# Follow live output (default behavior, like tail -f)
runix logs api-server

# View last 200 lines as a snapshot (no follow)
runix logs api-server -n 200 --nostream

# View snapshot without following
runix logs api-server --nostream

# View stderr only
runix logs api-server --err

# View stdout only
runix logs api-server --out
```

### TUI

Launch the TUI and press `l` on a selected process to view its logs:

```bash
runix tui
```

### Web UI

Open the web dashboard and click "Logs" on any process:

```bash
runix web
```

### MCP

Query process logs through the MCP server:

```
Tool: get_logs
Parameters: { "target": "api-server", "lines": 50 }
```

Or via MCP resources:

```
URI: logs://{name}
MIME: text/plain
```

See the [MCP documentation](mcp.md) for details.

## Direct File Access

Log files are plain text and can be read with standard tools:

```bash
# Tail the log
tail -f ~/.runix/apps/api/stdout.log

# Search for errors
grep "\[err\]" ~/.runix/apps/api/stderr.log

# Count lines
wc -l ~/.runix/apps/api/stdout.log
```

## Configuration Reference

| Setting        | Location              | Default  | Description                                        |
| -------------- | --------------------- | -------- | -------------------------------------------------- |
| `log_max_size` | `defaults` or process | `10MB`   | Max log size before rotation                       |
| `log_max_age`  | `defaults` or process | `168h`   | Max log age before rotation                        |
| `log_level`    | `daemon`              | `"info"` | Daemon log level: `debug`, `info`, `warn`, `error` |

## Verbose Mode

Enable debug-level logging with the `--verbose` flag:

```bash
runix --verbose start app.py
```

This sets the daemon's zerolog output to debug level, providing detailed information about internal operations including IPC requests, state transitions, and hook execution.
