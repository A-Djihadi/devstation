Describe 'Sandbox PowerShell' {
    It 'exécute Pester' {
        $PSVersionTable.PSVersion.Major | Should -BeGreaterOrEqual 7
    }
}
