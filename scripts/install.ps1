#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Install devsync CLI and server binaries.
.DESCRIPTION
    Downloads or builds devsync, adds the bin directory to the user PATH.
    One-liner: irm https://raw.githubusercontent.com/Hennnnnnn/DevWorkspaceSync/main/scripts/install.ps1 | iex
.PARAMETER Version
    GitHub release tag to download (default: latest). Ignored when -Build is set.
.PARAMETER Build
    Build from source instead of downloading a release. Requires Go.
.PARAMETER LocalPath
    Path to a pre-built bin/ directory. Skips download & build.
#>

param(
    [string]$Version = "latest",
    [switch]$Build,
    [string]$LocalPath
)

$ErrorActionPreference = "Stop"
$Repo = "Hennnnnnn/DevWorkspaceSync"

# --- targets ---
$BinDir = if ($LocalPath) { Resolve-Path $LocalPath } else { "$HOME\.devsync\bin" }
$ServerUrl = "http://localhost:8080"

Write-Host "==> devsync installer" -ForegroundColor Cyan

# --- 1. resolve binary source ---
if ($LocalPath) {
    Write-Host "   using pre-built binaries from $LocalPath"
}
elseif ($Build) {
    Write-Host "   building from source (requires Go)" -ForegroundColor Yellow
    $tmp = Join-Path $env:TMP "devsync-build-$(Get-Random)"
    git clone "https://github.com/$Repo.git" $tmp 2>&1 | Out-Null
    Push-Location $tmp
    try {
        $script:buildOk = $false
        go build -o "$BinDir\devsync.exe" ./cmd/devsync 2>&1
        go build -o "$BinDir\devsync-server.exe" ./cmd/devsync-server 2>&1
        $script:buildOk = $true
    } finally {
        Pop-Location
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }
    if (-not $script:buildOk) { throw "build failed" }
    Write-Host "   built devsync + devsync-server" -ForegroundColor Green
}
else {
    # download from GitHub releases
    if ($Version -eq "latest") {
        $api = "https://api.github.com/repos/$Repo/releases/latest"
        $release = Invoke-RestMethod $api
        $tag = $release.tag_name
    } else {
        $tag = $Version
    }

    $arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
    $os = if ($IsWindows -or (-not $IsLinux -and -not $IsMacOs)) { "windows" } else { "linux" }
    $ext = if ($os -eq "windows") { "zip" } else { "tar.gz" }

    $url = "https://github.com/$Repo/releases/download/$tag/devsync_${os}_${arch}.$ext"
    Write-Host "   downloading $url"

    $tmp = Join-Path $env:TMP "devsync-$(Get-Random)"
    New-Item -ItemType Directory -Path $tmp -Force | Out-Null
    $archive = Join-Path $tmp "archive.$ext"

    Invoke-WebRequest -Uri $url -OutFile $archive
    if ($ext -eq "zip") {
        Expand-Archive -Path $archive -DestinationPath $tmp
    } else {
        tar -xzf $archive -C $tmp
    }
    Move-Item "$tmp\devsync.exe" "$BinDir\devsync.exe" -Force -ErrorAction SilentlyContinue
    Move-Item "$tmp\devsync" "$BinDir\devsync" -Force -ErrorAction SilentlyContinue
    Move-Item "$tmp\devsync-server.exe" "$BinDir\devsync-server.exe" -Force -ErrorAction SilentlyContinue
    Move-Item "$tmp\devsync-server" "$BinDir\devsync-server" -Force -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
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
Write-Host "Set your server URL and register:" -ForegroundColor Gray
Write-Host "  devsync config set server_url $ServerUrl"
Write-Host "  devsync init"
Write-Host "  devsync register --username <you>"
