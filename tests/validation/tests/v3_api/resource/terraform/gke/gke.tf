provider "google" {
  region = var.region
  version = "~> 4.53.1"
  credentials = var.credentials
  project = "ei-container-platform-qa"
}

module "gke_auth" {
  source           = "terraform-google-modules/kubernetes-engine/google//modules/auth"
  project_id       = "ei-container-platform-qa"
  cluster_name     = var.cluster_name
  location         = var.location
  depends_on       = [google_container_cluster.default]
}

data "google_client_config" "current" {}

data "google_container_engine_versions" "default" {
  location = var.location
}

resource "google_container_cluster" "default" {
  name               = var.cluster_name
  location           = var.location
  initial_node_count = 3
  min_master_version = var.kubernetes_version
  monitoring_service = "none"
  logging_service    = "none"

  node_config {
    machine_type = var.machine_type
    disk_size_gb = var.disk_size
  }
}

output "cluster_name" {
  value = google_container_cluster.default.name
}

output "cluster_region" {
  value = var.region
}

output "cluster_location" {
  value = google_container_cluster.default.location
}

output "kube_config" {
  value = module.gke_auth.kubeconfig_raw
}
