#!/bin/bash

export GO111MODULE=on

set -eo pipefail

linter_path="${GOPATH}/bin/golangci-lint"

if [[ ! -x "${linter_path}" ]]; then
    go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
fi

SOURCE_DIRECTORY=$(dirname "${BASH_SOURCE[0]}")
cd "${SOURCE_DIRECTORY}/.."

if [[ -n $1 ]]; then
    golangci-lint run "$1"
else
    golangci-lint run ./...
fi

