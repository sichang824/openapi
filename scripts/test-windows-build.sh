#!/usr/bin/env bash
# Verifies cross-compilation targets produce Windows PE executables.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

make build-cli-windows
make build-cli-windows-arm64

AMD64="$ROOT/bin/oapi-windows-amd64.exe"
ARM64="$ROOT/bin/oapi-windows-arm64.exe"

test -f "$AMD64"
test -f "$ARM64"
test -s "$AMD64"
test -s "$ARM64"

# PE signature "MZ" at offset 0 (Windows executables)
head -c 2 "$AMD64" | cmp -s - <(printf '\x4d\x5a') || {
  echo "expected PE/MZ header in $AMD64" >&2
  exit 1
}
head -c 2 "$ARM64" | cmp -s - <(printf '\x4d\x5a') || {
  echo "expected PE/MZ header in $ARM64" >&2
  exit 1
}

echo "ok: windows cross-build artifacts look valid"
