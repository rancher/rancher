# Building Rancher Windows Bootstrap Agent Image

The Rancher Windows Bootstrap Agent is for collecting tools binaries from Rancher Server and copying them into the host.

There are two ways to build this image. But both of them will need a Windows Server 2016 host with Docker engine(>= 1.12.2).

* Build it from *nix
  - Need Docker(>= 1.12.6)
  - Docker engine in Windows Server needs to expose port 2375(Set `"hosts":["tcp://0.0.0.0:2375","npipe://"]` in daemon.json)
  - Execute `DOCKER_TLS_VERIFY= DOCKER_HOST=tcp://<Server_IP>:2375 ./build-image.sh`
* Build it in Windows Server 2016
  - Execute `./build-images.ps1` in Powershell

And then, this image will be built in the Windows Server host.