package components

var RKE1ClusterSyncPrefix = `resource "rancher2_cluster_sync" "rancher2_cluster_sync" {
cluster_id    = rancher2_cluster.rancher2_cluster.id
`