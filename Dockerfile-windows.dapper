FROM golang:1.12-windowsservercore
SHELL ["powershell", "-NoLogo", "-Command", "$ErrorActionPreference = 'Stop'; $ProgressPreference = 'SilentlyContinue';"]

ARG DAPPER_HOST_ARCH
ENV HOST_ARCH=${DAPPER_HOST_ARCH} ARCH=${DAPPER_HOST_ARCH}

RUN pushd c:\; \
    $URL = 'https://github.com/dscharrer/innoextract/releases/download/1.7/innoextract-1.7-windows.zip'; \
    \
    Write-Host ('Downloading innoextract from {0} ...' -f $URL); \
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; \
    Invoke-WebRequest -UseBasicParsing -OutFile c:\innoextract.zip -Uri $URL; \
    \
    Write-Host 'Expanding ...'; \
    Expand-Archive -Path c:\innoextract.zip -DestinationPath c:\innoextract\.; \
    \
    Write-Host 'Cleaning ...'; \
    Remove-Item -Force -Recurse -Path c:\innoextract.zip; \
    \
    Write-Host 'Updating PATH ...'; \
    [Environment]::SetEnvironmentVariable('PATH', ('c:\innoextract\;{0}' -f $env:PATH), [EnvironmentVariableTarget]::Machine); \
    \
    Write-Host 'Complete.'; \
    popd;

RUN pushd c:\; \
    $URL = 'https://github.com/docker/toolbox/releases/download/v18.09.3/DockerToolbox-18.09.3.exe'; \
    \
    Write-Host ('Downloading docker from {0} ...' -f $URL); \
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; \
    Invoke-WebRequest -UseBasicParsing -OutFile c:\dockertoolbox.exe -Uri $URL; \
    \
    Write-Host 'Expanding ...'; \
    innoextract c:\dockertoolbox.exe; \
    \
    Write-Host 'Cleaning ...'; \
    Remove-Item -Force -Recurse -Path @('c:\dockertoolbox.exe', 'c:\app\*') -Exclude @('docker.exe'); \
    \
    Write-Host 'Updating PATH ...'; \
    [Environment]::SetEnvironmentVariable('PATH', ('c:\app\;{0}' -f $env:PATH), [EnvironmentVariableTarget]::Machine); \
    \
    Write-Host 'Complete.'; \
    popd;

RUN pushd c:\; \
    $URL = 'https://github.com/golangci/golangci-lint/releases/download/v1.16.0/golangci-lint-1.16.0-windows-amd64.zip'; \
    \
    Write-Host ('Downloading golangci from {0} ...' -f $URL); \
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; \
    Invoke-WebRequest -UseBasicParsing -OutFile c:\golangci-lint.zip -Uri $URL; \
    \
    Write-Host 'Expanding ...'; \
    Expand-Archive -Path c:\golangci-lint.zip -DestinationPath c:\; \
    \
    Write-Host 'Cleaning ...'; \
    Remove-Item -Force -Recurse -Path c:\golangci-lint.zip; \
    \
    Write-Host 'Updating PATH ...'; \
    [Environment]::SetEnvironmentVariable('PATH', ('c:\golangci-lint-1.16.0-windows-amd64\;{0}' -f $env:PATH), [EnvironmentVariableTarget]::Machine); \
    \
    Write-Host 'Complete.'; \
    popd;

# upgrade git
RUN pushd c:\; \
    $URL = 'https://github.com/git-for-windows/git/releases/download/v2.21.0.windows.1/MinGit-2.21.0-64-bit.zip'; \
    \
    Write-Host ('Downloading git from {0} ...' -f $URL); \
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; \
    Invoke-WebRequest -UseBasicParsing -OutFile c:\git.zip -Uri $URL; \
    \
    Write-Host 'Expanding ...'; \
    Expand-Archive -Force -Path c:\git.zip -DestinationPath c:\git\.; \
    \
    Write-Host 'Cleaning ...'; \
    Remove-Item -Force -Recurse -Path c:\git.zip; \
    \
    Write-Host 'Complete.'; \
    popd;

ENV DAPPER_ENV REPO TAG DRONE_TAG
ENV DAPPER_SOURCE /gopath/src/github.com/rancher/rancher
ENV DAPPER_OUTPUT ./bin
ENV DAPPER_DOCKER_SOCKET true
ENV TRASH_CACHE ${DAPPER_SOURCE}/.trash-cache
ENV HOME ${DAPPER_SOURCE}

WORKDIR ${DAPPER_SOURCE}
ENTRYPOINT ["powershell", "-NoLogo", "-NonInteractive", "-File", "./scripts/windows/entry.ps1"]
CMD ["ci"]
