#Requires -Version 5.0
$ErrorActionPreference = "Stop"

trap {
    Write-Host -NoNewline -ForegroundColor Red "[ERROR]: "
    Write-Host -ForegroundColor Red "$_"

    popd
    exit 1
}

& "$PSScriptRoot\ci.ps1"