$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

function Log-Info
{
    Write-Host -NoNewline -ForegroundColor Blue "INFO: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($Args -join " "))
}

function Log-Warn
{
    Write-Host -NoNewline -ForegroundColor DarkYellow "WARN: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
}

function Log-Error
{
    Write-Host -NoNewline -ForegroundColor DarkRed "ERRO "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
}


function Log-Fatal
{
    Write-Host -NoNewline -ForegroundColor DarkRed "FATA: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))

    throw "EXITED"
}

function Invoke-Script
{
    param (
        [parameter(Mandatory = $true)] [string]$File
    )

    try {
        Invoke-Expression -Command $File
        if (-not $?) {
            throw "CALLFAILED"
        }
    } catch {
        $Msg = $_.Exception.Message
        if ($Msg -eq "CALLFAILED") {
            Log-Fatal "Failed to invoke $File"
        } else {
            Log-Fatal "Could not invoke $File, $Msg"
        }
    }
}

Export-ModuleMember -Function Log-Info
Export-ModuleMember -Function Log-Warn
Export-ModuleMember -Function Log-Error
Export-ModuleMember -Function Log-Fatal
Export-ModuleMember -Function Invoke-Script
