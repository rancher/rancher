#Requires -Version 5.0
$ErrorActionPreference = "Stop"

& "$PSScriptRoot\version.ps1"

$dirPath = Split-Path -Parent $MyInvocation.MyCommand.Definition
$rootPath = (Resolve-Path "$dirPath\..\..").Path
$version = $env:VERSION
$psVersion = $env:PS_VERSION
$imageTag = "rancher/rancher-agent:$version"

pushd $rootPath
Write-Host "Running agent build"
if ($version.endswith("-1803")) {
    docker build `
        --build-arg PS_VERSION=$psVersion `
        --build-arg VERSION=$version `
        -t $imageTag `
        -f package\windows\Dockerfile.agent .
} else {
    docker build `
        --isolation hyperv `
        --build-arg PS_VERSION=$psVersion `
        --build-arg VERSION=$version `
        -t $imageTag `
        -f package\windows\Dockerfile.agent .
}
if ($?) {
    Write-Host "Built $imageTag"
} else {
    throw "Build $imageTag FAILED"
}
popd
