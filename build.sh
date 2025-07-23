#!/usr/bin/env bash

set -euo pipefail

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./build/reloader ./main.go
