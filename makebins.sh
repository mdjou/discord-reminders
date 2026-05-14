#!/bin/bash
set -euo pipefail

mkdir -p release
go build -ldflags "-w -s" -o release/bot ./cmd/bot/
go build -ldflags "-w -s" -o release/register ./cmd/register/

