# watcher-cli

Directory watcher with per-folder actions, built in Go.

## What it does
- Poll-based watching of multiple folders with per-folder scan intervals and debounce.
- Multiple actions per folder; each action has its own filters (include/exclude globs), event types, size/age constraints, hidden ignore, and overwrite policy.
- Core actions: `exec`, `copy`, `move/rename`, `webhook`.
- Template tokens you can use in commands/destinations: `{path}`, `{relpath}`, `{dir}`, `{name}`, `{stem}`, `{ext}`, `{event}`, `{size}`, `{mtime}`, `{age_ms}`, `{age_days}`.
- Dry-run and simulate modes to verify behavior without making changes.

## Prerequisites
- Go 1.21+ installed (e.g., `brew install go` on macOS) and available on your `PATH`.

## Quick start
```sh
git clone <repo-url> watcher-cli
cd watcher-cli
./scripts/setup.sh          # runs tidy, tests, builds ./watcher
./watcher init              # optional: writes watcher.yaml template
./watcher run --config watcher.sample.yaml
```
During testing, `watcher.sample.yaml` runs in `dry_run: true` to avoid side effects. It watches `./incoming` and logs matches for jpg/png/pdf files and an archive move action.

## Installation options
- Local build (already done by setup): `./watcher run ...`
- Install to Go bin: `go install ./cmd/watcher` (binary goes to `$GOBIN` or `~/go/bin`).
- Manual install: `go build -o watcher ./cmd/watcher && install -m 755 watcher /usr/local/bin` (or `~/.local/bin`).

## Configuration basics (YAML)
- Durations ending in `_ms` accept integers in milliseconds or duration strings (`"200ms"`, `"1s"`, `"2m"`).
- Events: `create`, `modify`, `delete`, `move`.
- Include/exclude globs use doublestar (`**` supported). Use both `*.ext` and `**/*.ext` if you want top-level and nested matches.
- `dry_run: true` logs actions instead of executing.
- `overwrite`: defaults from `global.defaults.overwrite`, can be overridden per action.
- `ignore_hidden`: defaults to true if not set.

### Sample config (shipped as watcher.sample.yaml)
```yaml
global:
  scan_interval_ms: 1000
  debounce_ms: 200
  dry_run: true
  defaults:
    overwrite: false

watches:
  - path: ./incoming
    recursive: true
    scan_interval_ms: 500
    debounce_ms: 200
    stop_on_first_match: false
    actions:
      - name: images
        include: ["**/*.jpg", "**/*.png"]
        events: [create, modify]
        type: exec
        cmd: "echo processing {path}"
        retries: 3
        timeout_ms: 10000
      - name: pdf_backup
        include: ["**/*.pdf"]
        exclude: ["**/tmp/**"]
        events: [create]
        type: copy
        dest: "./backup/docs/{relpath}"
        overwrite: true
      - name: archive_old
        include: ["**/*"]
        events: [modify]
        type: move
        dest: "{dir}/archive/{name}"
```

## Usage
- Run: `./watcher run --config watcher.yaml`
- Validate config: `./watcher validate --config watcher.yaml`
- Simulate (dry-run by default): `./watcher simulate --config watcher.yaml --file /path/to/file.jpg --event create`
  - Add `--execute` to actually run matching actions.

## Testing and development
- Unit tests: `go test ./...`
- Format: `go fmt ./...`
- Update deps: `go mod tidy`

## Tips
- Start with `dry_run: true` until actions look correct.
- Point copy/move destinations to writable paths you own.
- Use `stop_on_first_match` per watch if only one action should run per event.
