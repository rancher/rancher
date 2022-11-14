package components

var AKSClusterPrefix = `resource "rancher2_cluster" "rancher2_cluster" {
  name = var.cluster_name
  aks_config_v2 {
    cloud_credential_id = rancher2_cloud_credential.rancher2_cloud_credential.id
    resource_group = var.resource_group
    resource_location = var.resource_location
    dns_prefix = var.dns_prefix
    kubernetes_version = "`