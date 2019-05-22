#Requires -Version 5.0
$ErrorActionPreference = "Stop"

$STATIC_CONTENT_PATH = "c:\inetpub\wwwroot"
$CONTAINER_NAME = $env:CONTAINER_NAME
if (-not $CONTAINER_NAME) {
    $CONTAINER_NAME = hostname
}
$CONTAINER_NAME | Out-File -Encoding ascii -Force -FilePath "$STATIC_CONTENT_PATH\name.html"
$CONTAINER_NAME | Out-File -Encoding ascii -Force -FilePath "$STATIC_CONTENT_PATH\service1.html"
$CONTAINER_NAME | Out-File -Encoding ascii -Force -FilePath "$STATIC_CONTENT_PATH\service2.html"

Start-Process -NoNewWindow -Wait -FilePath $Args[0] -ArgumentList $Args[1..$Args.Length]
