#!/usr/bin/env bash
# Run golangci-lint; installs a Go 1.26-compatible binary when missing.
set -euo pipefail

cd "$(dirname "$0")/.."

GOLANGCI_LINT_VERSION="${GOLANGCI_LINT_VERSION:-v2.12.2}"

lint_ready() {
	command -v golangci-lint >/dev/null || return 1
	golangci-lint version 2>&1 | grep -Eq 'go1\.(2[6-9]|[3-9][0-9])'
}

if ! lint_ready; then
	echo "==> installing golangci-lint ${GOLANGCI_LINT_VERSION} with $(go version)"
	go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}"
	export PATH="$(go env GOPATH)/bin:${PATH}"
fi

if [ "$#" -gt 0 ]; then
	exec golangci-lint run "$@"
fi

exec golangci-lint run ./...
