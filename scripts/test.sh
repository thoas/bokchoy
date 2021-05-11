#!/bin/bash -e

export GO111MODULE=on
export REDIS_PORT=${REDIS_PORT:-"6379"}
export REDIS_HOST=${REDIS_HOST:-"localhost"}


time go test -cover -coverprofile=coverage.out  -v -p 1

exit $?
