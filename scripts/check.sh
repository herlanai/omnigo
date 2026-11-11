#!/bin/sh
set -eu
cd "$(dirname "$0")/.."

go test ./...
go vet ./...
go test -race ./...
node --test omniweb/client/*.test.ts

for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  target_os=${target%/*}
  target_arch=${target#*/}
  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build ./...
done

# Cross-compilation checks core portability, not working desktop drivers.
go run ./examples/lifecycle
