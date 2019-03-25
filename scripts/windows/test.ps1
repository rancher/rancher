#Requires -Version 5.0
$ErrorActionPreference = "Stop"

Invoke-Expression -Command "$PSScriptRoot\version.ps1"

$DIR_PATH = Split-Path -Parent $MyInvocation.MyCommand.Definition
$SRC_PATH = (Resolve-Path "$DIR_PATH\..\..").Path
cd $SRC_PATH

$env:GOARCH=$env:ARCH
$env:GOOS='windows'
go test -v -cover -tags 'test' ./pkg/agent/...
if (-not $?) {
    throw "go test failed"
}
