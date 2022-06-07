output "cluster_name" {
  value = rancher2_cluster.rancher2_cluster.name
}

output "host_url" {
  value = var.rancher_api_url
  sensitive = true
}

output "token" {
  value = var.rancher_admin_bearer_token
  sensitive = true
}

output "token_prefix" {
  value = var.token_prefix
  sensitive = true
}

output "config1_expected_node_count" {
  value = var.config1_expected_node_count
}

output "config1_expected_provider" {
  value = var.config1_expected_provider
}

output "config1_expected_state" {
  value = var.config1_expected_state
}

output "config1_expected_kubernetes_version" {
  value = var.config1_expected_kubernetes_version
}

output "config1_expected_rancher_server_version" {
  value = var.config1_expected_rancher_server_version
}

output "config2_expected_node_count" {
  value = var.config2_expected_node_count
}

output "config2_expected_kubernetes_version" {
  value = var.config2_expected_kubernetes_version
}

output "config3_expected_node_count" {
  value = var.config3_expected_node_count
}