package components

var EKSClusterPrefix = `resource "rancher2_cluster" "rancher2_cluster" {
  name = var.cluster_name
  eks_config_v2 {
    cloud_credential_id = rancher2_cloud_credential.rancher2_cloud_credential.id
	  region = var.aws_region
	  kubernetes_version = "`