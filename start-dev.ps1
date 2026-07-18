$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$server = Join-Path $root 'server'
$web = Join-Path $root 'web'
$cpaStart = Join-Path $root 'cpa\start-cpa.ps1'
$portableGo = Join-Path $root '.tools\go\bin\go.exe'
$go = if (Get-Command go -ErrorAction SilentlyContinue) { 'go' } elseif (Test-Path $portableGo) { $portableGo } else { throw 'Go not found. Install Go or place portable Go under .tools/go.' }

Write-Host 'Starting mature CLIProxyAPI 7.2.71...' -ForegroundColor Magenta
& $cpaStart

Write-Host 'Starting CPA Monitor API at http://127.0.0.1:8080' -ForegroundColor Cyan
Start-Process pwsh -ArgumentList '-NoExit', '-Command', "Set-Location '$server'; & '$go' run ./cmd/server -project-root '$root'"

Write-Host 'Starting CPA Monitor UI at http://127.0.0.1:5173' -ForegroundColor Green
Write-Host 'CLIProxyAPI is available at http://127.0.0.1:8317/v1' -ForegroundColor Magenta
Set-Location $web
npm run dev
