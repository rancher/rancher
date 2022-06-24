output "cluster_name" {
  value = rancher2_cluster.rancher2_cluster.name
}

# output "host_url" {
#   value = var.rancher_api_url
#   sensitive = true
# }

# output "token" {
#   value = var.rancher_admin_bearer_token
#   sensitive = true
# }


output "expected_node_count_3" {
  value = var.expected_node_count_3
}

output "expected_provider" {
  value = var.expected_provider
}

output "expected_state_active" {
  value = var.expected_state_active
}

output "expected_kubernetes_version_12210" {
  value = var.expected_kubernetes_version_12210
}

output "expected_node_count_8" {
  value = var.expected_node_count_8
}

output "expected_kubernetes_version_1237" {
  value = var.expected_kubernetes_version_1237
}

output "expected_node_count_6" {
  value = var.expected_node_count_6
}