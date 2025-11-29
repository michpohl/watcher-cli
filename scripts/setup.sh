#!/usr/bin/env bash
set -euo pipefail

if ! command -v go >/dev/null 2>&1; then
  echo "Go is required (install Go 1.21+ and ensure it is on PATH)" >&2
  exit 1
fi

echo "[setup] go version: $(go version)"
echo "[setup] tidy modules"
go mod tidy

echo "[setup] run tests"
go test ./...

echo "[setup] build watcher binary"
go build -o watcher ./cmd/watcher

echo "[setup] done. Binary at ./watcher"
