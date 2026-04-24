# MCP Integration

Runix provides a Model Context Protocol (MCP) server that allows AI agents to manage processes programmatically. The MCP server exposes tools for process management and resources for reading process state and logs.

## Starting the MCP Server

### Stdio Transport (Default)

The stdio transport is designed for AI agent integration. The MCP server reads from stdin and writes to stdout.

```bash
runix mcp
```

### HTTP Transport

The HTTP transport enables remote access and web-based integrations.

```bash
runix mcp --transport http --listen localhost:8090
```

### Via Daemon Configuration

Enable MCP alongside the daemon in `runix.yaml`:

```yaml
mcp:
  enabled: true
  transport: "http"
  listen: "localhost:8090"
```

When enabled, the MCP HTTP server starts alongside the daemon automatically.

## AI Tool Integration

### Claude Code / Claude Desktop

Add Runix to your MCP server configuration:

```json
{
  "mcpServers": {
    "runix": {
      "command": "runix",
      "args": ["mcp"]
    }
  }
}
```

For HTTP transport:

```json
{
  "mcpServers": {
    "runix": {
      "url": "http://localhost:8090/mcp"
    }
  }
}
```

### Cursor / Other MCP Clients

Use the same configuration format appropriate for your client. The stdio transport (`runix mcp`) is the most compatible option.

## Tools

### `list_apps`

List all managed processes with their current status.

**Parameters:** None

**Response:**

```json
{
  "processes": [
    {
      "id": "abc123",
      "name": "api",
      "state": "running",
      "pid": 12345,
      "runtime": "go",
      "restarts": 0,
      "uptime": "2h30m",
      "cpu_percent": 2.5,
      "memory_mb": 45.2,
      "threads": 12,
      "fds": 8
    }
  ]
}
```

### `start_app`

Start a new managed process.

**Parameters:**

| Parameter        | Type   | Required | Description                                    |
| ---------------- | ------ | -------- | ---------------------------------------------- |
| `entrypoint`     | string | Yes      | File or command to run                         |
| `name`           | string | No       | Process name (defaults to entrypoint filename) |
| `runtime`        | string | No       | Runtime: `go`, `python`, `node`, `bun`, `auto` |
| `cwd`            | string | No       | Working directory                              |
| `restart_policy` | string | No       | `always`, `on-failure`, `never`                |
| `env`            | object | No       | Environment variables                          |

**Example:**

```json
{
  "entrypoint": "./cmd/api",
  "name": "api",
  "runtime": "go",
  "env": {
    "PORT": "8080"
  }
}
```

### `stop_app`

Stop a managed process.

**Parameters:**

| Parameter | Type   | Required | Description                              |
| --------- | ------ | -------- | ---------------------------------------- |
| `target`  | string | Yes      | Process ID or name                       |
| `force`   | bool   | No       | Force stop with SIGKILL (default: false) |

### `restart_app`

Restart a managed process.

**Parameters:**

| Parameter | Type   | Required | Description        |
| --------- | ------ | -------- | ------------------ |
| `target`  | string | Yes      | Process ID or name |

### `reload_app`

Graceful reload of a process (stop + start without backoff).

**Parameters:**

| Parameter | Type   | Required | Description        |
| --------- | ------ | -------- | ------------------ |
| `target`  | string | Yes      | Process ID or name |

### `delete_app`

Stop and remove a process from the process table.

**Parameters:**

| Parameter | Type   | Required | Description        |
| --------- | ------ | -------- | ------------------ |
| `target`  | string | Yes      | Process ID or name |

### `get_status`

Get detailed status for a specific process.

**Parameters:**

| Parameter | Type   | Required | Description        |
| --------- | ------ | -------- | ------------------ |
| `target`  | string | Yes      | Process ID or name |

**Response includes:** ID, name, state, PID, runtime, restart count, uptime, config, resource metrics, timestamps.

### `get_logs`

Retrieve recent log output for a process.

**Parameters:**

| Parameter | Type   | Required | Description                             |
| --------- | ------ | -------- | --------------------------------------- |
| `target`  | string | Yes      | Process ID or name                      |
| `lines`   | number | No       | Number of lines to return (default: 50) |

### `save_state`

Persist the current process table to disk. Processes can be restored with `resurrect_processes`.

**Parameters:** None

### `resurrect_processes`

Restore previously saved processes from disk.

**Parameters:** None

## Resources

MCP resources provide read-only access to process state and data.

### `apps://list`

All managed processes with their current state.

- **MIME type:** `application/json`
- **Content:** Same format as `list_apps` response

### `apps://{name}`

Status of a single process by name.

- **MIME type:** `application/json`
- **Parameters:** Replace `{name}` with the process name
- **Content:** Process details including state, PID, runtime, metrics, config

### `logs://{name}`

Process log output.

- **MIME type:** `text/plain`
- **Parameters:** Replace `{name}` with the process name
- **Content:** Recent log lines with timestamps

### `metrics://{name}`

Resource metrics for a process.

- **MIME type:** `application/json`
- **Parameters:** Replace `{name}` with the process name
- **Content:** CPU percentage, memory (MB), thread count, file descriptor count

## Example Workflows

### AI Agent: Deploy and Monitor

```
1. start_app({ entrypoint: "./cmd/api", name: "api", runtime: "go" })
2. get_status({ target: "api" })  → verify "running" state
3. get_logs({ target: "api", lines: 20 })  → check startup output
```

### AI Agent: Restart Unhealthy Process

```
1. list_apps()  → find processes with state "crashed" or "errored"
2. restart_app({ target: "worker" })
3. get_logs({ target: "worker", lines: 10 })  → verify recovery
```

### AI Agent: Save State Before Maintenance

```
1. list_apps()  → review running processes
2. save_state()  → persist to disk
3. (perform maintenance)
4. resurrect_processes()  → restore all processes
```

## Transport Comparison

| Feature        | stdio                | http                         |
| -------------- | -------------------- | ---------------------------- |
| Use case       | AI agent integration | Remote access, web UIs       |
| Startup        | `runix mcp`          | `runix mcp --transport http` |
| Access         | Local only           | Network-accessible           |
| Authentication | None                 | None                         |
| Default port   | N/A                  | `localhost:8090`             |

The stdio transport is recommended for AI agent integrations. The HTTP transport is useful for custom dashboards or remote monitoring.
