<#
.SYNOPSIS
Demarre DevStation : verifie les prerequis, installe l'executable et lance l'assistant.

.EXAMPLE
powershell -ExecutionPolicy Bypass -File .\setup.ps1

.EXAMPLE
powershell -ExecutionPolicy Bypass -File .\setup.ps1 -Proxy http://proxy.example.com:8080 -Kerberos
#>
param(
    [switch]$Dev,
    [string]$Proxy,
    [switch]$Kerberos,
    [Parameter(ValueFromRemainingArguments = $true)]
    $Rest
)

$DevStationVersion = "0.1.0"
$PxVersion = "0.11.0"

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Import-Module (Join-Path $PSScriptRoot "bootstrap\DevStation.Bootstrap.psm1") -Force

Write-Host "DevStation $DevStationVersion"

$prereq = Test-Prerequisites
foreach ($message in $prereq.Messages) {
    Write-Host $message
}
if (-not $prereq.Ok) {
    exit 1
}

if ($Dev) {
    Push-Location $PSScriptRoot
    try {
        & go run ./cmd/devstation setup @Rest
        exit $LASTEXITCODE
    }
    finally {
        Pop-Location
    }
}

if ($Kerberos) {
    if (-not $Proxy) {
        Write-Host "L'option -Kerberos exige -Proxy."
        exit 1
    }
    $Proxy = Start-BootstrapPx -Proxy $Proxy -Dir "$env:LOCALAPPDATA\DevStation\px" -PxVersion $PxVersion
}

$exe = Install-DevStation -Version $DevStationVersion -BinDir "$env:LOCALAPPDATA\DevStation\bin" -Proxy $Proxy

if (Add-UserPath -Dir "$env:LOCALAPPDATA\DevStation\bin") {
    Write-Host "devstation a été ajouté à votre PATH : ouvrez un nouveau terminal pour l'utiliser."
}

$hasProfile = $false
foreach ($arg in $Rest) {
    if ($arg -eq "--profile") { $hasProfile = $true }
}

if ($hasProfile) {
    & $exe setup @Rest
}
else {
    & $exe setup --profile (Join-Path $PSScriptRoot "profiles\demo.yaml") @Rest
}
exit $LASTEXITCODE
