#Requires -Version 5.0

param (
    [parameter(Mandatory = $false)] [string]$PushImageToLibrary = "maiwj"
)

$ErrorActionPreference = "Stop"

& "$PSScriptRoot\version.ps1" | Out-Null

$baseTag = "rancher-agent:$($env:VERSION)"
$currentTag = "rancher/$baseTag"
$pushTag = "$PushImageToLibrary/$baseTag"

$currentReleaseId = (docker images $currentTag --format "{{.ID}}")
$pushedReleaseId = (docker images $pushTag --format "{{.ID}}")
if ($currentReleaseId -ne $pushedReleaseId) {
    docker tag $pushTag "$pushTag-bak" | Out-Null
    docker tag $currentTag $pushTag | Out-Null
}

docker push $pushTag
if ($?) {
    docker rmi "$pushTag-bak" | Out-Null
    docker rmi $currentTag | Out-Null
    Write-Host "$pushTag was PUSHED"
} else {
    docker tag "$pushTag-bak" $pushTag | Out-Null
    docker rmi "$pushTag-bak" | Out-Null
    Write-Host -ForegroundColor Red "$pushTag has something wrong while PUSHING"
}
