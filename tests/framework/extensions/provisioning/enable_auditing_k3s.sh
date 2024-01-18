#!/bin/sh
sudo mkdir -p -m o+w /var/lib/rancher/k3s/server/logs
sudo install -m o+w /dev/null /var/lib/rancher/k3s/server/audit.yaml
sudo chmod o+w /etc/systemd/system/k3s.service
sed -i '$d' /etc/systemd/system/k3s.service
sudo echo -e "       '--kube-apiserver-arg=audit-log-path=/var/lib/rancher/k3s/server/logs/audit.log'  \ \n       '--kube-apiserver-arg=audit-policy-file=/var/lib/rancher/k3s/server/audit.yaml'" >> /etc/systemd/system/k3s.service
sudo systemctl daemon-reload
sudo systemctl restart k3s.service