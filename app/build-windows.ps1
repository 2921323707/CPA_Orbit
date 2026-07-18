[CmdletBinding()]
param(
    [ValidateSet('amd64', 'arm64')]
    [string]$Arch = 'amd64',
    [switch]$Installer
)

$ErrorActionPreference = 'Stop'
$WailsVersion = 'v2.13.0'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$LocalGo = Join-Path $RepoRoot '.tools\go\bin\go.exe'

if (Test-Path $LocalGo) {
    $Go = $LocalGo
} else {
    $GoCommand = Get-Command go -ErrorAction SilentlyContinue
    if (-not $GoCommand) {
        throw 'Go 1.25 or newer is required. Install Go and retry.'
    }
    $Go = $GoCommand.Source
}
if (-not (Get-Command node -ErrorAction SilentlyContinue) -or -not (Get-Command npm -ErrorAction SilentlyContinue)) {
    throw 'Node.js and npm are required. Install them and retry.'
}

$env:PATH = "$(Split-Path -Parent $Go);$env:PATH"
$BuildArgs = @(
    'run', "github.com/wailsapp/wails/v2/cmd/wails@$WailsVersion",
    'build', '-clean', '-trimpath', '-skipbindings', '-platform', "windows/$Arch",
    '-ldflags', '-s -w', '-webview2', 'download'
)
if ($Installer) {
    if (-not (Get-Command makensis -ErrorAction SilentlyContinue)) {
        throw 'NSIS is required for -Installer. Install NSIS or build the portable EXE without -Installer.'
    }
    $BuildArgs += @('-nsis', '-installscope', 'user')
}

Push-Location $PSScriptRoot
try {
    & $Go @BuildArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Wails build failed with exit code $LASTEXITCODE"
    }

    $OutputDir = Join-Path $PSScriptRoot 'build\bin'
    Copy-Item (Join-Path $PSScriptRoot 'cpa-orbit.config.example.json') (Join-Path $OutputDir 'cpa-orbit.config.example.json') -Force
    Copy-Item (Join-Path $RepoRoot 'LICENSE') (Join-Path $OutputDir 'LICENSE.txt') -Force
    Copy-Item (Join-Path $RepoRoot 'THIRD_PARTY_NOTICES.md') (Join-Path $OutputDir 'THIRD_PARTY_NOTICES.md') -Force

    $Checksums = Get-ChildItem $OutputDir -File | Where-Object { $_.Extension -eq '.exe' } | ForEach-Object {
        $Hash = (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        "$Hash  $($_.Name)"
    }
    $Checksums | Set-Content (Join-Path $OutputDir 'CHECKSUMS-SHA256.txt') -Encoding utf8
    Write-Host "Build complete: $OutputDir" -ForegroundColor Green
} finally {
    Pop-Location
}
