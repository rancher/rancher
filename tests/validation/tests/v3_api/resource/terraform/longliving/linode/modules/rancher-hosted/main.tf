terraform {
  required_providers {
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.5.1"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.2.0"
    }
    linode = {
      source  = "linode/linode"
      version = "~> 1.27.2"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.15.1"
    }
    ssh = {
      source  = "loafoe/ssh"
      version = "~> 1.2.0"
    }
    rancher2 = {
      source  = "rancher/rancher2"
      version = "1.21.0"
    }
  }
}

# Configure the Linode Provider
provider "linode" {
  token = var.linode_token
}

provider "helm" {
  alias = "rancher-super"
  kubernetes {
    config_path = local_file.kube_config_server_yaml.filename
  }
}


provider "rancher2" {
  alias     = "bootstrap"
  api_url   = "https://${var.super_load_balancer_subdomain}.${var.load_balancers_domain}"
  insecure  = true
  bootstrap = true
}


provider "rancher2" {
  alias     = "admin"
  api_url   = rancher2_bootstrap.admin.url
  insecure  = true
  token_key = rancher2_bootstrap.admin.token
  timeout   = "10m"
}

resource "rancher2_auth_config_github" "github_super" {
  provider = rancher2.admin
  client_id     = var.rancher_github_client_id_super
  client_secret = var.rancher_github_client_secret_super
  enabled       = true
}

locals{
  # Check to see if we're in Rancher 2.6 or newer
  splitted_version_hosted1 = split(".", var.rancher_version_hosted1)
  splitted_version_hosted2 = split(".", var.rancher_version_hosted2)
  splitted_version_hosted3 = split(".", var.rancher_version_hosted3)
  hosted1_version_ready_for_rke2 = tonumber(local.splitted_version_hosted1[1]) > 5 ? true : false
  hosted2_version_ready_for_rke2 = tonumber(local.splitted_version_hosted2[1]) > 5 ? true : false
  hosted3_version_ready_for_rke2 = tonumber(local.splitted_version_hosted3[1]) > 5 ? true : false
  
  # Custom cluster node roles
  node_config = ["controlplane", "etcd", "worker", "worker", "worker"]
}



resource "random_string" "k3s_token" {
  length  = 48
  upper   = false
  special = false
}

resource "local_file" "fullchain" {
    content  = base64decode("${var.fullchain}")
    filename = "${path.module}/scripts/certs/fullchain.pem"
}

resource "local_file" "privkey" {
    content  = base64decode("${var.privkey}")
    filename = "${path.module}/scripts/certs/privkey.pem"
}

resource "linode_instance" "super_lb" {
    label = "${var.linode_resource_prefix}superlb-longliving"
    image = "linode/ubuntu20.04"
    region = "us-east"
    type = "g6-standard-2"
    authorized_keys = ["${var.ssh_authorized_key}"]
    root_pass = var.linode_root_password

    group = "hosted_super"
    tags = [ "hosted_super" ]
    swap_size = 256
    private_ip = true
    
    alerts {
    cpu            = 0
    io             = 0
    network_in     = 0
    network_out    = 0
    transfer_quota = 0
  }

  connection {
      host = self.ip_address
      user = "root"
      password = var.linode_root_password
  }

  provisioner "file" {
    source      = "${path.module}/scripts/certs"
    destination = "certs"
  }
 
  provisioner "file" {
    source      = "${path.module}/scripts/nginx"
    destination = "nginx"
  }
  provisioner "remote-exec" {
    inline = [
        "hostnamectl set-hostname ${var.linode_resource_prefix}superlb",
        "wget https://releases.rancher.com/install-docker/${var.docker_version}.sh",
        "chmod +x ${var.docker_version}.sh",
        "bash ${var.docker_version}.sh",
        "sed -i \"s/<host1>/${linode_instance.super_node1.ip_address}/g\" nginx/nginx.conf",
        "sed -i \"s/<host2>/${linode_instance.super_node2.ip_address}/g\" nginx/nginx.conf",
        "sed -i \"s/<host3>/${linode_instance.super_node3.ip_address}/g\" nginx/nginx.conf",
        "sed -i \"s/<FQDN>/${var.super_load_balancer_subdomain}.${var.load_balancers_domain}/g\" nginx/nginx.conf",
        "docker run --name docker-nginx -p 80:80 -p 443:443 -v $(pwd)/certs/:/certs/ -v $(pwd)/nginx/nginx.conf:/etc/nginx/nginx.conf -d nginx"
    ]
  }

  depends_on = [
    local_file.fullchain,
    local_file.privkey
  ]
}

