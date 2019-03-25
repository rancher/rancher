#Requires -Version 5.0
$ErrorActionPreference = "Stop"

$DIRTY = ""
if ("$(git status --porcelain --untracked-files=no)") {
    $DIRTY = "-dirty"
}

$COMMIT = (git rev-parse --short HEAD)
$GIT_TAG = $env:DRONE_TAG
if (-not $GIT_TAG) {
    $GIT_TAG = $(git tag -l --contains HEAD | Select-Object -First 1)
}

$VERSION = "${COMMIT}${DIRTY}"
if ((-not $DIRTY) -and ($GIT_TAG)) {
    $VERSION = "${GIT_TAG}"
}
$env:VERSION = $VERSION

$ARCH = $env:ARCH
if (-not $ARCH) {
    $ARCH = "amd64"
}
$env:ARCH = $ARCH

Write-Host "ARCH: $ARCH"
Write-Host "VERSION: $VERSION"