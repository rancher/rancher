provider "helm" {
  alias = "rancher-hosted3"
  kubernetes {
    config_path = local_file.kube_config_hosted3_yaml.filename
  }
}

provider "rancher2" {
  alias     = "bootstrap_hosted3"
  api_url   = "https://${var.hosted3_load_balancer_subdomain}.${var.load_balancers_domain}"
  insecure  = true
  bootstrap = true
}

provider "rancher2" {
  alias     = "admin_hosted3"
  api_url   = rancher2_bootstrap.admin_hosted3.url
  insecure  = true
  token_key = rancher2_bootstrap.admin_hosted3.token
  timeout   = "10m"
}

resource "rancher2_auth_config_github" "github_hosted3" {
  provider = rancher2.admin_hosted3
  client_id     = var.rancher_github_client_id_hosted3
  client_secret = var.rancher_github_client_secret_hosted3
  enabled       = true
}

resource "aws_route53_record" "hosted3_rancher" {
  zone_id = var.zone_id
  name    = "${var.hosted3_load_balancer_subdomain}.${var.load_balancers_domain}"
  type    = "A"
  ttl     = "10"
  records = [linode_instance.hosted3_lb.ip_address]
  depends_on = [linode_instance.hosted3_lb]
}

resource "ssh_resource" "retrieve_config_hosted3" {
  host = linode_instance.hosted3_node1.ip_address
  commands = [
    "sed \"s/127.0.0.1/${linode_instance.hosted3_node1.ip_address}/g\" /etc/rancher/k3s/k3s.yaml"
  ]
  user  = "root"
  agent = false
  private_key = base64decode("${var.ssh_private_key}")
  depends_on = [
    linode_instance.hosted3_node1,
    ssh_resource.retrieve_config_super
    ]
}

resource "ssh_resource" "retrieve_token_hosted3" {
  host = linode_instance.hosted3_node1.ip_address
  commands = [
    "cat /var/lib/rancher/k3s/server/node-token"
  ]
  user  = "root"
  agent = false
  private_key = base64decode("${var.ssh_private_key}")

  depends_on = [
    linode_instance.hosted3_node1,
    ssh_resource.retrieve_token
    ]
}

resource "local_file" "kube_config_hosted3_yaml" {
  filename = format("%s/%s", path.root, "kube_config_server_hosted3.yaml")
  content  = ssh_resource.retrieve_config_hosted3.result
}

resource "local_file" "k3s_token_hosted3" {
  filename = format("%s/%s", path.root, "k3s_token_hosted3")
  content  = ssh_resource.retrieve_token_hosted3.result
}

resource "linode_instance" "hosted3_lb" {
    label = "${var.linode_resource_prefix}hosted3lb-longliving"
    image = "linode/ubuntu20.04"
    region = "us-east"
    type = "g6-standard-2"
    authorized_keys = ["${var.ssh_authorized_key}"]
    root_pass = var.linode_root_password

    group = "hosted_hosted3"
    tags = [ "hosted_hosted3" ]
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
        "hostnamectl set-hostname ${var.linode_resource_prefix}hosted3lb",
        "wget https://releases.rancher.com/install-docker/${var.docker_version}.sh",
        "chmod +x ${var.docker_version}.sh",
        "bash ${var.docker_version}.sh",
        "sed -i \"s/<host1>/${linode_instance.hosted3_node1.ip_address}/g\" nginx/nginx.conf",
        "sed -i \"s/<host2>/${linode_instance.hosted3_node2.ip_address}/g\" nginx/nginx.conf",
        "sed -i \"s/<host3>/${linode_instance.hosted3_node3.ip_address}/g\" nginx/nginx.conf",
        "sed -i \"s/<FQDN>/${var.hosted3_load_balancer_subdomain}.${var.load_balancers_domain}/g\" nginx/nginx.conf",
        "docker run --name docker-nginx -p 80:80 -p 443:443 -v $(pwd)/certs/:/certs/ -v $(pwd)/nginx/nginx.conf:/etc/nginx/nginx.conf -d nginx"
    ]
  }

  depends_on = [
    local_file.fullchain,
    local_file.privkey
  ]
}

