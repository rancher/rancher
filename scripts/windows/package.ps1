#Requires -Version 5.0
$ErrorActionPreference = "Stop"

Invoke-Expression -Command "$PSScriptRoot\version.ps1"

$DIR_PATH = Split-Path -Parent $MyInvocation.MyCommand.Definition
$SRC_PATH = (Resolve-Path "$DIR_PATH\..\..").Path
cd $SRC_PATH\package\windows

$TAG = $env:TAG
if (-not $TAG) {
    $TAG = ('{0}{1}' -f $env:VERSION, $env:SUFFIX)
}
$REPO = $env:REPO
if (-not $REPO) {
    $REPO = "rancher"
}

if ($TAG | Select-String -Pattern 'dirty') {
    $TAG = "dev"
}

if ($env:DRONE_TAG) {
    $TAG = $env:DRONE_TAG
}

$null = Copy-Item -Force -Path "$SRC_PATH\bin\agent.exe" -Destination .

# Get release id as image tag suffix
$HOST_RELEASE_ID = (Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\' -ErrorAction Ignore).ReleaseId
$RELEASE_ID = $env:RELEASE_ID
if (-not $RELEASE_ID) {
    $RELEASE_ID = $HOST_RELEASE_ID
}
$AGENT_IMAGE = ('{0}/rancher-agent:{1}-windows-{2}' -f $REPO, $TAG, $RELEASE_ID)

$ARCH = $env:ARCH
if ($RELEASE_ID -eq $HOST_RELEASE_ID) {
    docker build `
        --build-arg SERVERCORE_VERSION=$RELEASE_ID `
        --build-arg ARCH=$ARCH `
        --build-arg VERSION=$TAG `
        -t $AGENT_IMAGE `
        -f Dockerfile.agent .
} else {
    docker build `
        --isolation hyperv `
        --build-arg SERVERCORE_VERSION=$RELEASE_ID `
        --build-arg ARCH=$ARCH `
        --build-arg VERSION=$TAG `
        -t $AGENT_IMAGE `
        -f Dockerfile.agent .
}

# $AGENT_IMAGE | Out-File -Encoding ascii -Force -FilePath "$SRC_PATH\dist\images"
Write-Host "Built $AGENT_IMAGE`n"
