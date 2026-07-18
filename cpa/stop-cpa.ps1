$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$exe = Join-Path $root 'app\cli-proxy-api.exe'
$pidFile = Join-Path $root '.runtime\cpa.pid'
$process = $null

if (Test-Path $pidFile) {
    $savedPid = [int](Get-Content $pidFile -Raw)
    $process = Get-Process -Id $savedPid -ErrorAction SilentlyContinue
}
if (-not $process) {
    $listener = Get-NetTCPConnection -LocalPort 8317 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($listener) { $process = Get-Process -Id $listener.OwningProcess -ErrorAction SilentlyContinue }
}
if (-not $process) {
    Write-Host 'CLIProxyAPI is not running.' -ForegroundColor Yellow
    exit 0
}
if ([IO.Path]::GetFullPath($process.Path) -ne [IO.Path]::GetFullPath($exe)) {
    throw "Refusing to stop unrelated process: $($process.Path)"
}

Stop-Process -Id $process.Id -Force -Confirm:$false
Remove-Item $pidFile -Force -ErrorAction SilentlyContinue -Confirm:$false
Write-Host "Stopped merged CLIProxyAPI process $($process.Id)." -ForegroundColor Yellow
