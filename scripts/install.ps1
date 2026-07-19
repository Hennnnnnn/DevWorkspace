#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Install devsync CLI and server binaries.
.DESCRIPTION
    Downloads pre-built binaries from GitHub releases, or builds from source as fallback.
    Adds the bin directory to the user PATH and enables tab completion.
    One-liner: irm https://raw.githubusercontent.com/Hennnnnnn/DevWorkspace/main/scripts/install.ps1 | iex
.PARAMETER LocalPath
    Path to a pre-built bin/ directory. Skips download & build.
#>

param(
    [string]$LocalPath
)

$ErrorActionPreference = "Stop"
$Repo = "Hennnnnnn/DevWorkspace"

$BinDir = if ($LocalPath) { Resolve-Path -LiteralPath $LocalPath -ErrorAction Stop } else { "$HOME\.devsync\bin" }
$ServerUrl = "https://devworkspace.onrender.com"

Write-Host "==> devsync installer" -ForegroundColor Cyan
Write-Host "   target: $ServerUrl" -ForegroundColor Gray

New-Item -ItemType Directory -Path $BinDir -Force | Out-Null

# --- 1. resolve binary source ---
if ($LocalPath) {
    Write-Host "   using pre-built binaries from $LocalPath"
}
else {
    # Try download from latest release first.
    $downloaded = $false
    $api = "https://api.github.com/repos/$Repo/releases/latest"
    try {
        $release = Invoke-RestMethod -Uri $api -ErrorAction Stop
        $tag = $release.tag_name
        $url = "https://github.com/$Repo/releases/download/$tag/devsync_windows_amd64.zip"
        Write-Host "   downloading $tag ..." -ForegroundColor Gray

        $tmp = Join-Path $env:TMP "devsync-$(Get-Random)"
        New-Item -ItemType Directory -Path $tmp -Force | Out-Null
        $zip = Join-Path $tmp "archive.zip"
        Invoke-WebRequest -Uri $url -OutFile $zip
        Expand-Archive -Path $zip -DestinationPath $tmp -Force
        Move-Item "$tmp\devsync.exe" "$BinDir\devsync.exe" -Force -ErrorAction Stop
        Move-Item "$tmp\devsync-server.exe" "$BinDir\devsync-server.exe" -Force -ErrorAction Stop
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
        $downloaded = $true
        Write-Host "   downloaded devsync + devsync-server" -ForegroundColor Green
    }
    catch {
        Write-Host "   no release found, building from source" -ForegroundColor Yellow
    }

    if (-not $downloaded) {
        $tmp = Join-Path $env:TMP "devsync-build-$(Get-Random)"
        Write-Host "   cloning $Repo ..." -ForegroundColor Gray
        git clone "https://github.com/$Repo.git" $tmp --quiet
        if ($LASTEXITCODE -ne 0) {
            throw "git clone failed (exit code: $LASTEXITCODE)"
        }

        Push-Location $tmp
        try {
            $version = if ($tag) { $tag } else { "dev" }
        $ldflags = "-X github.com/Hennnnnnn/DevWorkspace/internal/client/config.DefaultServerURL=$ServerUrl -X github.com/Hennnnnnn/DevWorkspace/internal/client/commands.Version=$version"
            Write-Host "   building devsync.exe ..." -ForegroundColor Gray
            go build -ldflags "$ldflags" -o "$BinDir\devsync.exe" ./cmd/devsync
            if ($LASTEXITCODE -ne 0) { throw "go build devsync failed (exit code: $LASTEXITCODE)" }

            Write-Host "   building devsync-server.exe ..." -ForegroundColor Gray
            go build -ldflags "$ldflags" -o "$BinDir\devsync-server.exe" ./cmd/devsync-server
            if ($LASTEXITCODE -ne 0) { throw "go build devsync-server failed (exit code: $LASTEXITCODE)" }
        } finally {
            Pop-Location
            Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
        }
        Write-Host "   built devsync + devsync-server" -ForegroundColor Green
    }
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
    $null = & devsync --help 2>&1
    Write-Host "   devsync ready — run 'devsync --help' to see commands" -ForegroundColor Green
} catch {
    Write-Host "   installed - start a new terminal, then type 'devsync --help'" -ForegroundColor Yellow
}

# --- 4. auto-completion (PowerShell) ---
try {
    $completionScript = & devsync completion powershell 2>&1 | Out-String
    if ($LASTEXITCODE -eq 0 -and $completionScript) {
        $profileDir = Split-Path $PROFILE -Parent
        if (!(Test-Path $profileDir)) { New-Item -ItemType Directory -Path $profileDir -Force | Out-Null }
        if (!(Test-Path $PROFILE) -or (Get-Content $PROFILE -Raw) -notmatch "devsync completion") {
            Add-Content $PROFILE "`n# devsync tab completion`n$completionScript"
            Write-Host "   tab completion added to `$PROFILE" -ForegroundColor Green
        } else {
            Write-Host "   tab completion already configured" -ForegroundColor Gray
        }
    }
} catch {
    Write-Host "   tab completion skipped (run 'devsync completion powershell >> `$PROFILE' manually)" -ForegroundColor Gray
}

Write-Host ""
Write-Host "devsync installed!" -ForegroundColor Cyan
Write-Host "Server URL baked in: $ServerUrl" -ForegroundColor Gray
Write-Host "Get started:" -ForegroundColor Gray
Write-Host "  devsync init"
Write-Host "  devsync register --username <you>"
