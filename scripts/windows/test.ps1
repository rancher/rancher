$ErrorActionPreference = 'Stop'

Import-Module -WarningAction Ignore -Name "$PSScriptRoot\utils.psm1"

Invoke-Script -File "$PSScriptRoot\version.ps1"

$DIR_PATH = Split-Path -Parent $MyInvocation.MyCommand.Definition
$SRC_PATH = (Resolve-Path "$DIR_PATH\..\..").Path
cd $SRC_PATH

$env:GOARCH=$env:ARCH
$env:GOOS='windows'
go test -v -cover -tags 'test' ./cmd/agent/...
if (-not $?) {
    Log-Fatal "go test failed"
}
