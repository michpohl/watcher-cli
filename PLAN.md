## Directory Watcher Plan

### Scope and Goals
- Watch multiple folders (polling), each with its own scan interval (falls back to global) and recursion flag.
- Detect create/modify/delete/move events; per-folder debounce/backoff and optional batching.
- Configure 1..N actions per folder; each action has its own filters and options.
- Keep a concise core action set: exec, copy, move/rename, webhook. No plugin system needed now.
- Provide safety (retries, timeouts, dry-run, overwrite policy) and visibility (logs, status).

### Configuration Model (YAML sketch)
```yaml
global:
  scan_interval_ms: 1000
  debounce_ms: 200
  dry_run: false
  defaults:
    overwrite: false

watches:
  - path: /data/incoming
    recursive: true
    scan_interval_ms: 500  # overrides global
    stop_on_first_match: false
    actions:
      - name: images
        include: ["**/*.jpg", "**/*.png"]
        events: [create, modify]
        type: exec
        cmd: "python process_image.py {path}"
        retries: 3
        timeout_ms: 10000
      - name: pdf_backup
        include: ["**/*.pdf"]
        exclude: ["**/tmp/**"]
        events: [create]
        type: copy
        dest: "/backup/docs/{relpath}"
        overwrite: true
      - name: archive_old
        include: ["**/*"]
        events: [modify]
        if: "{age_days} > 30"
        type: move
        dest: "{dir}/archive/{name}"
```

### Filters and Matching
- Per-action include/exclude globs or regex; file vs directory; min/max size; age constraints; ignore hidden/temp.
- Actions can be ordered; choose run-all-matching vs stop-on-first-match per folder.
- Template tokens available in commands/paths: `{path}`, `{dir}`, `{name}`, `{stem}`, `{ext}`, `{relpath}`, `{event}`, `{size}`, `{mtime}`, `{age_days}`.

### Actions (core set)
- `exec`: shell command with templating; per-action `env`, `cwd`.
- `copy` / `move` / `rename`: destination templated; create dirs; overwrite/skip policy.
- `webhook`: POST event payload to a URL.

### Reliability and Safety
- Per-action retries with backoff; timeouts; optional state to avoid reprocessing.
- Debounce to coalesce rapid changes; optional batching within a scan.
- Dry-run mode to show decisions without executing.
- Audit log of actions run and their outcomes.

### Observability and CLI
- Structured logs; `watcher status` to show folders, intervals, last scan; `watcher tail` to stream events.
- `watcher init` to create sample config; `watcher validate` to lint config and filters; `watcher run` to start polling (with `--watch path` to limit).

### Open Decisions
- Defaults for overwrite and stop_on_first_match.
- Whether polling-only is sufficient or if event-based backends are desired later.
- Finalizing the core action set (exec/copy/move/rename/webhook).
