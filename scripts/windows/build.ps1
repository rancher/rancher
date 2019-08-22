$ErrorActionPreference = 'Stop'

Import-Module -WarningAction Ignore -Name "$PSScriptRoot\utils.psm1"

Log-Info "Running: build-agent"
Invoke-Script -File "$PSScriptRoot\build-agent.ps1"
