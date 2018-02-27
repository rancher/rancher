# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure(2) do |config|
  config.vm.box = "bento/ubuntu-16.04"

  config.vm.synced_folder ".", "/go/src/github.com/containernetworking/cni"

  config.vm.provision "shell", inline: <<-SHELL
    set -e -x -u

    apt-get update -y || (sleep 40 && apt-get update -y)
    apt-get install -y golang git
    echo "export GOPATH=/go" >> /root/.bashrc
    export GOPATH=/go
    go get github.com/tools/godep
    cd /go/src/github.com/containernetworking/cni
    /go/bin/godep restore
  SHELL
end
