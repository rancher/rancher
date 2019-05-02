ARG SERVERCORE_VERSION

FROM mcr.microsoft.com/powershell:nanoserver-${SERVERCORE_VERSION}
SHELL ["pwsh", "-NoLogo", "-Command", "$ErrorActionPreference = 'Stop'; $ProgressPreference = 'SilentlyContinue';"]

ARG ARCH=amd64
RUN $URL = 'http://nginx.org/download/nginx-1.15.9.zip'; \
    $SRC_PATH = ('{0}\nginx.zip' -f $env:ProgramFiles); \
    $DST_PATH = ('{0}\nginx' -f $env:ProgramFiles); \
    \
    $null = New-Item -Type Directory -Path $DST_PATH -ErrorAction Ignore; \
    \
    Write-Host ('Downloading Nginx from {0}...'  -f $URL); \
    \
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; \
    Invoke-WebRequest -UseBasicParsing -OutFile $SRC_PATH -Uri $URL; \
    \
    Write-Host 'Expanding ...'; \
    \
    Expand-Archive -Force -Path $SRC_PATH -DestinationPath $DST_PATH; \
    \
    Write-Host 'Cleaning ...'; \
    \
    Copy-Item -Force -Path ('{0}\nginx-1.15.9\nginx.exe' -f $DST_PATH) -Destination ('{0}\nginx.exe' -f $DST_PATH); \
    Remove-Item -Force -Recurse -Path @($SRC_PATH, ('{0}\nginx-1.15.9' -f $DST_PATH)); \
    \
    Write-Host 'Complete.';
RUN $URL = 'https://github.com/Microsoft/K8s-Storage-Plugins/releases/download/v0.0.2/flexvolume-windows.zip'; \
    $SRC_PATH = ('{0}\flexvolume.zip' -f $env:ProgramFiles); \
    $DST_PATH = ('{0}\kubelet\volumeplugins' -f $env:ProgramFiles); \
    \
    $null = New-Item -Type Directory -Path $DST_PATH -ErrorAction Ignore; \
    \
    Write-Host ('Downloading Volume Plugins from {0} ...' -f $URL); \
    \
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; \
    Invoke-WebRequest -UseBasicParsing -OutFile $SRC_PATH -Uri $URL; \
    \
    Write-Host 'Expanding ...'; \
    \
    Expand-Archive -Force -Path $SRC_PATH -DestinationPath $DST_PATH; \
    \
    Write-Host 'Cleaning ...'; \
    \
    Remove-Item -Force -Recurse -Path $SRC_PATH; \
    \
    Write-Host 'Complete.';

ARG VERSION=dev
ENV AGENT_IMAGE rancher/rancher-agent:${VERSION}

COPY ["agent.exe", "*.ps1", "*.psm1", "C:/Program Files/rancher/"]
WORKDIR "C:\\Program Files\\rancher"
ENTRYPOINT ["pwsh", "-NoLogo", "-NonInteractive", "-File", "./start.ps1"]
