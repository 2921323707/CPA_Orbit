[CmdletBinding()]
param(
    [switch]$SkipE2E,
    [switch]$SkipPackage
)

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$LocalGo = Join-Path $RepoRoot '.tools\go\bin\go.exe'

if (Test-Path -LiteralPath $LocalGo) {
    $Go = $LocalGo
} else {
    $GoCommand = Get-Command go -ErrorAction SilentlyContinue
    if (-not $GoCommand) {
        throw 'Go is required. Install Go or restore .tools/go.'
    }
    $Go = $GoCommand.Source
}

if (-not (Get-Command node -ErrorAction SilentlyContinue) -or -not (Get-Command npm -ErrorAction SilentlyContinue)) {
    throw 'Node.js and npm are required.'
}

function Invoke-NativeStep {
    param(
        [Parameter(Mandatory)]
        [string]$Name,
        [Parameter(Mandatory)]
        [scriptblock]$Action
    )

    Write-Host "`n==> $Name" -ForegroundColor Cyan
    & $Action
    if ($LASTEXITCODE -ne 0) {
        throw "$Name failed with exit code $LASTEXITCODE"
    }
}

Push-Location (Join-Path $RepoRoot 'server')
try {
    Invoke-NativeStep 'Server tests' { & $Go test -count=1 ./... }
    Invoke-NativeStep 'Server vet' { & $Go vet ./... }
} finally {
    Pop-Location
}

Push-Location (Join-Path $RepoRoot 'app')
try {
    Invoke-NativeStep 'Desktop tests' { & $Go test -count=1 ./... }
    Invoke-NativeStep 'Desktop vet' { & $Go vet ./... }
} finally {
    Pop-Location
}

Push-Location (Join-Path $RepoRoot 'web')
try {
    Invoke-NativeStep 'Browser production build' { npm run build }
    Invoke-NativeStep 'Desktop frontend build' { npm run build:desktop }
    if (-not $SkipE2E) {
        Invoke-NativeStep 'Playwright E2E' { npm run test:e2e }
    }
} finally {
    Pop-Location
}

if (-not $SkipPackage) {
    Invoke-NativeStep 'Windows portable package' { & (Join-Path $RepoRoot 'app\build-windows.ps1') }
}

Write-Host "`nCPA Orbit verification completed successfully." -ForegroundColor Green
