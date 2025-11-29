## Implementation Plan (Go)

A detailed, sequential plan to build the watcher CLI.

### 1) Project Scaffolding
- Initialize Go module: `go mod init watcher-cli`.
- Add CLI dependency (e.g., cobra or urfave/cli) and YAML parser (`gopkg.in/yaml.v3`), globbing (`github.com/bmatcuk/doublestar/v4`).
- Create directories: `cmd/watcher`, `internal/config`, `internal/watcher`, `internal/scanner`, `internal/match`, `internal/template`, `internal/actions`, `internal/logging`, `internal/status`.

### 2) Config Layer
- Define types: `Config`, `Global`, `Watch`, `Action`, `Defaults`, `Condition`.
- Implement `Load(path string) (Config, error)` to read YAML, expand env vars, apply defaults (global then per-watch), and validate.
- Validation rules: required `watches`, valid paths, intervals > 0, known action types, filters compiled, events valid, no duplicate action names per watch.
- Add a helper to compile include/exclude globs/regex and cache them in the config for faster matching.
- Add `Validate()` used by CLI `validate` command and by `run`.

### 3) Template Engine
- Implement a small replacer for tokens: `{path}`, `{dir}`, `{name}`, `{stem}`, `{ext}`, `{relpath}`, `{event}`, `{size}`, `{mtime}`, `{age_days}`.
- Ensure escaping or literal braces handling; unit tests for edge cases.

### 4) Scanner (Polling + Diff)
- Define `FileInfo` (size, mtime, mode, isDir, hashHint).
- Implement `Scan(root, recursive, ignoreHidden) (Snapshot, error)` that walks the tree and returns a map[path]FileInfo with relpath support.
- Implement `Diff(prev, curr) []Event` to produce create/modify/delete/move events; move detection via hashHint or size+mtime heuristic.
- Add debounce/batch support hooks (configurable) to coalesce rapid successive events.
- Unit tests: create/modify/delete/move detection using temp dirs.

### 5) Matcher
- `Matcher.Match(ev Event, actions []Action) []Action` applying:
  - include/exclude globs/regex
  - event types
  - size/age min/max
  - file vs dir flag; hidden ignore
- Respect `stop_on_first_match` per watch.
- Tests covering matching combinations.

### 6) Actions and Registry
- Define `Runner` interface: `Run(ctx, ev, cfg) error`.
- Build registry mapping `ActionType` → runner.
- Implement runners:
  - `exec`: build command with templated args, apply env/cwd, honor timeout.
  - `copy` / `move` / `rename`: ensure dest dirs, overwrite policy, use template for dest.
  - `webhook`: POST JSON payload with event fields.
- Implement `Executor` wrapper to handle retries/backoff, timeouts, dry-run short-circuit, logging of outcome.
- Tests with fakes: exec runner using a fake command hook; filesystem ops in temp dirs; webhook using httptest server.

### 7) Watcher Supervisor and Workers
- `Supervisor` starts one `Worker` per watch with its scan interval (fallback to global).
- Worker loop:
  - ticker based on scan interval.
  - call `scanner.Scan`, `Diff` against prior snapshot.
  - optional debounce/batch.
  - for each event, call `Matcher`, then `Executor` for selected actions.
  - update status/metrics and log audit entries.
- Support context cancellation for graceful shutdown.
- Tests: integration with temp dir and short intervals to assert actions fired.

### 8) Status/Observability
- Structured logging setup (std `slog` or zap).
- `status.Tracker`: per-watch/action counters (events seen, actions run, successes/failures), last scan time, last error.
- `watcher status` command reads tracker (in-process or via shared state) and prints summary.
- `watcher tail` streams log file or stdout with filtering.

### 9) CLI Commands
- `run`: load config, validate, start supervisor; handle signals for shutdown.
- `validate`: load/validate config; print issues.
- `init`: write sample config file to path.
- `simulate`: feed synthetic events into matcher/executor for debugging.
- `status`: display status snapshot from tracker.
- Wire flags: `--config`, `--watch path`, `--dry-run`, log level, etc.

### 10) Tooling and Quality
- Add `Makefile` targets: `lint`, `test`, `build`, `fmt`.
- Go fmt/vet integration; consider `golangci-lint` if available.
- CI plan (GitHub Actions) to run tests and lint (optional for later).

### 11) Distribution (later)
- Build single static binary for target platforms; embed version info via ldflags.
- Package sample config in release assets.

### 12) Documentation
- Update README with usage, config reference, examples.
- Document tokens, action types, safety notes, and troubleshooting.

### 13) Stretch (optional later)
- Persistent state to avoid reprocessing after restart.
- More action types (S3 upload, MQ).
- Event-based FS backends as an option to polling.
