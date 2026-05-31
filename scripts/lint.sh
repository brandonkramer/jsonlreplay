#!/usr/bin/env bash
# Run pinned golangci-lint from go.mod (go tool).
set -euo pipefail

cd "$(dirname "$0")/.."

if [ "$#" -gt 0 ]; then
	exec go tool golangci-lint run "$@"
fi

exec go tool golangci-lint run ./...
