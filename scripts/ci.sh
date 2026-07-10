#!/usr/bin/env bash
set -euo pipefail

unformatted="$(gofmt -l src cmd)"
if [ -n "$unformatted" ]; then
  echo "$unformatted"
  exit 1
fi
node scripts/build.mjs
go test ./...
go vet ./...
node --experimental-strip-types --test "tests/node/*.test.ts"
