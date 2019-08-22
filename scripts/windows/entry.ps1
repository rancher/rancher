$ErrorActionPreference = 'Stop'

$SCRIPT_PATH = ("{0}\{1}.ps1" -f $PSScriptRoot, $Args[0])
if (Test-Path $SCRIPT_PATH -ErrorAction Ignore)
{
    Invoke-Expression -Command $SCRIPT_PATH
    if (-not $?) {
        exit 255
    }
    exit 0
}

Start-Process -Wait -FilePath $Args[0] -ArgumentList $Args[1..$Args.Length]
