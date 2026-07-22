$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$server = Join-Path $root 'server'
$web = Join-Path $root 'web'
$cpaStart = Join-Path $root 'cpa\start-cpa.ps1'
$portableGo = Join-Path $root '.tools\go\bin\go.exe'
$go = if (Get-Command go -ErrorAction SilentlyContinue) { 'go' } elseif (Test-Path $portableGo) { $portableGo } else { throw 'Go not found. Install Go or place portable Go under .tools/go.' }
$ownedChildren = @()

function Get-EnvValue([string] $name) {
    $value = [Environment]::GetEnvironmentVariable($name)
    if ($null -eq $value) { return '' }
    return $value.Trim()
}

function Test-HttpReady([string] $uri, [string] $expected) {
    try {
        $response = Invoke-WebRequest -Uri $uri -TimeoutSec 1
        return $response.StatusCode -eq 200 -and $response.Content.Contains($expected)
    } catch { return $false }
}

function Start-Sub2Api {
    $executable = Get-EnvValue 'CPA_ORBIT_SUB2API_EXECUTABLE'
    $argsJSON = Get-EnvValue 'CPA_ORBIT_SUB2API_ARGS'
    $address = Get-EnvValue 'CPA_ORBIT_SUB2API_ADDRESS'
    $readinessURL = Get-EnvValue 'CPA_ORBIT_SUB2API_READINESS_URL'
    $required = (Get-EnvValue 'CPA_ORBIT_SUB2API_REQUIRED') -eq 'true'
    if ([string]::IsNullOrWhiteSpace($executable) -or [string]::IsNullOrWhiteSpace($argsJSON) -or [string]::IsNullOrWhiteSpace($address)) {
        if ($required) { throw 'Required Sub2API companion configuration is missing: set CPA_ORBIT_SUB2API_EXECUTABLE, CPA_ORBIT_SUB2API_ARGS (JSON array), and CPA_ORBIT_SUB2API_ADDRESS.' }
        Write-Host 'Sub2API companion is not configured; skipping.' -ForegroundColor Yellow
        return
    }
    try { $args = ConvertFrom-Json -InputObject $argsJSON -AsHashtable } catch { throw 'CPA_ORBIT_SUB2API_ARGS must be a JSON array.' }
    if ($args -isnot [array]) { throw 'CPA_ORBIT_SUB2API_ARGS must be a JSON array.' }
    if ($readinessURL -and (Test-HttpReady $readinessURL '')) {
        Write-Host "Reusing Sub2API at $address" -ForegroundColor Cyan
        return
    }
    if (-not (Test-Path $executable)) { throw "Configured Sub2API executable not found: $executable" }
    $workingDirectory = Get-EnvValue 'CPA_ORBIT_SUB2API_WORKING_DIRECTORY'
    $processArgs = @{ FilePath = $executable; ArgumentList = [string[]]$args; PassThru = $true }
    if ($workingDirectory) { $processArgs.WorkingDirectory = $workingDirectory }
    $process = Start-Process @processArgs
    $ownedChildren += $process
    $timeout = 10
    $timeoutValue = Get-EnvValue 'CPA_ORBIT_SUB2API_STARTUP_TIMEOUT'
    if ($timeoutValue) { [int]::TryParse($timeoutValue, [ref]$timeout) | Out-Null }
    for ($attempt = 0; $attempt -lt ($timeout * 4); $attempt++) {
        if ($readinessURL -and (Test-HttpReady $readinessURL '')) { Write-Host "Sub2API is ready at $address" -ForegroundColor Green; return }
        Start-Sleep -Milliseconds 250
    }
    if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force -Confirm:$false }
    throw "Sub2API did not become ready within $timeout seconds."
}

try {
    Write-Host 'Starting or reusing CPA...' -ForegroundColor Magenta
    & $cpaStart
    Start-Sub2Api

    $monitorOnline = Test-HttpReady 'http://127.0.0.1:8090/api/health' '"version":"1.3.0"'
    if ($monitorOnline) {
        Write-Host 'Reusing CPA Monitor API at http://127.0.0.1:8090' -ForegroundColor Cyan
    } else {
        Write-Host 'Starting CPA Monitor API at http://127.0.0.1:8090' -ForegroundColor Cyan
        $monitor = Start-Process pwsh -ArgumentList '-NoExit', '-Command', "Set-Location '$server'; & '$go' run ./cmd/server -project-root '$root'" -PassThru
        $ownedChildren += $monitor
    }

    Write-Host 'Starting CPA Monitor UI at http://127.0.0.1:5173' -ForegroundColor Green
    Write-Host 'CLIProxyAPI is available at http://127.0.0.1:8317/v1' -ForegroundColor Magenta
    Set-Location $web
    npm run dev
} finally {
    foreach ($child in ($ownedChildren | Sort-Object -Property Id -Descending)) {
        if ($child -and -not $child.HasExited) { Stop-Process -Id $child.Id -Force -Confirm:$false }
    }
}
