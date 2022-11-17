package components

var RKE1NodePoolSpecs1 = `" {
  depends_on       = [rancher2_cluster.rancher2_cluster]
  cluster_id       = rancher2_cluster.rancher2_cluster.id
  name             = "pool` 