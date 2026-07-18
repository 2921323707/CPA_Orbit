[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [ValidateSet("start", "stop", "restart", "status", "test", "setup")]
    [string]$Command = "status",

    [switch]$WithUI,
    [switch]$Open
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$CoreExe = Join-Path $Root "app\cli-proxy-api.exe"
$CoreConfig = Join-Path $Root "app\config.yaml"
$ChatRoot = Join-Path $Root "Application\Chat"
$ChatPython = Join-Path $ChatRoot ".venv\Scripts\python.exe"
$ChatPidFile = Join-Path $ChatRoot "backend\data\chat.pid"
$LogsDir = Join-Path $Root "logs"
$ApiBaseUrl = if ($env:CPA_BASE_URL) { $env:CPA_BASE_URL.TrimEnd("/") } else { "http://127.0.0.1:8317/v1" }
$ChatBaseUrl = "http://127.0.0.1:5050"

function Get-CpaApiKey {
    if ($env:CPA_API_KEY) {
        return $env:CPA_API_KEY
    }

    if (-not (Test-Path -LiteralPath $CoreConfig)) {
        throw "缺少配置文件：$CoreConfig"
    }

    $insideApiKeys = $false
    foreach ($line in Get-Content -LiteralPath $CoreConfig) {
        if ($line -match '^api-keys:\s*$') {
            $insideApiKeys = $true
            continue
        }
        if ($insideApiKeys -and $line -match '^\s+-\s+["'']?([^"''#\s]+)') {
            return $Matches[1]
        }
        if ($insideApiKeys -and $line -match '^\S') {
            break
        }
    }

    throw "config.yaml 中没有可用的 api-keys。"
}

function Get-CoreProcess {
    Get-CimInstance Win32_Process -Filter "Name='cli-proxy-api.exe'" -ErrorAction SilentlyContinue |
        Where-Object {
            $_.ExecutablePath -and
            [string]::Equals($_.ExecutablePath, $CoreExe, [System.StringComparison]::OrdinalIgnoreCase)
        }
}

function Get-ChatProcess {
    if (-not (Test-Path -LiteralPath $ChatPidFile)) {
        return $null
    }

    $rawPid = (Get-Content -LiteralPath $ChatPidFile -Raw).Trim()
    if ($rawPid -notmatch '^\d+$') {
        Remove-Item -LiteralPath $ChatPidFile -Force
        return $null
    }

    $process = Get-CimInstance Win32_Process -Filter "ProcessId=$rawPid" -ErrorAction SilentlyContinue
    if ($null -eq $process -or $process.CommandLine -notmatch 'backend[\\/]server\.py') {
        Remove-Item -LiteralPath $ChatPidFile -Force
        return $null
    }
    return $process
}

function Test-CoreApi {
    param([switch]$Quiet)
    try {
        $key = Get-CpaApiKey
        $response = Invoke-RestMethod -Uri "$ApiBaseUrl/models" -Headers @{ Authorization = "Bearer $key" } -Method Get -TimeoutSec 5
        $count = @($response.data).Count
        if (-not $Quiet) {
            Write-Host "核心 API 正常：$ApiBaseUrl（$count 个模型）"
        }
        return $true
    }
    catch {
        if (-not $Quiet) {
            Write-Host "核心 API 不可用：$($_.Exception.Message)" -ForegroundColor Red
        }
        return $false
    }
}

function Test-ChatApi {
    param([switch]$Quiet)
    try {
        $response = Invoke-RestMethod -Uri "$ChatBaseUrl/api/health" -Method Get -TimeoutSec 3
        if (-not $response.ok) {
            throw "健康检查未返回 ok=true"
        }
        if (-not $Quiet) {
            Write-Host "Chat Studio 正常：$ChatBaseUrl"
        }
        return $true
    }
    catch {
        if (-not $Quiet) {
            Write-Host "Chat Studio 未运行或不可用。"
        }
        return $false
    }
}

function Wait-For {
    param(
        [Parameter(Mandatory = $true)][scriptblock]$Probe,
        [int]$TimeoutSeconds = 20
    )
    for ($attempt = 0; $attempt -lt $TimeoutSeconds; $attempt++) {
        if (& $Probe) {
            return $true
        }
        Start-Sleep -Seconds 1
    }
    return $false
}

function Start-Core {
    if (Get-CoreProcess) {
        Write-Host "核心 API 已在运行。"
        return
    }
    if (-not (Test-Path -LiteralPath $CoreExe)) {
        throw "缺少核心程序：$CoreExe"
    }
    if (-not (Test-Path -LiteralPath $CoreConfig)) {
        throw "缺少配置文件：$CoreConfig"
    }

    $occupied = Get-NetTCPConnection -LocalPort 8317 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($occupied) {
        throw "8317 端口已被 PID $($occupied.OwningProcess) 占用。"
    }

    New-Item -ItemType Directory -Path $LogsDir -Force | Out-Null
    Start-Process -FilePath $CoreExe `
        -ArgumentList "-config `"$CoreConfig`"" `
        -WorkingDirectory (Split-Path -Parent $CoreExe) `
        -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $LogsDir "core.out.log") `
        -RedirectStandardError (Join-Path $LogsDir "core.err.log") | Out-Null

    if (-not (Wait-For -Probe { Test-CoreApi -Quiet })) {
        throw "核心 API 启动超时，请检查 logs\core.err.log。"
    }
    Write-Host "核心 API 已启动：$ApiBaseUrl"
}

function Start-Chat {
    if (Test-ChatApi -Quiet) {
        Write-Host "Chat Studio 已在运行。"
        return
    }
    if (-not (Test-Path -LiteralPath $ChatPython)) {
        throw "Chat Studio 环境不存在，请先运行：.\cpa.ps1 setup"
    }

    New-Item -ItemType Directory -Path $LogsDir -Force | Out-Null
    Start-Process -FilePath $ChatPython `
        -ArgumentList "-B backend\server.py" `
        -WorkingDirectory $ChatRoot `
        -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $LogsDir "chat.out.log") `
        -RedirectStandardError (Join-Path $LogsDir "chat.err.log") | Out-Null

    if (-not (Wait-For -Probe { Test-ChatApi -Quiet })) {
        throw "Chat Studio 启动超时，请检查 logs\chat.err.log。"
    }
    Write-Host "Chat Studio 已启动：$ChatBaseUrl"
}

function Stop-Chat {
    $process = Get-ChatProcess
    if ($null -eq $process) {
        Write-Host "Chat Studio 未运行。"
        return
    }
    Stop-Process -Id $process.ProcessId -Force
    Wait-Process -Id $process.ProcessId -Timeout 10 -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $ChatPidFile -Force -ErrorAction SilentlyContinue
    Write-Host "Chat Studio 已停止。"
}

function Stop-Core {
    $processes = @(Get-CoreProcess)
    if ($processes.Count -eq 0) {
        Write-Host "核心 API 未运行。"
        return
    }
    foreach ($process in $processes) {
        Stop-Process -Id $process.ProcessId -Force
        Wait-Process -Id $process.ProcessId -Timeout 10 -ErrorAction SilentlyContinue
    }
    Write-Host "核心 API 已停止。"
}

function Install-Chat {
    if (-not (Test-Path -LiteralPath $ChatRoot)) {
        throw "缺少 Chat Studio 目录：$ChatRoot"
    }
    if (-not (Test-Path -LiteralPath $ChatPython)) {
        $python = Get-Command python.exe -ErrorAction SilentlyContinue
        if ($null -eq $python) {
            throw "未找到 Python 3.11+。"
        }
        & $python.Source -m venv (Join-Path $ChatRoot ".venv")
        if ($LASTEXITCODE -ne 0) {
            throw "创建 Python 虚拟环境失败。"
        }
    }
    & $ChatPython -m pip install -r (Join-Path $ChatRoot "requirements.txt")
    if ($LASTEXITCODE -ne 0) {
        throw "安装 Chat Studio 依赖失败。"
    }
    Write-Host "Chat Studio 环境已就绪。"
}

function Show-Status {
    $coreProcess = @(Get-CoreProcess)
    $coreHealthy = Test-CoreApi -Quiet
    $chatProcess = Get-ChatProcess
    $chatHealthy = Test-ChatApi -Quiet

    [pscustomobject]@{
        Component = "Core API"
        Process = if ($coreProcess.Count -gt 0) { "running" } else { "stopped" }
        Health = if ($coreHealthy) { "ok" } else { "unavailable" }
        Endpoint = $ApiBaseUrl
    }
    [pscustomobject]@{
        Component = "Chat Studio (optional)"
        Process = if ($null -ne $chatProcess) { "running" } else { "stopped" }
        Health = if ($chatHealthy) { "ok" } else { "unavailable" }
        Endpoint = $ChatBaseUrl
    }

    if (-not $coreHealthy) {
        exit 1
    }
}

function Invoke-SmokeTest {
    if (-not (Test-CoreApi)) {
        throw "核心 API 冒烟测试失败。"
    }
    if (Test-ChatApi -Quiet) {
        $accounts = Invoke-RestMethod -Uri "$ChatBaseUrl/api/accounts" -Method Get -TimeoutSec 30
        Write-Host "账号路由正常：$($accounts.count) 个账号（响应不含令牌）。"
    }
    else {
        Write-Host "Chat Studio 未运行，已跳过可选 UI 检查。"
    }
    Write-Host "冒烟测试通过。" -ForegroundColor Green
}

try {
    switch ($Command) {
        "start" {
            Start-Core
            if ($WithUI -or $Open) {
                Start-Chat
            }
            if ($Open) {
                Start-Process $ChatBaseUrl
            }
        }
        "stop" {
            Stop-Chat
            Stop-Core
        }
        "restart" {
            $restartUi = $WithUI -or $Open -or (Test-ChatApi -Quiet)
            Stop-Chat
            Stop-Core
            Start-Core
            if ($restartUi) {
                Start-Chat
            }
            if ($Open) {
                Start-Process $ChatBaseUrl
            }
        }
        "status" { Show-Status }
        "test" { Invoke-SmokeTest }
        "setup" { Install-Chat }
    }
}
catch {
    Write-Error $_.Exception.Message
    exit 1
}
