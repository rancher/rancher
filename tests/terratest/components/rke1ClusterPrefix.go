package components

var RKE1ClusterPrefix = `resource "rancher2_cluster" "rancher2_cluster" {
  depends_on = [rancher2_node_template.rancher2_node_template]
  name       = var.cluster_name
  rke_config {
    kubernetes_version = "` 