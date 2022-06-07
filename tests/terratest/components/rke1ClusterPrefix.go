package components

var RKE1ClusterPrefix = `resource "rancher2_cluster" "rancher2_cluster" {
  name = var.cluster_name
  rke_config {
    kubernetes_version = "`