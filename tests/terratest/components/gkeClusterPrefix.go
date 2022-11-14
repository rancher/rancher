package components

var GKEClusterPrefix = `resource "rancher2_cluster" "rancher2_cluster" {
  name = var.cluster_name
  gke_config_v2 {
    name                     = var.cluster_name
	  google_credential_secret = rancher2_cloud_credential.rancher2_cloud_credential.id
	  region                   = var.gke_region
	  project_id               = var.gke_project_id
	  kubernetes_version       = "`