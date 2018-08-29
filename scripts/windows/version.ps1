#Requires -Version 5.0
$ErrorActionPreference = "Stop"

Write-Host "Running version getting"

$psVersion = "6.0.2-nanoserver-1709"
$versionSuffix = "nanoserver-1709"
$currentVersion = (Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\')
switch ($currentVersion.ReleaseId) {
    "1803" {
        $psVersion = "6.0.2-nanoserver-1803"
        $versionSuffix = "nanoserver-1803"
    }
}

$dirty = ""
if ("$(git status --porcelain --untracked-files=no)") {
    $dirty = "-dirty"
}

$commitHash = (git rev-parse --short HEAD)
$gitTag = $env:DRONE_TAG
if (-not $gitTag) {
    $gitTag = $(git tag -l --contains HEAD | Select-Object -First 1)
}

$version = "${commitHash}${dirty}-${versionSuffix}"
if ((-not $dirty) -and ($gitTag)) {
    $version = "${gitTag}-${versionSuffix}"
}

Write-Host "PS_VERSION is $psVersion"
Write-Host "VERSION is $version"

$env:PS_VERSION = $psVersion
$env:VERSION = $version