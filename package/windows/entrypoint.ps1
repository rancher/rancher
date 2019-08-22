<#
	entry.ps1 detects the first argument as command, and then
	select the corresponding execution logic
 #>

$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

function Run-PowerShell
{
    param (
        [parameter(Mandatory = $false)] [string[]]$ArgumentList
    )

    try {
        if ($ArgumentList) {
            Start-Process -NoNewWindow -Wait -FilePath "powershell" -ArgumentList $ArgumentList
        } else {
            Start-Process -NoNewWindow -Wait -FilePath "powershell"
        }
    } catch {
        $errMsg = $_.Exception.Message
        if ($errMsg -like "*This command cannot be run due to the error: *") {
            if ($ArgumentList) {
                Start-Process -NoNewWindow -Wait -FilePath "pwsh" -ArgumentList $ArgumentList
            } else {
                Start-Process -NoNewWindow -Wait -FilePath "pwsh"
            }
        } else {
            throw $_
        }
    }
}

$psArgs = $null
$componentScript = "$PSScriptRoot\$($args[0]).ps1"
if (-not (Test-Path -ErrorAction Ignore -Path $componentScript))
{
    if ($env:CATTLE_AGENT_CONNECT -eq "true")
    {
        $psArgs = @("-NoLogo", "-NonInteractive", "-File", "$PSScriptRoot\execute.ps1") + $args[0..$args.Length]
    }
}
else
{
    $psArgs = @("-NoLogo", "-NonInteractive", "-File", "$PSScriptRoot\$($args[0]).ps1") + $args[1..$args.Length]
}


if ($psArgs)
{
    Run-PowerShell -ArgumentList $psArgs
    exit 0
}

Run-PowerShell
