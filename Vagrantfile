# -*- mode: ruby -*-
# vi: set ft=ruby :

require_relative 'vagrant_ros_guest_plugin.rb'

$number_of_nodes = 1
# DO NOT USE 172.17.*, 172.18.*, 10.0.2 as these are used on other NICs: routing will get screwed!
$private_ip_prefix = '172.19.8' # rancher-server will be on 172.19.8.100, other nodes will start from 172.19.8.101
$expose_rancher_ui = 8080
$vb_gui = false
$vb_memory = 1024
$vb_cpus = 1

$rsync_folder_disabled = true

def config_node(node, hostname, node_ip, server_ip)
  node.vm.hostname = hostname
  node.vm.network 'private_network', ip: node_ip
  node.vm.provision :shell, :inline => "docker run -e CATTLE_AGENT_IP=#{node_ip} -e WAIT=true -v /var/run/docker.sock:/var/run/docker.sock rancher/agent:latest http://#{server_ip}:8080", :privileged => true
end

Vagrant.configure(2) do |config|
  config.vm.box         = 'rancherio/rancheros'
  config.vm.box_version = '>=0.3.3'

  config.vm.provider 'virtualbox' do |vb|
    vb.gui = $vb_gui
    vb.memory = $vb_memory
    vb.cpus = $vb_cpus
  end

  hostname_server = 'rancher-server'
  server_ip = "#{$private_ip_prefix}.100"
  config.vm.define hostname_server do |server|
    server.vm.network 'forwarded_port', guest: 8080, host: $expose_rancher_ui, auto_correct: true
    server.vm.provision :shell, :inline => 'docker run --name=rancher-server -l io.rancher.container.system=rancher-agent --restart=always -d -p 8080:8080 rancher/server:latest', :privileged => true
    config_node(server, hostname_server, server_ip, server_ip)
  end

  (1..$number_of_nodes-1).each do |i|
    hostname_node = 'rancher-%02d' % i
    node_ip = "#{$private_ip_prefix}.#{100+i}"
    config.vm.define hostname_node do |node|
      config_node(node, hostname_node, node_ip, server_ip)
    end
  end

  # Disabling compression because OS X has an ancient version of rsync installed.
  # Add -z or remove rsync__args below if you have a newer version of rsync on your machine.
  config.vm.synced_folder '.', '/opt/rancher', type: 'rsync',
      rsync__exclude: '.git/', rsync__args: ['--verbose', '--archive', '--delete', '--copy-links'],
      disabled: $rsync_folder_disabled
end
