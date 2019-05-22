ARG SERVERCORE

FROM mcr.microsoft.com/windows/servercore:$SERVERCORE
MAINTAINER frank "https://github.com/thxcode"
SHELL ["powershell", "-NoLogo", "-Command", "$ErrorActionPreference = 'Stop'; $ProgressPreference = 'SilentlyContinue';"]
ENV NGINX_WORKDIR "c:\\Program Files\\nginx"
ENV NGINX_VERSION "1.16.0"
RUN [Environment]::SetEnvironmentVariable('PATH', ('{0};{1}' -f $env:NGINX_WORKDIR, $env:PATH), [EnvironmentVariableTarget]::Machine); \
    Invoke-WebRequest -Uri ('https://nginx.org/download/nginx-{0}.zip' -f $env:NGINX_VERSION) -OutFile nginx.zip; \
	Expand-Archive nginx.zip -DestinationPath $env:ProgramFiles; \
	Remove-Item -Force nginx.zip; \
	Move-Item $env:ProgramFiles\nginx-* $env:NGINX_WORKDIR
EXPOSE 80 443
WORKDIR $NGINX_WORKDIR
CMD Start-Process -NoNewWindow -FilePath nginx.exe ; \
	Add-Content logs\access.log 'nginx started...' ; \
	Get-Content -Wait logs\access.log
