#!/bin/bash -e

export GO111MODULE=on

time go test -v -p 1

exit $?
