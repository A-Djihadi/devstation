# Module de demarrage DevStation : telechargement verifie de l'exe et amorcage du proxy px.
# Compatible Windows PowerShell 5.1 et PowerShell 7.

Set-StrictMode -Version Latest

<#
.SYNOPSIS
Construit les URL de telechargement de l'executable et des empreintes pour une version donnee.
#>
function Get-DevStationUrls {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Version
    )

    $base = "https://github.com/A-Djihadi/devstation/releases/download/v$Version"
    return @{
        Exe  = "$base/devstation-windows-amd64.exe"
        Sums = "$base/checksums.txt"
    }
}

<#
.SYNOPSIS
Extrait l'empreinte sha256 d'un fichier depuis un texte au format sha256sum.
#>
function Get-ExpectedHash {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ChecksumsText,
        [Parameter(Mandatory = $true)]
        [string]$FileName
    )

    foreach ($line in ($ChecksumsText -split "`r?`n")) {
        $trimmed = $line.Trim()
        if ($trimmed -eq "") { continue }
        $parts = $trimmed -split '\s+', 2
        if ($parts.Count -lt 2) { continue }
        $hash = $parts[0]
        $name = $parts[1].Trim()
        if ($name.StartsWith("*")) { $name = $name.Substring(1) }
        if ($name -eq $FileName) {
            return $hash.ToLowerInvariant()
        }
    }

    throw "empreinte introuvable pour $FileName"
}

<#
.SYNOPSIS
Verifie que l'empreinte SHA256 d'un fichier correspond a l'empreinte attendue.
#>
function Test-FileHash256 {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,
        [Parameter(Mandatory = $true)]
        [string]$Expected
    )

    $actual = (Get-FileHash -Path $Path -Algorithm SHA256).Hash
    return ($actual.ToLowerInvariant() -eq $Expected.ToLowerInvariant())
}

<#
.SYNOPSIS
Telecharge un fichier avec TLS 1.2, trois essais et prise en charge optionnelle du proxy.
#>
function Invoke-Download {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Url,
        [Parameter(Mandatory = $true)]
        [string]$OutFile,
        [string]$Proxy
    )

    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

    $attempts = 3
    for ($i = 1; $i -le $attempts; $i++) {
        try {
            if ($Proxy) {
                Invoke-WebRequest -Uri $Url -OutFile $OutFile -UseBasicParsing -Proxy $Proxy -ProxyUseDefaultCredentials
            }
            else {
                Invoke-WebRequest -Uri $Url -OutFile $OutFile -UseBasicParsing
            }
            return
        }
        catch {
            if ($i -lt $attempts) {
                Start-Sleep -Seconds 2
            }
            else {
                throw "echec du telechargement de $Url : $($_.Exception.Message)"
            }
        }
    }
}

<#
.SYNOPSIS
Verifie les prerequis de la machine (Windows 64 bits, PowerShell 5.1, git, WSL).
#>
function Test-Prerequisites {
    $messages = New-Object System.Collections.Generic.List[string]
    $ok = $true

    if (-not [Environment]::Is64BitOperatingSystem) {
        $ok = $false
        $messages.Add("Windows 64 bits est requis.")
    }

    if ($PSVersionTable.PSVersion -lt [Version]"5.1") {
        $ok = $false
        $messages.Add("PowerShell 5.1 ou superieur est requis.")
    }

    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        $ok = $false
        $messages.Add("git est introuvable dans le PATH.")
    }

    if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
        $messages.Add("WSL est introuvable : Podman sera indisponible.")
    }

    return @{
        Ok       = $ok
        Messages = $messages.ToArray()
    }
}

<#
.SYNOPSIS
Telecharge l'executable DevStation, verifie son empreinte et renvoie son chemin.
#>
function Install-DevStation {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Version,
        [Parameter(Mandatory = $true)]
        [string]$BinDir,
        [string]$Proxy
    )

    if (-not (Test-Path -LiteralPath $BinDir)) {
        New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    }

    $urls = Get-DevStationUrls -Version $Version
    $exePath = Join-Path $BinDir "devstation.exe"
    $sumsPath = Join-Path $BinDir "checksums.txt"

    try {
        Invoke-Download -Url $urls.Exe -OutFile $exePath -Proxy $Proxy
        Invoke-Download -Url $urls.Sums -OutFile $sumsPath -Proxy $Proxy
    }
    catch {
        if (-not $Proxy) {
            $entered = Read-Host "Adresse du proxy (ex. http://proxy.example.com:8080), vide pour abandonner"
            if ($entered) {
                Invoke-Download -Url $urls.Exe -OutFile $exePath -Proxy $entered
                Invoke-Download -Url $urls.Sums -OutFile $sumsPath -Proxy $entered
            }
            else {
                throw
            }
        }
        else {
            throw
        }
    }

    $checksums = Get-Content -LiteralPath $sumsPath -Raw
    $expected = Get-ExpectedHash -ChecksumsText $checksums -FileName "devstation-windows-amd64.exe"

    if (-not (Test-FileHash256 -Path $exePath -Expected $expected)) {
        Remove-Item -LiteralPath $exePath -Force
        throw "empreinte invalide pour devstation.exe : fichier supprimé"
    }

    return $exePath
}

<#
.SYNOPSIS
Telecharge et lance px pour amorcer un proxy local (authentification Kerberos).
#>
function Start-BootstrapPx {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Proxy,
        [Parameter(Mandatory = $true)]
        [string]$Dir,
        [Parameter(Mandatory = $true)]
        [string]$PxVersion
    )

    if (-not (Test-Path -LiteralPath $Dir)) {
        New-Item -ItemType Directory -Path $Dir -Force | Out-Null
    }

    $zipName = "px-v$PxVersion-windows-amd64.zip"
    $base = "https://github.com/genotrance/px/releases/download/v$PxVersion"
    $zipPath = Join-Path $Dir $zipName
    $shaPath = Join-Path $Dir "$zipName.sha256"

    Invoke-Download -Url "$base/$zipName" -OutFile $zipPath -Proxy $Proxy
    Invoke-Download -Url "$base/$zipName.sha256" -OutFile $shaPath -Proxy $Proxy

    $shaText = Get-Content -LiteralPath $shaPath -Raw
    $expected = Get-ExpectedHash -ChecksumsText $shaText -FileName $zipName

    if (-not (Test-FileHash256 -Path $zipPath -Expected $expected)) {
        Remove-Item -LiteralPath $zipPath -Force
        throw "empreinte invalide pour $zipName : fichier supprimé"
    }

    Expand-Archive -LiteralPath $zipPath -DestinationPath $Dir -Force

    $server = $Proxy -replace '^[a-zA-Z]+://', ''
    $pxExe = Join-Path $Dir "px.exe"
    Start-Process -FilePath $pxExe -WindowStyle Hidden -ArgumentList "--server=$server", "--port=3128", "--listen=127.0.0.1"

    return "http://127.0.0.1:3128"
}

Export-ModuleMember -Function Get-DevStationUrls, Get-ExpectedHash, Test-FileHash256, Invoke-Download, Test-Prerequisites, Install-DevStation, Start-BootstrapPx
