FROM microsoft/nanoserver:10.0.14393.1593
ENV RANCHER_AGENT_WINDOWS_IMAGE rancher/agent-windows:v0.1.0
RUN mkdir "c:/program files/rancher"
VOLUME [ "c:/program files/rancher" ]
ADD [ "devcon.exe","c:/program files/" ]
ADD [ "run.ps1","c:/program files/" ]
WORKDIR "c:/program files/"
ENTRYPOINT [ "powershell","-file","c:/program files/run.ps1" ]