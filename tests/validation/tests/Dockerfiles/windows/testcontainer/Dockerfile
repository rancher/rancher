ARG SERVERCORE

FROM mcr.microsoft.com/windows/servercore:$SERVERCORE
SHELL ["powershell", "-NoLogo", "-Command", "$ErrorActionPreference = 'Stop'; $ProgressPreference = 'SilentlyContinue';"]
RUN Add-WindowsFeature Web-Server; \
    Invoke-WebRequest -UseBasicParsing -Uri "https://dotnetbinaries.blob.core.windows.net/servicemonitor/2.0.1.6/ServiceMonitor.exe" -OutFile "c:\svcm.exe"
COPY ["run.ps1", "c:/scripts/"]
EXPOSE 80
CMD ["powershell", "c:/scripts/run.ps1"]
