# Namespaces and Instances

Runix supports logical grouping of processes through namespaces and running multiple copies of a process through instances.

## Namespaces

Namespaces provide a way to logically group related processes. They act as tags that help organize and filter processes in multi-service environments.

### Assigning a Namespace

**Via configuration:**

```yaml
processes:
  - name: "api"
    entrypoint: "./cmd/api"
    namespace: "backend"

  - name: "worker"
    entrypoint: "worker.py"
    namespace: "backend"

  - name: "frontend"
    entrypoint: "dev"
    runtime: "bun"
    namespace: "frontend"
```

**Via CLI:**

```bash
runix start app.py --namespace backend
```

### Naming Conventions

- Namespaces are free-form strings with no enforced format
- Use short, descriptive names that reflect the logical grouping (e.g., `backend`, `frontend`, `data-pipeline`, `monitoring`)
- A process without a namespace has an empty string as its namespace

### Current Behavior

Namespaces are stored as part of the process configuration and are visible in:

- `runix list` output
- `runix status` output
- `runix inspect` output
- MCP tool responses
- Saved state (`dump.json`)

Namespace-based filtering is available via `runix list --namespace backend`.

### Organizing with Labels

For additional organization beyond namespaces, processes support key-value labels:

```yaml
processes:
  - name: "api"
    entrypoint: "./cmd/api"
    namespace: "backend"
    labels:
      team: "platform"
      tier: "critical"
      env: "production"
```

Labels are stored in the process configuration and persisted across save/resurrect cycles.

## Instances

The `instances` field allows you to specify how many copies of a process to run. This is useful for scaling workers or running multiple replicas of a service.

### Configuring Instances

**Via configuration:**

```yaml
processes:
  - name: "worker"
    entrypoint: "worker.py"
    instances: 4
```

**Via CLI:**

```bash
runix start worker.py --instances 4
```

### Current Behavior

When `instances` is set to a value greater than 1, Runix creates multiple `ManagedProcess` entries, each with a unique ID and a suffixed name (e.g., `worker:0`, `worker:1`, `worker:2`, `worker:3`). Each instance shares the same configuration but runs as an independent process with its own PID, logs, and restart state.

Instances:

- Share the same configuration (entrypoint, runtime, env, hooks, etc.)
- Have unique IDs and suffixed names (e.g., `worker:0`, `worker:1`)
- Maintain independent lifecycle states and restart counters
- Write to separate log files under their unique process ID
- Are individually addressable by their suffixed name or ID

## Recommended Organization Patterns

### By Service Tier

```yaml
processes:
  - name: "api"
    namespace: "core"
    labels:
      tier: "critical"
    restart_policy: "always"
    max_restarts: 10

  - name: "background-worker"
    namespace: "core"
    labels:
      tier: "normal"
    restart_policy: "on-failure"

  - name: "metrics-exporter"
    namespace: "monitoring"
    labels:
      tier: "low"
    restart_policy: "on-failure"
    max_restarts: 3
```

### By Environment

```yaml
processes:
  - name: "dev-api"
    namespace: "development"
    watch:
      enabled: true
    restart_policy: "always"

  - name: "dev-worker"
    namespace: "development"
    restart_policy: "on-failure"
```

### Combined with Labels

```yaml
processes:
  - name: "ingestion-api"
    namespace: "data-pipeline"
    labels:
      component: "ingestion"
      team: "data"
      version: "v2"

  - name: "processing-worker"
    namespace: "data-pipeline"
    labels:
      component: "processing"
      team: "data"
      version: "v2"
    instances: 4
```