resource "linode_instance" "hosted3_node1" {
    label = "${var.linode_resource_prefix}hosted3n1-longliving"
    image = "linode/ubuntu20.04"
    region = "us-east"
    type = "g6-dedicated-4"
    authorized_keys = ["${var.ssh_authorized_key}"]
    root_pass = var.linode_root_password

    group = "hosted_hosted3"
    tags = [ "hosted_hosted3" ]
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
        "hostnamectl set-hostname ${var.linode_resource_prefix}hosted3n1",
        "echo \"vm.panic_on_oom=0\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"vm.overcommit_memory=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic=10\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic_on_oops=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "sysctl -p /etc/sysctl.d/90-kubelet.conf",
        "systemctl restart systemd-sysctl",
        "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--disable=traefik' INSTALL_K3S_VERSION='${var.k3s_version_hosted3}' K3S_TOKEN=${random_string.k3s_token.result} sh -s - server --node-name ${self.label} --cluster-init --node-external-ip=${self.ip_address} --tls-san ${var.hosted3_load_balancer_subdomain}.${var.load_balancers_domain}"
      ]
    }

    provisioner "file" {
      source      = "${path.module}/manifests/ingress-nginx.yaml"
      destination = "/var/lib/rancher/k3s/server/manifests/ingress-nginx.yaml"
    }
}

resource "linode_instance" "hosted3_node2" {
    label = "${var.linode_resource_prefix}hosted3n2-longliving"
    image = "linode/ubuntu20.04"
    region = "us-east"
    type = "g6-dedicated-4"
    authorized_keys = ["${var.ssh_authorized_key}"]
    root_pass = var.linode_root_password

    group = "hosted_hosted3"
    tags = [ "hosted_hosted3" ]
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
      source      = format("%s/%s", path.root, "k3s_token_hosted3")
      destination = "k3s_token"
    }
    
    provisioner "remote-exec" {
      inline = [
        "hostnamectl set-hostname ${var.linode_resource_prefix}hosted3n2",
        "echo \"vm.panic_on_oom=0\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"vm.overcommit_memory=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic=10\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic_on_oops=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "sysctl -p /etc/sysctl.d/90-kubelet.conf",
        "systemctl restart systemd-sysctl",
        "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--disable=traefik' INSTALL_K3S_VERSION='${var.k3s_version_hosted3}' K3S_TOKEN=`cat k3s_token` sh -s - server --node-name ${self.label} --server https://${linode_instance.hosted3_node1.ip_address}:6443 --node-external-ip=${self.ip_address} --tls-san ${var.hosted3_load_balancer_subdomain}.${var.load_balancers_domain}"
      ]
    }

    depends_on = [local_file.k3s_token_hosted3]
}

resource "linode_instance" "hosted3_node3" {
    label = "${var.linode_resource_prefix}hosted3n3-longliving"
    image = "linode/ubuntu20.04"
    region = "us-east"
    type = "g6-dedicated-4"
    authorized_keys = ["${var.ssh_authorized_key}"]
    root_pass = var.linode_root_password

    group = "hosted_hosted3"
    tags = [ "hosted_hosted3" ]
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
      source      = format("%s/%s", path.root, "k3s_token_hosted3")
      destination = "k3s_token"
    }

    provisioner "remote-exec" {
      inline = [
        "hostnamectl set-hostname ${var.linode_resource_prefix}hosted3n3",
        "echo \"vm.panic_on_oom=0\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"vm.overcommit_memory=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic=10\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic_on_oops=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "sysctl -p /etc/sysctl.d/90-kubelet.conf",
        "systemctl restart systemd-sysctl",
        "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--disable=traefik' INSTALL_K3S_VERSION='${var.k3s_version_hosted3}' K3S_TOKEN=`cat k3s_token` sh -s - server --node-name ${self.label} --server https://${linode_instance.hosted3_node1.ip_address}:6443 --node-external-ip=${self.ip_address} --tls-san ${var.hosted3_load_balancer_subdomain}.${var.load_balancers_domain}",
      ]
    }

    depends_on = [linode_instance.hosted3_node2]
}

