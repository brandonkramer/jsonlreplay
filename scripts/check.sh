#!/usr/bin/env bash
# Run the same checks as CI before pushing.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> go test -race -cover ./..."
go test -race -cover ./...

echo "==> examples build"
go build -o /dev/null ./examples/...

echo "==> linux compile check"
GOOS=linux GOARCH=amd64 go test -c -o /dev/null ./...

echo "==> golangci-lint"
./scripts/lint.sh

echo "check: ok"
