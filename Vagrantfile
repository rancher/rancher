# -*- mode: ruby -*-
# vi: set ft=ruby :

require_relative 'vagrant_rancheros_guest_plugin.rb'

# To enable rsync folder share change to false
$rancher_ui_port = 8080
$vb_gui = false
$rsync_folder_disabled = true
$number_of_nodes = 1
$vm_mem = "1024"
$host_ip = "172.19.8.100"


# All Vagrant configuration is done below. The "2" in Vagrant.configure
# configures the configuration version (we support older styles for
# backwards compatibility). Please don't change it unless you know what
# you're doing.
Vagrant.configure(2) do |config|
  config.vm.box   = "rancherio/rancheros"
  config.vm.box_version = ">=0.1.2"


  config.vm.define "rancher" do |config|
    config.vm.provider "virtualbox" do |vb|
        vb.memory = $vm_mem
        vb.gui = $vb_gui
    end

    config.vm.network "private_network", ip: $host_ip

    # Disabling compression because OS X has an ancient version of rsync installed.
    # Add -z or remove rsync__args below if you have a newer version of rsync on your machine.
    config.vm.synced_folder ".", "/opt/rancher", type: "rsync",
        rsync__exclude: ".git/", rsync__args: ["--verbose", "--archive", "--delete", "--copy-links"],
        disabled: $rsync_folder_disabled


    config.vm.provision :shell,
        :inline => "docker run -d -p 8080:8080 rancher/server",
        :privileged => true

    config.vm.provision :shell,
        :inline => "docker run -e CATTLE_AGENT_IP=%s -e WAIT=true -v /var/run/docker.sock:/var/run/docker.sock rancher/agent http://localhost:8080" % $host_ip,
        :privileged => true
  end
end