resource "null_resource" "import_hosted_cluster3" {
  provisioner "local-exec" {
    command = "${rancher2_cluster.hosted3.cluster_registration_token.0.insecure_command}"

    environment = {
      KUBECONFIG       = local_file.kube_config_hosted3_yaml.filename
      RANCHER_HOSTNAME = "${var.super_load_balancer_subdomain}.${var.load_balancers_domain}"
    }
  }

  depends_on = [
    rancher2_cluster.hosted3,
    linode_instance.hosted3_node3
  ]
}

resource "helm_release" "rancher_hosted3_server" {
  provider         = helm.rancher-hosted3
  name             = "rancher"
  chart            = "https://releases.rancher.com/server-charts/latest/rancher-${var.rancher_version_hosted3}.tgz"
  namespace        = "cattle-system"
  create_namespace = true
  wait             = true

  set {
    name  = "hostname"
    value = "${var.hosted3_load_balancer_subdomain}.${var.load_balancers_domain}"
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
    null_resource.wait_for_ingress_rollout_hosted3
  ]
}

resource "null_resource" "wait_for_hosted3_rancher" {
  provisioner "local-exec" {
    command = <<-EOT
    kubectl -n cattle-system rollout status deploy/rancher
    EOT

    environment = {
      KUBECONFIG       = local_file.kube_config_hosted3_yaml.filename
      RANCHER_HOSTNAME = "${var.hosted3_load_balancer_subdomain}.${var.load_balancers_domain}"
    }
  }
  depends_on = [
    helm_release.rancher_hosted3_server
  ]
}

resource "null_resource" "wait_for_ingress_rollout_hosted3" {
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
      KUBECONFIG       = local_file.kube_config_hosted3_yaml.filename
      RANCHER_HOSTNAME = "${var.hosted1_load_balancer_subdomain}.${var.load_balancers_domain}"
    }
  }
  depends_on = [
    aws_route53_record.hosted3_rancher
  ]
}

resource "rancher2_bootstrap" "admin_hosted3" {
  provider         = rancher2.bootstrap_hosted3
  initial_password = "admin"
  password         = var.rancher_password
  depends_on       = [null_resource.wait_for_hosted3_rancher]
}

resource "rancher2_cluster" "custom_cluster3" {
  provider = rancher2.admin_hosted3
  name = "custom-cluster-hosted3"
  description = "Rancher custom-cluster-hosted3"
  enable_cluster_monitoring = false
  rke_config {
    network {
      plugin = "flannel"
    }
  }
}

resource "rancher2_cluster_sync" "custom_cluster3" {
    provider = rancher2.admin_hosted3
    cluster_id =  rancher2_cluster.custom_cluster3.id
    state_confirm = 25
    wait_catalogs = true
    depends_on = [
      linode_instance.custom_nodes3
    ]
}

resource "linode_instance" "custom_nodes3"{
    count  = length(local.node_config)
    label  = "${var.linode_resource_prefix}custom3-n${count.index}-longliving" 
    image  = "linode/ubuntu20.04"
    region = "us-east"
    type   = "g6-dedicated-4"
    authorized_keys = ["${var.ssh_authorized_key}"]
    root_pass = var.linode_root_password

    group = "hosted_hosted3"
    tags = [ "hosted_hostd3" ]
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

    depends_on = [
        rancher2_cluster.custom_cluster3
    ]

    provisioner "remote-exec" {
        inline = [
            "hostnamectl set-hostname ${var.linode_resource_prefix}custom3-n${count.index}",
            "wget https://releases.rancher.com/install-docker/${var.docker_version}.sh",
            "chmod +x ${var.docker_version}.sh",
            "bash ${var.docker_version}.sh",
            "${rancher2_cluster.custom_cluster3.cluster_registration_token[0].node_command} --address ${self.ip_address} --internal-address ${self.private_ip_address} --${local.node_config[count.index]}"
        ]
    }

}

