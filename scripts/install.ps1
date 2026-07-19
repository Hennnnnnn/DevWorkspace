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
$BinDir = if ($LocalPath) { Resolve-Path $LocalPath } else { "$HOME\.devsync\bin" }
$ServerUrl = "https://devworkspace.onrender.com"

Write-Host "==> devsync installer" -ForegroundColor Cyan

# --- 1. resolve binary source ---
if ($LocalPath) {
    Write-Host "   using pre-built binaries from $LocalPath"
}
else {
    Write-Host "   building from source" -ForegroundColor Yellow
    $tmp = Join-Path $env:TMP "devsync-build-$(Get-Random)"
    git clone "https://github.com/$Repo.git" $tmp 2>&1 | Out-Null
    Push-Location $tmp
    try {
        $script:buildOk = $false
        $ldflags = "-X github.com/Hennnnnnn/DevWorkspace/internal/client/config.DefaultServerURL=$ServerUrl"
        go build -ldflags "$ldflags" -o "$BinDir\devsync.exe" ./cmd/devsync 2>&1
        go build -ldflags "$ldflags" -o "$BinDir\devsync-server.exe" ./cmd/devsync-server 2>&1
        $script:buildOk = $true
    } finally {
        Pop-Location
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }
    if (-not $script:buildOk) { throw "build failed" }
    Write-Host "   built devsync + devsync-server" -ForegroundColor Green
}

# --- 2. add to PATH ---
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$BinDir*") {
    $newPath = if ($userPath) { "$userPath;$BinDir" } else { "$BinDir" }
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    # also update current session
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
    Write-Host "   installed but start a new terminal to pick up PATH" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "devsync installed!" -ForegroundColor Cyan
Write-Host "Server URL baked in: $ServerUrl" -ForegroundColor Gray
Write-Host "Get started:" -ForegroundColor Gray
Write-Host "  devsync init"
Write-Host "  devsync register --username <you>"
