#!/bin/bash

if [[ ${1} == *"sles"* ]]
then
  echo "ENABLING FIPS IN SLES SYSTEM"
  sysctl -a | grep fips
  zypper -n in -t pattern fips
  sed -i 's/^GRUB_CMDLINE_LINUX_DEFAULT="/&fips=1 /'  /etc/default/grub
  grub2-mkconfig -o /boot/grub2/grub.cfg
  mkinitrd
  shutdown -r +1
fi