resource "linode_instance" "super_node1" {
    label = "${var.linode_resource_prefix}supern1-longliving"
    image = "linode/ubuntu20.04"
    region = "us-east"
    type = "g6-dedicated-4"
    authorized_keys = ["${var.ssh_authorized_key}"]
    root_pass = var.linode_root_password

    group = "hosted_super"
    tags = [ "hosted_super" ]
    swap_size = 256
    private_ip = true

    alerts {
      cpu            = 0
      io             = 0
      network_in     = 0
      network_out    = 0
      transfer_quota = 0
    }

    connection {
      host = self.ip_address
      user = "root"
      password = var.linode_root_password
    }

    provisioner "remote-exec" {
      inline = [
        "hostnamectl set-hostname ${var.linode_resource_prefix}supern1",
        "echo \"vm.panic_on_oom=0\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"vm.overcommit_memory=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic=10\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic_on_oops=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "sysctl -p /etc/sysctl.d/90-kubelet.conf",
        "systemctl restart systemd-sysctl",
        "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--disable=traefik' INSTALL_K3S_VERSION='${var.k3s_version_super}' K3S_TOKEN=${random_string.k3s_token.result} sh -s - server --node-name ${self.label} --cluster-init --node-external-ip=${self.ip_address} --tls-san ${var.super_load_balancer_subdomain}.${var.load_balancers_domain}"
      ]
    }

    provisioner "file" {
      source      = "${path.module}/manifests/ingress-nginx.yaml"
      destination = "/var/lib/rancher/k3s/server/manifests/ingress-nginx.yaml"
    }
}

resource "linode_instance" "super_node2" {
    label = "${var.linode_resource_prefix}supern2-longliving"
    image = "linode/ubuntu20.04"
    region = "us-east"
    type = "g6-dedicated-4"
    authorized_keys = ["${var.ssh_authorized_key}"]
    root_pass = var.linode_root_password

    group = "hosted_super"
    tags = [ "hosted_super" ]
    swap_size = 256
    private_ip = true

    alerts {
      cpu            = 0
      io             = 0
      network_in     = 0
      network_out    = 0
      transfer_quota = 0
    }

    connection {
      host = self.ip_address
      user = "root"
      password = var.linode_root_password
    }

    provisioner "file" {
      source      = format("%s/%s", path.root, "k3s_token_super")
      destination = "k3s_token"
    }
    
    provisioner "remote-exec" {
      inline = [
        "hostnamectl set-hostname ${var.linode_resource_prefix}supern2",
        "echo \"vm.panic_on_oom=0\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"vm.overcommit_memory=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic=10\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic_on_oops=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "sysctl -p /etc/sysctl.d/90-kubelet.conf",
        "systemctl restart systemd-sysctl",
        "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--disable=traefik' INSTALL_K3S_VERSION='${var.k3s_version_super}' K3S_TOKEN=`cat k3s_token` sh -s - server --node-name ${self.label} --server https://${linode_instance.super_node1.ip_address}:6443 --node-external-ip=${self.ip_address} --tls-san ${var.super_load_balancer_subdomain}.${var.load_balancers_domain}"
      ]
    }

    depends_on = [local_file.k3s_token]
}

resource "linode_instance" "super_node3" {
    label = "${var.linode_resource_prefix}supern3-longliving"
    image = "linode/ubuntu20.04"
    region = "us-east"
    type = "g6-dedicated-4"
    authorized_keys = ["${var.ssh_authorized_key}"]
    root_pass = var.linode_root_password

    group = "hosted_super"
    tags = [ "hosted_super" ]
    swap_size = 256
    private_ip = true

    alerts {
      cpu            = 0
      io             = 0
      network_in     = 0
      network_out    = 0
      transfer_quota = 0
    }

    connection {
      host = self.ip_address
      user = "root"
      password = var.linode_root_password
    }

    provisioner "file" {
      source      = format("%s/%s", path.root, "k3s_token_super")
      destination = "k3s_token"
    }

    provisioner "remote-exec" {
      inline = [
        "hostnamectl set-hostname ${var.linode_resource_prefix}supern3",
        "echo \"vm.panic_on_oom=0\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"vm.overcommit_memory=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic=10\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic_on_oops=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "sysctl -p /etc/sysctl.d/90-kubelet.conf",
        "systemctl restart systemd-sysctl",
        "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--disable=traefik' INSTALL_K3S_VERSION='${var.k3s_version_super}' K3S_TOKEN=`cat k3s_token` sh -s - server --node-name ${self.label} --server https://${linode_instance.super_node1.ip_address}:6443 --node-external-ip=${self.ip_address} --tls-san ${var.super_load_balancer_subdomain}.${var.load_balancers_domain}"
      ]
    }

    depends_on = [linode_instance.super_node2]
}

