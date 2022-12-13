#Cluster creation with v1 charts installed
terraform {
  required_version = ">= 0.14"
  required_providers {
    rancher2 = {
      source = "rancher/rancher2"
    }

  }
}

provider "rancher2" {
  api_url   = var.rancher_api_url
  token_key = var.rancher_token_key
}

resource "rancher2_cluster" "custom" {
  name = var.cluster_name
  description = "Rancher v1 Charts Cluster"
  rke_config {
    kubernetes_version = var.rancher_k8s_version
    network {
      plugin = "canal"
    }
  }
  enable_cluster_monitoring = false
}

resource "rancher2_node_template" "node-template" {
  name = "tf-aws-node-template"
  description = "TF AWS node template"

  amazonec2_config {
    access_key = var.aws_access_key
    secret_key = var.aws_secret_key
    ami =  var.aws_ami
    region = var.aws_region
    security_group = var.aws_security_groups
    subnet_id = var.aws_subnet
    vpc_id = var.aws_vpc_id
    zone = var.aws_zone
    root_size = var.aws_root_size
    instance_type = var.aws_instance_type
    volume_type = var.aws_volume_type
  }
}

resource "rancher2_node_pool" "cp-node-pool" {
  cluster_id =  rancher2_cluster.custom.id
  name = "tf-cp-node-pool"
  hostname_prefix = var.node_pool_name_cp
  node_template_id = rancher2_node_template.node-template.id
  quantity = 2
  control_plane = true
  etcd = false
  worker = false
}

resource "rancher2_node_pool" "etcd-node-pool" {
  cluster_id =  rancher2_cluster.custom.id
  name = "tf-etcd-node-pool"
  hostname_prefix = var.node_pool_name_etcd
  node_template_id = rancher2_node_template.node-template.id
  quantity = 3
  control_plane = false
  etcd = true
  worker = false
}

resource "rancher2_node_pool" "worker-node-pool" {
  cluster_id =  rancher2_cluster.custom.id
  name = "tf-worker-node-pool"
  hostname_prefix = var.node_pool_name_worker
  node_template_id = rancher2_node_template.node-template.id
  quantity = var.worker_count
  control_plane = false
  etcd = false
  worker = true
}

resource "rancher2_cluster_sync" "v1charts-sync" {
  cluster_id = rancher2_cluster.custom.id
  wait_catalogs = true
  state_confirm = 60
}

resource "local_file" "sync_kube_config" {
  content  = rancher2_cluster_sync.v1charts-sync.kube_config
  filename = "kube_config_provisioned_cluster.yaml"
  
  depends_on = [
    rancher2_cluster_sync.v1charts-sync
  ]
}



resource "rancher2_namespace" "longhorn-system-namespace" {
  name = "longhorn-system"
  project_id = rancher2_cluster_sync.v1charts-sync.system_project_id
  description = "longhorn-system namespace"
}

resource "null_resource" "longhorn-iscsi-nfs" {
  provisioner "local-exec" {
    environment = {
      KUBECONFIG = "kube_config_provisioned_cluster.yaml"
    }
    command = "kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/${var.rancher_longhorn_prereq_version}/deploy/prerequisite/longhorn-iscsi-installation.yaml --namespace=longhorn-system && kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/${var.rancher_longhorn_prereq_version}/deploy/prerequisite/longhorn-nfs-installation.yaml --namespace=longhorn-system"
  }

  depends_on = [
    rancher2_cluster_sync.v1charts-sync,
    local_file.sync_kube_config,
    rancher2_namespace.longhorn-system-namespace
  ]
}

resource "rancher2_app" "longhorn-system" {
  catalog_name = "library"
  name = "longhorn"
  description = "Terraform app acceptance test"
  project_id = rancher2_cluster_sync.v1charts-sync.system_project_id
  template_name = "longhorn"
  target_namespace = rancher2_namespace.longhorn-system-namespace.id
  answers = {
    "image.defaultImage" = true
    "privateRegistry.registryUrl" = ""
    "privateRegistry.registryUser" = ""
    "privateRegistry.registryPasswd" = ""
    "privateRegistry.registrySecret" = ""
    "longhorn.default_setting" = false
    "persistence.defaultClass" = true
    "persistence.reclaimPolicy" = "Delete"
    "persistence.defaultClassReplicaCount" = "3"
    "persistence.recurringJobSelector.enable" = false
    "persistence.backingImage.enable" = false
    "ingress.enabled" = false
    "service.ui.type" = "Rancher-Proxy"
    "enablePSP" = true
  }

  depends_on = [
    null_resource.longhorn-iscsi-nfs
  ]
}

resource "null_resource" "longhorn-statefulset-example" {
  provisioner "local-exec" {
    environment = {
      KUBECONFIG = "kube_config_provisioned_cluster.yaml"
    }
    command = "kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/master/examples/statefulset.yaml"
  }

  depends_on = [
    rancher2_app.longhorn-system
  ]
}
