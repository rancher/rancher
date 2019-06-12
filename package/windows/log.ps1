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

Import-Module -Force -Name "C:\etc\rancher\tool.psm1"

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
                    Log-Error $item.Message
                }
                "Warning" {
                    Log-Warn $item.Message
                }
                "Information" {
                    Log-Info $item.Message
                }
            }
        }

        $LastIdx = $newestIdx
    } catch {}

    Start-Sleep -s 1
}