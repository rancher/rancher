#Requires -Version 5.0

param (
    [parameter(Mandatory = $false)] [switch]$Latest10Mins = $false,
    [parameter(Mandatory = $false)] [int]$LatestMins = 0
)

$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

$SVCRancherAgtName = "rancher-agent"
$LastIdx = 0
try {
    if ($Latest10Mins) {
        $datetime = (Get-Date).AddMinutes(-10)
        $LastIdx = (Get-EventLog -LogName Application -Source $SVCRancherAgtName -After $datetime | Sort-Object Index | Select-Object -First 1).Index
    } elseif ($LatestMins -ne 0) {
        $datetime = (Get-Date).AddMinutes(-$LatestMins)
        $LastIdx = (Get-EventLog -LogName Application -Source $SVCRancherAgtName -After $datetime | Sort-Object Index | Select-Object -First 1).Index
    } else {
        $LastIdx = (Get-EventLog -LogName Application -Source $SVCRancherAgtName -Newest 1).Index
    }
} catch {}

while($true) {
    try {
        $newestIdx = (Get-EventLog -LogName Application -Source $SVCRancherAgtName -Newest 1).Index

        Get-EventLog -LogName Application -Source $SVCRancherAgtName -Newest ($newestIdx - $LastIdx) | Sort-Object Index | % {
            $item = $_

            if ($item.Index -le $LastIdx) {
                return
            }

            switch ($item.EntryType) {
                "Error" {
                    Write-Host -NoNewline -ForegroundColor DarkRed "ERRO"
                    Write-Host -NoNewline -ForegroundColor Gray ("[{0}] " -f $item.Index)
                    Write-Host -ForegroundColor Gray $item.Message
                    break
                }
                "Warning" {
                    Write-Host -NoNewline -ForegroundColor DarkYellow "WARN"
                    Write-Host -NoNewline -ForegroundColor Gray ("[{0}] " -f $item.Index)
                    Write-Host -ForegroundColor Gray $item.Message
                    break
                }
                "Information" {
                    Write-Host -NoNewline -ForegroundColor DarkBlue "INFO"
                    Write-Host -NoNewline -ForegroundColor Gray ("[{0}] " -f $item.Index)
                    Write-Host -ForegroundColor Gray $item.Message
                }
            }
        }

        $LastIdx = $newestIdx
    } catch {}

    Start-Sleep -s 5
}