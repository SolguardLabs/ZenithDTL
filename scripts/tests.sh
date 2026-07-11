#!/usr/bin/env bash
set -euo pipefail

node scripts/build.mjs
go test ./...
node --experimental-strip-types --test tests/node/*.test.ts
