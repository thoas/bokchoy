#!/bin/bash -e

export GO111MODULE=on

time go test -cover -coverprofile=coverage.out  -v -p 1

exit $?
