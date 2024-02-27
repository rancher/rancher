package rke1

const (
	RKEConfig = `ssh_key_path: /root/go/src/github.com/rancher/rancher/tests/v2/validation/.ssh/jenkins-elliptic-validation.pem
kubernetes_version: %s
nodes:
 - address: %s
   internal_address: %s
   user: %s
   role: [controlplane]
 - address: %s
   internal_address: %s
   user: %s
   role: [etcd]
 - address: %s
   internal_address: %s
   user: %s
   role: [worker]`

	RKE_URL        = "https://github.com/rancher/rke/releases/download/%s/rke_linux-amd64"
	StartDockerCmd = "sudo systemctl start docker && sudo chmod 777 /var/run/docker.sock"
)
