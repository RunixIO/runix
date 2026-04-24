# internal/runtime/ — Language Adapters

Seven runtime adapters that implement the `Runtime` interface for auto-detecting and launching processes.

## Interface

```go
type Runtime interface {
    Name() string                           // e.g. "go", "python"
    Detect(dir string) bool                 // true if project uses this runtime
    StartCmd(opts StartOptions) (*exec.Cmd, error)  // build exec.Cmd to run
}
```

## Implementations

| File | Runtime | Detection Files | Start Strategy |
|------|---------|-----------------|----------------|
| `go.go` | Go | `go.mod` | `go run .` / `go run file.go` / binary path |
| `python.go` | Python | `requirements.txt`, `pyproject.toml`, `setup.py`, `Pipfile`, `*.py` | Resolves venv/`.venv` first, then `python3`/`python` |
| `node.go` | Node | `package.json` | Handles TypeScript via `npx tsx` |
| `bun.go` | Bun | `bun.lockb`, `bunfig.toml`, `bun.lock` | `bun run` |
| `deno.go` | Deno | `deno.json`, `deno.jsonc` | `deno run [perms] <entrypoint>` |
| `ruby.go` | Ruby | `Gemfile`, `Gemfile.lock` | `ruby <entrypoint>` / `bundle exec ruby <entrypoint>` (`UseBundle`) |
| `php.go` | PHP | `composer.json`, `artisan`, `*.php` | `php <entrypoint> [args...]` |

## Detection Order

`detector.go`: iterates runtimes in priority order: Go → Python → Node.js → Bun → Deno → Ruby → PHP. First match wins. `--runtime` flag bypasses detection entirely.

## Env Merging

`runtime.go:buildEnv()`: merges `os.Environ()` with overlay map. Overlay keys replace existing; new keys appended. Used by all adapters via `StartOptions.Env`.
