#Target cluster for v1 charts installation
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

resource "rancher2_cluster_sync" "v1target-sync" {
  cluster_id =  var.cluster_id
}

resource "local_file" "sync_kube_config" {
  content  = rancher2_cluster_sync.v1target-sync.kube_config
  filename = "kube_config_target_cluster.yaml"

  depends_on = [
    rancher2_cluster_sync.v1target-sync
  ]
}


resource "rancher2_namespace" "longhorn-system-namespace" {
  name = "longhorn-system"
  project_id = var.project_id
  description = "longhorn-system namespace"

  depends_on = [
    rancher2_cluster_sync.v1target-sync,
    local_file.sync_kube_config
  ]
}


resource "null_resource" "longhorn-iscsi-nfs" {
  provisioner "local-exec" {
    environment = {
      KUBECONFIG = "kube_config_target_cluster.yaml"
    }
    command = "kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/${var.rancher_longhorn_prereq_version}/deploy/prerequisite/longhorn-iscsi-installation.yaml --namespace=longhorn-system && kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/${var.rancher_longhorn_prereq_version}/deploy/prerequisite/longhorn-nfs-installation.yaml --namespace=longhorn-system"
  }

  depends_on = [
    rancher2_cluster_sync.v1target-sync,
    local_file.sync_kube_config,
    rancher2_namespace.longhorn-system-namespace
  ]
}


resource "rancher2_app" "longhorn-system" {
  catalog_name = "library"
  name = "longhorn"
  description = "Terraform app acceptance test"
  project_id = var.project_id
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
}

resource "null_resource" "longhorn-statefulset-example" {
  provisioner "local-exec" {
    environment = {
      KUBECONFIG = "kube_config_target_cluster.yaml"
    }
    command = "kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/master/examples/statefulset.yaml"
  }

  depends_on = [
    rancher2_app.longhorn-system
  ]
}
