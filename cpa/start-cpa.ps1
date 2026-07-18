$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$exe = Join-Path $root 'app\cli-proxy-api.exe'
$config = Join-Path $root 'app\config.yaml'
$runtime = Join-Path $root '.runtime'
$pidFile = Join-Path $runtime 'cpa.pid'

if (-not (Test-Path $exe)) { throw "CLIProxyAPI executable not found: $exe" }
if (-not (Test-Path $config)) { throw "CLIProxyAPI config not found: $config" }

$listener = Get-NetTCPConnection -LocalPort 8317 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
if ($listener) {
    $process = Get-Process -Id $listener.OwningProcess -ErrorAction Stop
    if ([IO.Path]::GetFullPath($process.Path) -eq [IO.Path]::GetFullPath($exe)) {
        Write-Host "CLIProxyAPI already running (PID $($process.Id))." -ForegroundColor Green
        exit 0
    }
    throw "Port 8317 is occupied by $($process.Path). Stop that process before starting the merged CPA service."
}

New-Item -ItemType Directory -Force -Path $runtime | Out-Null
$process = Start-Process -FilePath $exe -ArgumentList @('-config', $config) -WorkingDirectory (Split-Path -Parent $exe) -PassThru
Set-Content -Path $pidFile -Value $process.Id -Encoding ascii

for ($attempt = 0; $attempt -lt 40; $attempt++) {
    if (Get-NetTCPConnection -LocalPort 8317 -State Listen -ErrorAction SilentlyContinue) {
        Write-Host "CLIProxyAPI 7.2.71 is listening on http://127.0.0.1:8317 (PID $($process.Id))." -ForegroundColor Green
        exit 0
    }
    Start-Sleep -Milliseconds 250
}

if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force -Confirm:$false }
throw 'CLIProxyAPI did not open port 8317 within 10 seconds.'
