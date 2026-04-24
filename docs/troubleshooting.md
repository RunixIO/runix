# Troubleshooting

Common issues, debugging steps, and error explanations for Runix.

## Diagnostics

### `runix doctor`

Run the built-in diagnostic tool first. It checks:

- Available runtimes (Go, Python, Node.js, Bun, Deno, Ruby, PHP)
- Data and runtime directories
- Daemon status and socket accessibility
- File permissions

```bash
runix doctor
```

Address any issues reported before investigating further.

### Verbose Mode

Enable debug output with the `--verbose` flag to see internal operations:

```bash
runix --verbose start app.py
```

This shows IPC requests, state transitions, hook execution, and runtime detection details.

## Common Issues

### Process won't start

**Symptom:** `runix start app.py` reports failure or the process immediately enters `crashed` or `errored` state.

**Debugging steps:**

1. Check the process logs:

   ```bash
   runix logs app.py
   runix logs app.py --err
   ```

2. Verify the entrypoint exists and is executable:

   ```bash
   ls -la app.py
   ```

3. Check runtime detection:

   ```bash
   runix --verbose start app.py
   ```

   Look for the "detected runtime" log line.

4. Try running the command directly:

   ```bash
   python3 app.py
   ```

   If this fails, the issue is with the application, not Runix.

5. Check for missing dependencies (e.g., Python venv, node_modules):

   ```bash
   ls venv/bin/python   # Python
   ls node_modules      # Node.js
   ```

6. Verify environment variables:
   ```bash
   runix inspect app.py
   ```

### Daemon not responding

**Symptom:** Commands hang or report that the daemon is unreachable.

**Debugging steps:**

1. Check daemon status:

   ```bash
   runix daemon status
   ```

2. Check if the socket file exists:

   ```bash
   ls -la ~/.runix/tmp/runix.sock
   ```

3. Check if the PID file matches a running process:

   ```bash
   cat ~/.runix/tmp/runix.pid
   ps -p $(cat ~/.runix/tmp/runix.pid)
   ```

4. Restart the daemon:

   ```bash
   runix daemon restart
   ```

5. If restart fails, stop and start fresh:

   ```bash
   runix daemon stop
   runix daemon start
   ```

6. Check for stale socket files (daemon crashed without cleanup):
   ```bash
   rm ~/.runix/tmp/runix.sock
   runix daemon start
   ```

### Process keeps restarting (crash loop)

**Symptom:** Process state oscillates between `running` and `crashed`, eventually reaches `errored`.

**Debugging steps:**

1. Check stderr for error messages:

   ```bash
   runix logs worker --err -n 100
   ```

2. Review restart count and policy:

   ```bash
   runix inspect worker
   ```

   Check `restarts`, `max_restarts`, and `restart_policy`.

3. The process is likely crashing due to an application error. Fix the root cause.

4. Temporarily prevent auto-restart:
   ```yaml
   restart_policy: "never"
   ```
   Or reduce max restarts:
   ```yaml
   max_restarts: 1
   ```

### Runtime detected incorrectly

**Symptom:** Runix uses the wrong runtime or interpreter.

**Debugging steps:**

1. Specify the runtime explicitly:

   ```bash
   runix start app.py --runtime python
   ```

2. Check what runtime is detected:

   ```bash
   runix --verbose start app.py
   ```

3. Detection priority is: Go → Python → Node.js → Bun → Deno → Ruby → PHP. If multiple markers exist (e.g., both `go.mod` and `package.json`), the first match wins. Use `--runtime` to override.

### Watch not triggering restarts

**Symptom:** File changes are not detected or process is not restarted.

**Debugging steps:**

1. Verify watch is enabled:

   ```bash
   runix inspect api-server
   ```

   Check the watch configuration section.

2. The changed file might be in an ignored directory. Check ignore patterns:
   - Default ignores: `.git`, `node_modules`, `__pycache__`, `*.pyc`, `.DS_Store`, `vendor`, `dist`, `build`, `bin`

3. The debounce window might be too high. Try reducing it:

   ```bash
   runix watch api-server --debounce 50ms
   ```

4. Verify the watched paths include your source files:
   ```bash
   runix watch api-server --paths ./src --paths ./internal
   ```

### Logs not appearing

**Symptom:** `runix logs` shows no output or stale data.

**Debugging steps:**

1. Verify the process is running:

   ```bash
   runix list
   ```

2. Check if log files exist:

   ```bash
   ls -la ~/.runix/apps/{process-name}/
   ```

3. The process might be buffering output. Many runtimes buffer stdout when not connected to a terminal. Set the appropriate unbuffered flag:
   - Python: `PYTHONUNBUFFERED=1`
   - Node.js: `NODE_OPTIONS=--no-buffering`
   - Go: output is unbuffered by default

   ```yaml
   env:
     PYTHONUNBUFFERED: "1"
   ```

4. Check for log rotation — old logs may have been archived:
   ```bash
   ls -la ~/.runix/apps/{process-name}/stdout.log.*
   ```

### Permission denied on socket

**Symptom:** `permission denied` when connecting to the daemon socket.

**Debugging steps:**

1. Check socket permissions:

   ```bash
   ls -la ~/.runix/tmp/runix.sock
   ```

   The socket should be `0o660` (read/write for owner and group).

2. Ensure you are the same user that started the daemon:

   ```bash
   runix daemon status
   ```

3. Do not start the daemon with `sudo` unless you intend to manage it as root.

### Self-update fails

**Symptom:** `runix update` reports an error.

**Debugging steps:**

1. Check if you have write permission to the binary:

   ```bash
   which runix
   ls -la $(which runix)
   ```

2. If installed system-wide, you may need elevated privileges:

   ```bash
   sudo runix update
   ```

3. Check network connectivity to GitHub:

   ```bash
   curl -sf https://api.github.com/repos/runixio/runix/releases/latest | head -5
   ```

4. Verify checksum integrity. If the download was corrupted, retry:
   ```bash
   runix update --version v0.2.0
   ```

### Save/resurrect not restoring processes

**Symptom:** `runix resurrect` does not start saved processes.

**Debugging steps:**

1. Verify the dump file exists:

   ```bash
   ls -la ~/.runix/dump.json
   ```

2. Check its contents:

   ```bash
   cat ~/.runix/dump.json | python3 -m json.tool
   ```

3. Ensure entrypoints are still valid. If files were moved or deleted since `save`, resurrect will fail.

4. Check permissions on the dump file and data directory.

## Error States

| State     | Meaning                                  | Resolution                                     |
| --------- | ---------------------------------------- | ---------------------------------------------- |
| `stopped` | Process was stopped normally             | Start it with `runix start` or `runix restart` |
| `crashed` | Process exited with a non-zero code      | Check stderr logs for the error                |
| `errored` | Exceeded max restarts or hook failure    | Fix the root cause, then restart               |
| `waiting` | Waiting for backoff delay before restart | Normal transient state during auto-recovery    |

## Getting Help

If the troubleshooting steps above don't resolve your issue:

1. Check the [GitHub Issues](https://github.com/runixio/runix/issues) for known problems
2. Run `runix doctor` and include the output in your report
3. Include the Runix version (`runix version --verbose`) and OS information
4. Include relevant log output (`runix logs --err`)
