# -*- mode: ruby -*-
# # vi: set ft=ruby :

# This is fork of https://github.com/coreos/coreos-vagrant

require 'fileutils'

Vagrant.require_version ">= 1.6.0"

CONFIG = File.join(File.dirname(__FILE__), "vagrant", "config.rb")

# Defaults for config options defined in CONFIG
$update_channel = "alpha"
$expose_rancher_ui = 8080
$vb_gui = false
$vb_memory = 1024
$vb_cpus = 1
$private_ip = "172.17.8.100"

if File.exist?(CONFIG)
  require CONFIG
end

Vagrant.configure("2") do |config|
  config.vm.box = "coreos-%s" % $update_channel
  config.vm.box_version = ">= 308.0.1"
  config.vm.box_url = "http://%s.release.core-os.net/amd64-usr/current/coreos_production_vagrant.json" % $update_channel

  config.vm.provider :vmware_fusion do |vb, override|
    override.vm.box_url = "http://%s.release.core-os.net/amd64-usr/current/coreos_production_vagrant_vmware_fusion.json" % $update_channel
  end

  config.vm.provider :virtualbox do |v|
    # On VirtualBox, we don't have guest additions or a functional vboxsf
    # in CoreOS, so tell Vagrant that so it can be smarter.
    v.check_guest_additions = false
    v.functional_vboxsf     = false
  end

  # plugin conflict
  if Vagrant.has_plugin?("vagrant-vbguest") then
    config.vbguest.auto_update = false
  end

  config.vm.define vm_name = "rancher" do |config|
    config.vm.hostname = vm_name

    config.vm.network "forwarded_port", guest: 8080, host: $expose_rancher_ui, auto_correct: true

    config.vm.provider :vmware_fusion do |vb|
      vb.gui = $vb_gui
    end

    config.vm.provider :virtualbox do |vb|
      vb.gui = $vb_gui
      vb.memory = $vb_memory
      vb.cpus = $vb_cpus
    end

    config.vm.network :private_network, ip: $private_ip

    config.vm.provision :shell, :inline => "docker run -d -p 8080:8080 rancher/server:latest", :privileged => true
    config.vm.provision :shell, :inline => "docker run -e CATTLE_AGENT_IP=%s -e WAIT=true -v /var/run/docker.sock:/var/run/docker.sock rancher/agent:latest http://localhost:8080" % $private_ip, :privileged => true
  end
end
