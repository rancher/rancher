#Requires -Version 5.0
$ErrorActionPreference = "Stop"

$dirPath = Split-Path -Parent $MyInvocation.MyCommand.Definition
$rootPath = (Resolve-Path "$dirPath\..\..").Path

pushd $rootPath
Write-Host "Running validation"
$packages = Get-ChildItem -Recurse -Force -Include "*.go" | % Fullname | Resolve-Path -Relative | Select-String -NotMatch -Pattern "(^\.$|.git|.trash-cache|vendor|bin)" | % { $r = $_ -Split "\\"; if ($r.Count -gt 2) {$r[1]} } | Sort-Object -Unique | % { ('./{0}/...' -f $_)}

Write-Host "go vet"
go vet $packages

Write-Host "golint"
try {
    go get -u golang.org/x/lint/golint | Out-Null
    foreach ($pkg in $packages) {
        $lintResult = golint $pkg | Select-String -NotMatch -Pattern "hyperkube|should have comment.*or be unexported"
        if (-not $lintResult) {
            throw $lintResult
        }
    }
} catch {}

Write-Host "go fmt"
$fmtResult = go fmt $packages
if (-not $fmtResult) {
    throw $fmtResult
}

popd
