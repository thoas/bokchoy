#!/bin/bash -e

export GO111MODULE=on

time go test -cover -v -p 1

exit $?
