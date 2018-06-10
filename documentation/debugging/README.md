# How to set up a debugging environment for Rancher 2.x
It took me some time to find out, how a good debugging workflow for rancher 2.x would look like.

I had several problems: At first I tried to create a debug session with Visual Studio Code: I had lots of performance issues with that.

Also I had the problem that after the first successful connection to delve, it was unresponsive, after I restarted the debug session. I had to write scripts which kill delve and the rancher process before a new delve process could be started. And so on...

But this process here should work rather good. Give me feedback, if you find errors or have improvement tips for me, please.

## Content
1 [Prerequisites](#prerequisites)<br>
2 [Install GOLAND IDE](#install-goland-ide)<br>
3 [Create build settings and build with GOLAND](#create-build-settings-and-build-rancher-with-goland)<br>
4 [Create debug container](#create-debug-container)<br>
5 [Configure GOLAND for remote debugging](#configure-goland-for-remote-debugging)<br>
6 [Remote debug Rancher 2.x](#remote-debug-rancher-2.x-in-goland)<br>
7 [Watch stdout output of Rancher 2.x](#watch-stdout-output-of-rancher-2.x)<br>
8 [Optional: Make new docker debug image](#optional:-make-a-new-docker-debug-image)

## Prerequisites
* Virtual machine with Ubuntu 18.04 desktop
    * Virtualbox
    * as much as CPU cores and RAM you can spend for it (I love my AMD Ryzen system for that)
    * Harddisk 40 GByte 
    * Network Interface set to bridged mode
    * Ubuntu-18.04-desktop-amd64 ISO image
* Docker 
  * At the time of this writing Docker is not available as stable for Ubuntu 18.04
  * Install docker from the Ubuntu repository instead:
    
      ```bash
    sudo apt-get update
    sudo apt-get install docker.io
    docker -v
    ```
  * The docker version should be around 17.12.x. Compilation of  some rancher projects will ned higher versions of Docker because they use multi stage Dockerfiles
  * Add your user to the docker group:

      ```bash
      sudo usermod -aG docker <your username>
      ```

    Logout / Login afterwards
  * Test docker:
    
      ```bash
      docker images
      ```
  * The output should give no errors but an empty list of docker images.
* Git

    ```bash
    sudo apt-get install git
    ```

* Go compiler
  * Download and install it like described here:<br>
    https://golang.org/dl/
  * Logout / Login afterwards
  * Test go:

      ```bash
      go -v
      ```
  * You should see Version _1.10.3_ or higher.
  * Set _GOPATH_ and _GOBIN_ environment variables in your _.profile_ file to...:

      ```bash
      export GOPATH=/go
      export GOBIN=/go/bin
      ```
     
      Reason for not setting _GOPATH_ to your home directory: <br>
      Later we will debug your Rancher binary in a docker container. The _GOPATH_ in the docker container and the _GOPATH_ of the environment outside of it should be the same to get the debugger working.
  * Create directories /go and /go/bin:

      ```bash
      sudo mkdir -p /go/bin
      sudo chown -R <username>:<groupname> /go
      ```

      To find out your username and groupname type:

          ```bash
          id -un   # prints your user name
          id -gn   # prints your group name
          ```

  * Add /go/bin to your _PATH_ environment variable in your _~/.profile_ file.

      ```bash
      export PATH=$PATH:/go/bin
      ```

      Logout / Login afterwards
* Delve debugger
  * Install delve debugger:

      ```bash
      go get -u github.com/derekparker/delve/cmd/dlv
      ```
  * Test debugger:

      ```bash
      dlv version
      ```

      You should see a version _1.0.0_ or higher.
* Install rancher sources:
  * Create directory structure for rancher in your _GOPATH_:

      ```bash
      mkdir -p $GOPATH/src/github.com/rancher
      ```

  * Clone rancher's Git repository:

      ```bash
      cd $GOPATH/src/github.com/rancher
      git clone https://github.com/rancher/rancher.git
      ```

  * Build rancher on the command line interface:

      ```bash
      cd $GOPATH/src/github.com/rancher/rancher
      make
      ```

      This will take a few minutes.

## Install _GOLAND_ IDE

### Why not using Visual Studio Code?
At first I tried to set up a build environment with _Visual Studio Code_. I was able to compile and debug but for some reason (the _GO_ plugin of VSCode maybe?) the debugging experience was not good. _Rancher_ is a rather big go project and I often had problems with _VSCode_ getting very slow.

I switched to the [_Goland_](https://www.jetbrains.com/go/) development environment therefore. It also is the only supported debugger by the Rancher dev team.

### Download, install and start _GOLAND_

* Download _GOLAND_ from https://www.jetbrains.com/go/
* (In my case I got _GOLAND_ Version _2018.1.4_)
* Untar it somewhere:

    ```bash
    tar xvf Goland-...tar.gz
    cd Goland-.../bin
    ./goland.sh
    ```
* Press **Open Project**
* Navigate to the directory _/go/src/github.com/rancher/rancher_ and press **Ok**

## Create build settings and build rancher with _GOLAND_
* File -> Settings -> Tools -> File Watchers : Press the **+** button :
  * Name: **Auto compile on Save**
  * Files to Watch -> Filetype: **Go**
  * Files to Watch -> Scope: **All places**
  * Tool to Run on Changes -> Program: **go**
  * Tool to Run on Changes -> Arguments: **build -gcflags "all=-N -l" -tags k8s -o debug**
  * Tool to Run on Changes -> Output paths to refresh: 
  * Tool to Run on Changes -> Working directory: **/go/src/github.com/rancher/rancher**
  * Tool to Run on Changes -> Environment variables: <empty>
  * Keep all other settings on their defaults and press **OK** button.
* Press **OK** button in File -> Settings dialog.
* Open the file **main.go** in the root folder of **$GOPATH/src/github.com/rancher/rancher** and modify it a little bit (enter a space somewhere) and save the file.<br>You should see Goland starting the **Auto compile on Save** task because the files has changed. After a while the compilation ends and there should be the binary 

    ```bash
    $GOPATH/src/github.com/rancher/rancher/debug
    ```

## Create debug container 

* Create debug container:

    ```bash
    cd $GOPATH/src/github.com/rancher/rancher
    ./scripts/package-debugger.sh
    ```

    The result is a docker image named _rancher/rancher:debug_

    GOLAND will create a container from this image at the start of each debug session and volume binds the binary _debug_ we created earlier into this container. Then the debugger _delve_ will be started and _GOLAND_ remote controls it.

## Configure _GOLAND_ for remote debugging

Perform this steps to create a debug configuration in _GOLAND_:

* Run -> Edit Configurations... -> + (Add New Configuration) -> Go Remote:
    * Name: **Remote Debug Rancher**
    * Host: **<IP Address of your Debug Container, in my case 172.17.0.1>**
    * Before launch: Activate tool windows: Press + button (Add) -> Run External tool -> Press + (Add)
        * Tool Settings -> Name: **Start debug container**
        * Tool Settings -> Program: **/go/src/github.com/rancher/rancher/scripts/start-debug-container.sh**
        * Tool Settings -> Working directory: **/go/src/github.com/rancher/rancher/scripts**
        * Press **OK** button
    * External Tools: Press **OK** button
* Run/Debug Configurations: Press **OK** button

## Remote debug rancher 2.x in _GOLAND_
* Select the configuration _Remote Debug Rancher_
* Set a breakpoint in the code (e.g. the first line in _main.go:main()_ )
* Press the debug button ...
  * if previously a debug container was started, the debugger in it will be killed and the container will be removed.
  * a new debug container will be started in the background
  * delve will be started and load the volume binded binary **_debug_**
  * _GOLAND_ connects to the delve debugger in the container and remote controls it
* If you change code, save the file and in the background the compilation task will automatically start, creating a new version of the binary **_debug_**
* Restart the debugger and the new binary will be debugged.

## Watch stdout output of rancher 2.x 
If you want to see the log messages of rancher, you must call:

```bash
docker logs debugger
```

## Optional: Make a new Docker debug image
* Make your changes
* Build a new version of the docker debug image:

    ```bash
    ./scripts/package-debugger.sh
    ```


