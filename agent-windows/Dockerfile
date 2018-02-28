FROM microsoft/nanoserver:10.0.14393.1593
ENV RANCHER_AGENT_WINDOWS_IMAGE rancher/agent-windows:v0.2.0-rc4
ENV TOOLS_LIST "devcon.exe,startup.ps1,cleanup.ps1,rancher-tools.psm1,crash-loop.ps1"
RUN mkdir "c:/program files/rancher"
VOLUME [ "c:/program files/rancher" ]
ADD [ "tools/devcon.exe","c:/program files/" ]
ADD [ "tools/startup.ps1","c:/program files/" ]
ADD [ "tools/cleanup.ps1","c:/program files/" ]
ADD [ "tools/run.ps1","c:/program files/" ]
ADD [ "tools/rancher-tools.psm1","c:/program files/" ]
ADD [ "tools/crash-loop.ps1","c:/program files/"]
WORKDIR "c:/program files/"
ENTRYPOINT [ "powershell","-file","c:/program files/run.ps1" ]