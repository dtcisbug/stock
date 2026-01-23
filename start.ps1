<#
.SYNOPSIS
    A股/期货实时行情服务 启动脚本

.PARAMETER Token
    API Token (ANTHROPIC_AUTH_TOKEN)

.PARAMETER BaseUrl
    API Base URL (ANTHROPIC_BASE_URL)

.PARAMETER Model
    模型名称 (ANTHROPIC_MODEL)

.PARAMETER Cli
    启用终端实时行情模式

.EXAMPLE
    .\start.ps1
    .\start.ps1 -Token "sk-xxx" -BaseUrl "http://proxy.com" -Model "claude-sonnet-4-5-20250929"
    .\start.ps1 -Cli
#>

param(
    [string]$Token = "",
    [string]$BaseUrl = "",
    [string]$Model = "",
    [switch]$Cli
)

Write-Host ""
Write-Host "========================================"
Write-Host "    A Stock / Futures Real-time Service"
Write-Host "========================================"
Write-Host ""

# Set environment variables
if ($Token -ne "") {
    $env:ANTHROPIC_AUTH_TOKEN = $Token
    Write-Host "[CONFIG] API Token set"
}

if ($BaseUrl -ne "") {
    $env:ANTHROPIC_BASE_URL = $BaseUrl
    Write-Host "[CONFIG] API Base URL: $BaseUrl"
}

if ($Model -ne "") {
    $env:ANTHROPIC_MODEL = $Model
    Write-Host "[CONFIG] Model: $Model"
}

# Get script directory
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ScriptDir

# Check if compiled
$ExePath = Join-Path $ScriptDir "stock.exe"
if (-not (Test-Path $ExePath)) {
    Write-Host "[BUILD] Compiling..."
    go build -o stock.exe .
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[ERROR] Build failed"
        exit 1
    }
    Write-Host "[BUILD] Done"
}

Write-Host "[START] Starting service..."
Write-Host ""

# Start service
if ($Cli) {
    & $ExePath -cli
} else {
    & $ExePath
}
