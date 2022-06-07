# Rancher variables
variable rancher_api_url {}
variable rancher_admin_bearer_token {}

# Azure credential variables
variable cloud_credential_name {}
variable azure_client_id {}
variable azure_client_secret {}
variable azure_subscription_id {}

# AKS variables
variable cluster_name {}
variable resource_group {}
variable resource_location {}
variable dns_prefix {}
variable network_plugin {}
variable availability_zones {}
variable os_disk_size_gb {}
variable vm_size {}

# Testing variables
variable token_prefix {}
variable config1_expected_node_count {}
variable config1_expected_provider {}
variable config1_expected_state {}
variable config1_expected_kubernetes_version {}
variable config1_expected_rancher_server_version {}
variable config2_expected_node_count {}
variable config2_expected_kubernetes_version {}
variable config3_expected_node_count {}
