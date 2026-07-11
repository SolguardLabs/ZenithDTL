#!/usr/bin/env bash
set -euo pipefail

format_dirs=()
for dir in src cmd; do
  if [ -d "$dir" ]; then
    format_dirs+=("$dir")
  fi
done

if [ "${#format_dirs[@]}" -gt 0 ]; then
  unformatted="$(gofmt -l "${format_dirs[@]}")"
  if [ -n "$unformatted" ]; then
    echo "$unformatted"
    exit 1
  fi
fi
node scripts/build.mjs
go test ./...
go vet ./...
node --experimental-strip-types --test tests/node/*.test.ts
