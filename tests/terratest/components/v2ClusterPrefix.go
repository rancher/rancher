package components

var V2ClusterPrefix = `resource "rancher2_cluster_v2" "rancher2_cluster_v2" {
  name                                     = var.cluster_name
  kubernetes_version                       = "`