#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Install devsync CLI and server binaries.
.DESCRIPTION
    Builds devsync from source, adds the bin directory to the user PATH. Requires Go.
    One-liner: irm https://raw.githubusercontent.com/Hennnnnnn/DevWorkspace/main/scripts/install.ps1 | iex
.PARAMETER LocalPath
    Path to a pre-built bin/ directory. Skips download & build.
#>

param(
    [string]$LocalPath
)

$ErrorActionPreference = "Stop"
$Repo = "Hennnnnnn/DevWorkspace"

# --- targets ---
$BinDir = if ($LocalPath) { Resolve-Path -LiteralPath $LocalPath -ErrorAction Stop } else { "$HOME\.devsync\bin" }
$ServerUrl = "https://devworkspace.onrender.com"

Write-Host "==> devsync installer" -ForegroundColor Cyan
Write-Host "   target: $ServerUrl" -ForegroundColor Gray

# --- 1. resolve binary source ---
if ($LocalPath) {
    Write-Host "   using pre-built binaries from $LocalPath"
}
else {
    Write-Host "   building from source" -ForegroundColor Yellow

    $tmp = Join-Path $env:TMP "devsync-build-$(Get-Random)"
    Write-Host "   cloning $Repo ..." -ForegroundColor Gray
    git clone "https://github.com/$Repo.git" $tmp --quiet
    if ($LASTEXITCODE -ne 0) {
        throw "git clone failed (exit code: $LASTEXITCODE) — check network or repo access"
    }

    # Ensure output directory exists.
    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null

    Push-Location $tmp
    try {
        $ldflags = "-X github.com/Hennnnnnn/DevWorkspace/internal/client/config.DefaultServerURL=$ServerUrl"

        Write-Host "   building devsync.exe ..." -ForegroundColor Gray
        go build -ldflags "$ldflags" -o "$BinDir\devsync.exe" ./cmd/devsync
        if ($LASTEXITCODE -ne 0) {
            throw "go build devsync failed (exit code: $LASTEXITCODE)"
        }

        Write-Host "   building devsync-server.exe ..." -ForegroundColor Gray
        go build -ldflags "$ldflags" -o "$BinDir\devsync-server.exe" ./cmd/devsync-server
        if ($LASTEXITCODE -ne 0) {
            throw "go build devsync-server failed (exit code: $LASTEXITCODE)"
        }
    } finally {
        Pop-Location
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }

    Write-Host "   built devsync + devsync-server" -ForegroundColor Green
}

# --- 2. add to PATH ---
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$BinDir*") {
    $newPath = if ($userPath) { "$userPath;$BinDir" } else { "$BinDir" }
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    $env:Path += ";$BinDir"
    Write-Host "   added $BinDir to PATH (user)" -ForegroundColor Green
} else {
    Write-Host "   $BinDir already in PATH" -ForegroundColor Gray
}

# --- 3. verify ---
try {
    $v = & devsync --help 2>&1 | Select-Object -First 1
    Write-Host "   devsync ready: $v" -ForegroundColor Green
} catch {
    Write-Host "   installed — start a new terminal, then type 'devsync --help'" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "devsync installed!" -ForegroundColor Cyan
Write-Host "Server URL baked in: $ServerUrl" -ForegroundColor Gray
Write-Host "Get started:" -ForegroundColor Gray
Write-Host "  devsync init"
Write-Host "  devsync register --username <you>"
