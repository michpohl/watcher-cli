# watcher-cli

Directory watcher with per-folder actions, built in Go.

## Features
- Poll-based watching of multiple folders with per-folder scan intervals.
- Per-folder debounce to suppress rapid repeat events.
- Multiple actions per folder; each action has its own filters (include/exclude), event types, size/age constraints, hidden ignore, and overwrite policy.
- Core actions: exec, copy, move/rename, webhook.
- Template tokens available in commands/destinations: `{path}`, `{relpath}`, `{dir}`, `{name}`, `{stem}`, `{ext}`, `{event}`, `{size}`, `{mtime}`, `{age_ms}`, `{age_days}`.
- Dry-run flag from config to log decisions without executing actions.
- Simulation CLI to test matching/actions without touching the filesystem.

## Getting Started
Requires Go 1.21+. After installing Go, run:
```sh
go mod tidy
go fmt ./...
go build ./...
```

Initialize a sample config:
```sh
./watcher init
```
This writes `watcher.yaml` in the current directory. Adjust paths and actions to your needs.

Run the watcher:
```sh
./watcher run --config watcher.yaml
```

Validate config without running:
```sh
./watcher validate --config watcher.yaml
```

Simulate an event (dry-run by default):
```sh
./watcher simulate --config watcher.yaml --file /path/to/file.jpg --event create
```
Use `--execute` to actually run matching actions.

## Configuration Notes
- `global.scan_interval_ms`: default polling interval (ms), overridable per watch.
- `global.debounce_ms` and `watches[*].debounce_ms`: suppress rapid duplicate events (ms).
- `watches[*].actions[*].events`: any of `create`, `modify`, `delete`, `move`.
- `overwrite`: if omitted, uses global default; can be true/false per action.
- `ignore_hidden`: defaults to true per action.

See `cmd/watcher/main.go` for the embedded sample config or create your own based on `PLAN.md`.

## Status
- Implements core pipeline (config, scanner, matcher, actions, supervisor, CLI).
- Go toolchain not available in this environment, so go.sum and compiled binary are not present. Run `go mod tidy` locally to fetch deps and create go.sum.