resource "rancher2_cloud_credential" "linode_rke2_hosted3" {
  count = local.hosted3_version_ready_for_rke2 ? 1 : 0
  provider = rancher2.admin_hosted3
  name = "linode-rke2-cluster1"
  linode_credential_config {
    token = var.linode_token
  }
}

resource "rancher2_machine_config_v2" "linode_rke2_control_plane_hosted3" {
  count = local.hosted3_version_ready_for_rke2 ? 1 : 0
  provider = rancher2.admin_hosted3
  generate_name = "hosted3-rke2-cp"
  linode_config {
    create_private_ip = true
    image = "linode/ubuntu20.04"
    swap_size = 256
    root_pass = var.linode_root_password
  }
}

resource "rancher2_machine_config_v2" "linode_rke2_etcd_hosted3" {
  count = local.hosted3_version_ready_for_rke2 ? 1 : 0
  provider = rancher2.admin_hosted3
  generate_name = "hosted3-rke2-etcd"
  linode_config {
    create_private_ip = true
    image = "linode/ubuntu20.04"
    swap_size = 256
    root_pass = var.linode_root_password
  }
}

resource "rancher2_machine_config_v2" "linode_rke2_worker_hosted3" {
  count = local.hosted3_version_ready_for_rke2 ? 1 : 0
  provider = rancher2.admin_hosted3
  generate_name = "hosted3-rke2-worker"
  linode_config {
    create_private_ip = true
    image = "linode/ubuntu20.04"
    swap_size = 256
    root_pass = var.linode_root_password
  }
}

resource "rancher2_cluster_v2" "linode_rke2_hosted3" {
  count = local.hosted3_version_ready_for_rke2 ? 1 : 0
  provider = rancher2.admin_hosted3
  name = "longliving-rke2-hosted3"
  kubernetes_version = var.rke2_cluster_version_hosted3
  enable_network_policy = false
  default_cluster_role_for_project_members = "user"
  rke_config {
    machine_pools {
      name = "pool-cp"
      cloud_credential_secret_name = rancher2_cloud_credential.linode_rke2_hosted3[count.index].id
      control_plane_role = true
      etcd_role = false
      worker_role = false
      quantity = 1
      machine_config {
        kind = rancher2_machine_config_v2.linode_rke2_control_plane_hosted3[count.index].kind
        name = rancher2_machine_config_v2.linode_rke2_control_plane_hosted3[count.index].name
      }
    }
    machine_pools {
      name = "pool-etcd"
      cloud_credential_secret_name = rancher2_cloud_credential.linode_rke2_hosted3[count.index].id
      control_plane_role = false
      etcd_role = true
      worker_role = false
      quantity = 1
      machine_config {
        kind = rancher2_machine_config_v2.linode_rke2_etcd_hosted3[count.index].kind
        name = rancher2_machine_config_v2.linode_rke2_etcd_hosted3[count.index].name
      }
    }
    machine_pools {
      name = "pool-worker"
      cloud_credential_secret_name = rancher2_cloud_credential.linode_rke2_hosted3[count.index].id
      control_plane_role = false
      etcd_role = false
      worker_role = true
      quantity = 3
      machine_config {
        kind = rancher2_machine_config_v2.linode_rke2_worker_hosted3[count.index].kind
        name = rancher2_machine_config_v2.linode_rke2_worker_hosted3[count.index].name
      }
    }
  }
}
