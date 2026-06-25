#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PS1="$ROOT/scripts/install-windows.ps1"

test -f "$PS1"

rg -q 'RuntimeInformation\]::OSArchitecture' "$PS1"
rg -q 'oapi-windows-\$distArch\.exe' "$PS1"
rg -q 'Unsupported Windows architecture' "$PS1"
rg -q 'Prebuilt binary not found' "$PS1"
rg -q 'Copy-Item -LiteralPath \$prebuiltExe -Destination \$CliDest -Force' "$PS1"

if rg -q 'go build|Get-Command go' "$PS1"; then
  echo "installer should not require local go build anymore" >&2
  exit 1
fi

echo "ok: windows installer uses prebuilt arch-specific binaries"