resource "ssh_resource" "retrieve_config_super" {
  host = linode_instance.super_node1.ip_address
  commands = [
    "sed \"s/127.0.0.1/${linode_instance.super_node1.ip_address}/g\" /etc/rancher/k3s/k3s.yaml"
  ]
  user  = "root"
  agent = false
  private_key = base64decode("${var.ssh_private_key}")
  depends_on = [linode_instance.super_node1]
}

resource "ssh_resource" "retrieve_token" {
  host = linode_instance.super_node1.ip_address
  commands = [
    "cat /var/lib/rancher/k3s/server/node-token"
  ]
  user  = "root"
  agent = false
  private_key = base64decode("${var.ssh_private_key}")

  depends_on = [linode_instance.super_node1]
}

resource "local_file" "kube_config_server_yaml" {
  filename = format("%s/%s", path.root, "kube_config_server_super.yaml")
  content  = ssh_resource.retrieve_config_super.result
}

resource "local_file" "k3s_token" {
  filename = format("%s/%s", path.root, "k3s_token_super")
  content  = ssh_resource.retrieve_token.result
}

resource "aws_route53_record" "super_rancher" {
  zone_id = var.zone_id
  name    = "${var.super_load_balancer_subdomain}.${var.load_balancers_domain}"
  type    = "A"
  ttl     = "10"
  records = [linode_instance.super_lb.ip_address]
  depends_on = [linode_instance.super_lb]
}

# Install Rancher helm chart on SUPER
resource "helm_release" "rancher_server" {
  provider         = helm.rancher-super
  name             = "rancher"
  chart            = "https://releases.rancher.com/server-charts/latest/rancher-${var.rancher_version_super}.tgz"
  namespace        = "cattle-system"
  create_namespace = true
  wait             = true

  set {
    name  = "hostname"
    value = "${var.super_load_balancer_subdomain}.${var.load_balancers_domain}"
  }

   set {
    name  = "tls"
    value = "external"
  }

  set {
    name  = "bootstrapPassword"
    value = "admin"
  }

  depends_on = [
    null_resource.wait_for_ingress_rollout_super
  ]
}

resource "null_resource" "wait_for_rancher" {
  provisioner "local-exec" {
    command = <<-EOT
    kubectl -n cattle-system rollout status deploy/rancher
    EOT

    environment = {
      KUBECONFIG       = local_file.kube_config_server_yaml.filename
      RANCHER_HOSTNAME = "${var.super_load_balancer_subdomain}.${var.load_balancers_domain}"
    }
  }

  depends_on = [
    helm_release.rancher_server
  ]
}

resource "null_resource" "wait_for_ingress_rollout_super" {
  provisioner "local-exec" {
    command = <<-EOT
    exit_test () {
      if [ $? -eq 0 ]; then
        printf "\n Check completed \n"
      else
        printf "\n There was a failure \n" >&2
        exit 1
      fi
    }
    kubectl wait job -n kube-system helm-install-ingress-nginx --for condition=Complete --timeout=30s; exit_test
    kubectl wait pods -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx --for condition=Ready --timeout=30s; exit_test
    kubectl -n ingress-nginx rollout status ds/ingress-nginx-controller; exit_test
    EOT

    environment = {
      KUBECONFIG       = local_file.kube_config_server_yaml.filename
      RANCHER_HOSTNAME = "${var.hosted1_load_balancer_subdomain}.${var.load_balancers_domain}"
    }
  }
  depends_on = [
    aws_route53_record.super_rancher
  ]
}

resource "rancher2_bootstrap" "admin" {
  provider         = rancher2.bootstrap
  initial_password = "admin"
  password         = var.rancher_password
  depends_on       = [null_resource.wait_for_rancher]
}

resource "rancher2_cluster" "hosted1" {
  provider    = rancher2.admin
  name        = "hosted1"
  description = ""
}


resource "rancher2_cluster" "hosted2" {
  provider    = rancher2.admin
  name        = "hosted2"
  description = ""
}

resource "rancher2_cluster" "hosted3" {
  provider    = rancher2.admin
  name        = "hosted3"
  description = ""
}
