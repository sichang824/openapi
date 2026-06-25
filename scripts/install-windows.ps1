# Mirrors `make install TARGET=agents` for skill payload + user CLI path:
# - Skill: %USERPROFILE%\.agents\skills\<repo-folder-name>\  (assets, scripts, references?, bin, SKILL.md)
# - CLI:   %USERPROFILE%\.local\bin\oapi.exe (copy of prebuilt bin\oapi-windows-<arch>.exe)
# Repo root is inferred from this script location (no need to cd first).
#
# If the window "flashes and closes", you double-clicked the script: use install-windows.cmd instead,
# or run from an already-open PowerShell: powershell -ExecutionPolicy Bypass -File .\scripts\install-windows.ps1
# Optional: -PauseAtEnd keeps the window open when run from Explorer.
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File .\scripts\install-windows.ps1 [-PauseAtEnd] [-SkipUserPath]
# -SkipUserPath: do not add %USERPROFILE%\.local\bin to the user PATH (you must add it yourself to run `oapi` without a full path).

param(
    [switch]$PauseAtEnd,
    [switch]$SkipUserPath
)

$ErrorActionPreference = "Stop"

function Pause-IfNeeded {
    param([string]$Message = "Press Enter to exit")
    if ($PauseAtEnd -or $env:OPENAPI_INSTALL_PAUSE -eq "1") {
        Read-Host $Message
    }
}

function Add-CliBinToUserPath {
    param([string]$BinDir)
    $normalized = [System.IO.Path]::GetFullPath($BinDir).TrimEnd('\')
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $already = $false
    if (-not [string]::IsNullOrEmpty($userPath)) {
        foreach ($segment in $userPath -split ';') {
            if ([string]::IsNullOrWhiteSpace($segment)) { continue }
            try {
                $seg = [System.IO.Path]::GetFullPath($segment.Trim()).TrimEnd('\')
            } catch {
                continue
            }
            if ($seg -ieq $normalized) {
                $already = $true
                break
            }
        }
    }
    if ($already) {
        Write-Host "User PATH already contains: $normalized"
    } else {
        $newUserPath = if ([string]::IsNullOrEmpty($userPath)) { $normalized } else { "$userPath;$normalized" }
        [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
        Write-Host "Added to user PATH: $normalized"
        Write-Host "You can run: oapi call ...  (open a NEW terminal if this one does not find oapi yet)"
    }
    # Current session so `oapi` works immediately in this window
    if ($env:Path -notlike "*$normalized*") {
        $env:Path = "$normalized;$env:Path"
    }
}

try {
    $Root = Split-Path -Parent $PSScriptRoot
    $SkillName = Split-Path -Leaf $Root
    $SkillDest = Join-Path $env:USERPROFILE (Join-Path ".agents\skills" $SkillName)
    $BinDir = Join-Path $env:USERPROFILE ".local\bin"
    $CliDest = Join-Path $BinDir "oapi.exe"

    function Copy-TreeIfExists {
        param([string]$RelativeName)
        $src = Join-Path $Root $RelativeName
        if (-not (Test-Path -LiteralPath $src)) { return }
        $dst = Join-Path $SkillDest $RelativeName
        if (Test-Path -LiteralPath $dst) {
            Remove-Item -LiteralPath $dst -Recurse -Force
        }
        Copy-Item -LiteralPath $src -Destination $SkillDest -Recurse -Force
    }

    Write-Host "Skill source: $Root"
    Write-Host "Skill target: $SkillDest"

    New-Item -ItemType Directory -Path $SkillDest -Force | Out-Null

    Copy-TreeIfExists "assets"
    Copy-TreeIfExists "scripts"
    Copy-TreeIfExists "references"

    $skillMd = Join-Path $Root "SKILL.md"
    if (-not (Test-Path -LiteralPath $skillMd)) {
        throw "SKILL.md not found at $skillMd"
    }
    Copy-Item -LiteralPath $skillMd -Destination (Join-Path $SkillDest "SKILL.md") -Force

    $osArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
    switch ($osArch) {
        "x64" { $distArch = "amd64" }
        "arm64" { $distArch = "arm64" }
        default { throw "Unsupported Windows architecture: $osArch (expected x64 or arm64)" }
    }

    $prebuiltExe = Join-Path $Root "bin\oapi-windows-$distArch.exe"
    if (-not (Test-Path -LiteralPath $prebuiltExe)) {
        throw "Prebuilt binary not found: $prebuiltExe. Build and commit bin/oapi-windows-amd64.exe and bin/oapi-windows-arm64.exe first."
    }

    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    Copy-Item -LiteralPath $prebuiltExe -Destination $CliDest -Force
    Write-Host "Detected architecture: $osArch"
    Write-Host "Dist binary: $prebuiltExe"
    Write-Host "CLI installed as: $CliDest"

    if ((Test-Path -LiteralPath $CliDest) -and -not $SkipUserPath) {
        Add-CliBinToUserPath -BinDir $BinDir
    } elseif ($SkipUserPath -and (Test-Path -LiteralPath $CliDest)) {
        Write-Host "Skipped user PATH update (-SkipUserPath). Add $BinDir to PATH manually to run oapi without a full path."
    }

    Copy-TreeIfExists "bin"

    Write-Host "Done. Skill installed to $SkillDest"
    Pause-IfNeeded
    exit 0
} catch {
    Write-Host ""
    Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
    if (-not $env:CI) {
        Read-Host "Press Enter to exit"
    }
    exit 1
}
