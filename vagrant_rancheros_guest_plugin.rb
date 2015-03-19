require 'ipaddr'

## Hacking this until we get a real plugin

# Borrowing from http://stackoverflow.com/questions/1825928/netmask-to-cidr-in-ruby
IPAddr.class_eval do
  def to_cidr
    self.to_i.to_s(2).count("1")
  end
end

module VagrantPlugins
    module GuestLinux
        class Plugin < Vagrant.plugin("2")
            guest_capability("linux", "configure_networks") do
                Cap::ConfigureNetworks
            end
        end

        module Cap
            class ConfigureNetworks

                def self.configure_networks(machine, networks)
                    machine.communicate.tap do |comm|
                        interfaces = []
                        comm.sudo("ip link show|grep eth[1-9]|awk -e '{print $2}'|sed -e 's/:$//'") do |_, result|
                            interfaces = result.split("\n")
                        end

                        networks.each do |network|
                            dhcp = "true"
                            iface = interfaces[network[:interface].to_i - 1]

                            if network[:type] == :static
                                cidr = IPAddr.new(network[:netmask]).to_cidr
                                comm.sudo("rancherctl config set network.interfaces.#{iface}.address #{network[:ip]}/#{cidr}")
                                comm.sudo("rancherctl config set network.interfaces.#{iface}.match #{iface}")

                                dhcp = "false"
                            end
                            comm.sudo("rancherctl config set network.interfaces.#{iface}.dhcp #{dhcp}")
                        end

                        comm.sudo("system-docker restart network")
                    end
                end
            end
        end
    end
end
