BeforeAll {
    Import-Module (Join-Path $PSScriptRoot "..\..\bootstrap\DevStation.Bootstrap.psm1") -Force
}

Describe "Get-DevStationUrls" {
    It "construit les deux URL exactes" {
        $urls = Get-DevStationUrls -Version "1.2.3"
        $urls.Exe | Should -Be "https://github.com/A-Djihadi/devstation/releases/download/v1.2.3/devstation-windows-amd64.exe"
        $urls.Sums | Should -Be "https://github.com/A-Djihadi/devstation/releases/download/v1.2.3/checksums.txt"
    }
}

Describe "Get-ExpectedHash" {
    It "trouve la bonne ligne parmi plusieurs" {
        $text = @"
aaaa1111  autre.exe
bbbb2222  devstation-windows-amd64.exe
cccc3333  encore.exe
"@
        Get-ExpectedHash -ChecksumsText $text -FileName "devstation-windows-amd64.exe" | Should -Be "bbbb2222"
    }

    It "echoue si le fichier est absent" {
        $text = "aaaa1111  autre.exe"
        { Get-ExpectedHash -ChecksumsText $text -FileName "devstation-windows-amd64.exe" } | Should -Throw
    }
}

Describe "Test-FileHash256" {
    It "vrai pour un fichier au contenu connu" {
        $path = Join-Path $TestDrive "contenu.txt"
        Set-Content -LiteralPath $path -Value "devstation" -NoNewline
        $expected = (Get-FileHash -Path $path -Algorithm SHA256).Hash
        Test-FileHash256 -Path $path -Expected $expected | Should -BeTrue
    }

    It "faux pour une empreinte differente" {
        $path = Join-Path $TestDrive "contenu2.txt"
        Set-Content -LiteralPath $path -Value "devstation" -NoNewline
        Test-FileHash256 -Path $path -Expected "0000" | Should -BeFalse
    }
}

Describe "Invoke-Download" {
    It "reessaie et reussit au troisieme essai" {
        Mock -ModuleName DevStation.Bootstrap Start-Sleep { }
        $script:calls = 0
        Mock -ModuleName DevStation.Bootstrap Invoke-WebRequest {
            $script:calls++
            if ($script:calls -lt 3) { throw "echec simule" }
        }

        Invoke-Download -Url "https://example.com/fichier" -OutFile (Join-Path $TestDrive "fichier")

        Should -Invoke -ModuleName DevStation.Bootstrap Invoke-WebRequest -Times 3 -Exactly
        Should -Invoke -ModuleName DevStation.Bootstrap Start-Sleep -Times 2 -Exactly
    }

    It "passe -ProxyUseDefaultCredentials avec -Proxy" {
        Mock -ModuleName DevStation.Bootstrap Invoke-WebRequest { }

        Invoke-Download -Url "https://example.com/fichier" -OutFile (Join-Path $TestDrive "fichier") -Proxy "http://proxy.example.com:8080"

        Should -Invoke -ModuleName DevStation.Bootstrap Invoke-WebRequest -Times 1 -Exactly -ParameterFilter {
            $Proxy -eq "http://proxy.example.com:8080" -and $ProxyUseDefaultCredentials
        }
    }
}

Describe "Install-DevStation" {
    It "supprime l'exe et echoue si l'empreinte est fausse" {
        $binDir = Join-Path $TestDrive "bin"
        Mock -ModuleName DevStation.Bootstrap Invoke-Download {
            param($Url, $OutFile, $Proxy)
            if ($Url -like "*checksums.txt") {
                Set-Content -LiteralPath $OutFile -Value "0000  devstation-windows-amd64.exe"
            }
            else {
                Set-Content -LiteralPath $OutFile -Value "binaire"
            }
        }

        { Install-DevStation -Version "1.2.3" -BinDir $binDir } | Should -Throw
        Test-Path -LiteralPath (Join-Path $binDir "devstation.exe") | Should -BeFalse
    }
}

Describe "Add-UserPath" {
    It "ajoute en tete quand absent" {
        Mock -ModuleName DevStation.Bootstrap Get-UserPathValue { 'C:\a;C:\b' }
        Mock -ModuleName DevStation.Bootstrap Set-UserPathValue { }

        $result = Add-UserPath -Dir "C:\x"

        $result | Should -BeTrue
        Should -Invoke -ModuleName DevStation.Bootstrap Set-UserPathValue -Times 1 -Exactly -ParameterFilter {
            $Value -eq 'C:\x;C:\a;C:\b'
        }
    }

    It "ne modifie rien si deja present (casse et antislash final differents)" {
        Mock -ModuleName DevStation.Bootstrap Get-UserPathValue { 'C:\a;c:\X\' }
        Mock -ModuleName DevStation.Bootstrap Set-UserPathValue { }

        $result = Add-UserPath -Dir "C:\x"

        $result | Should -BeFalse
        Should -Invoke -ModuleName DevStation.Bootstrap Set-UserPathValue -Times 0 -Exactly
    }

    It "ecrit la seule valeur quand le PATH utilisateur est vide" {
        Mock -ModuleName DevStation.Bootstrap Get-UserPathValue { '' }
        Mock -ModuleName DevStation.Bootstrap Set-UserPathValue { }

        $result = Add-UserPath -Dir "C:\x"

        $result | Should -BeTrue
        Should -Invoke -ModuleName DevStation.Bootstrap Set-UserPathValue -Times 1 -Exactly -ParameterFilter {
            $Value -eq 'C:\x'
        }
    }
}

Describe "Test-Prerequisites" {
    It "Ok faux si git est absent" {
        Mock -ModuleName DevStation.Bootstrap Get-Command {
            param($Name)
            if ($Name -eq "git") { return $null }
            return [pscustomobject]@{ Name = $Name }
        }

        $result = Test-Prerequisites
        $result.Ok | Should -BeFalse
    }

    It "Ok vrai avec un avertissement si WSL est absent" {
        Mock -ModuleName DevStation.Bootstrap Get-Command {
            param($Name)
            if ($Name -eq "wsl.exe") { return $null }
            return [pscustomobject]@{ Name = $Name }
        }

        $result = Test-Prerequisites
        $result.Ok | Should -BeTrue
        ($result.Messages -join " ") | Should -Match "Podman"
    }
}
