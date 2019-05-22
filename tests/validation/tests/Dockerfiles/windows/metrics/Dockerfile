ARG SERVERCORE

FROM mcr.microsoft.com/windows/servercore:$SERVERCORE as builder
MAINTAINER logan "https://github.com/loganhz"
SHELL ["powershell", "-Command", "$ErrorActionPreference = 'Stop'; $ProgressPreference = 'SilentlyContinue';"]
ENV NODE_VERSION 10.16.0
RUN $URL = ('https://nodejs.org/dist/v{0}/node-v{0}-win-x64.zip' -f $env:NODE_VERSION); \
    \
    Write-Host ('Downloading Nodejs from {0} ...' -f $URL); \
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; \
    Invoke-WebRequest -UseBasicParsing -OutFile c:\node.zip -Uri $URL; \
    \
    Write-Host 'Expanding ...'; \
    Expand-Archive -Force -Path c:\node.zip -DestinationPath c:\; \
    Rename-Item -Path $('c:\node-v{0}-win-x64' -f $env:NODE_VERSION) -NewName 'c:\nodejs'; \
    \
    Write-Host 'Cleaning ...'; \
    Remove-Item -Force -Recurse -Path c:\node.zip; \
    \
    Write-Host 'Complete.'

FROM mcr.microsoft.com/powershell:nanoserver-$SERVERCORE
SHELL ["pwsh", "-Command", "$ErrorActionPreference = 'Stop'; $ProgressPreference = 'SilentlyContinue';"]
COPY --from=builder /nodejs /nodejs
COPY ["app.js", "c:/web/"]
EXPOSE 8080
WORKDIR /web
CMD Start-Process -NoNewWindow -Wait -FilePath c:/nodejs/node.exe -ArgumentList c:/web/app.js
