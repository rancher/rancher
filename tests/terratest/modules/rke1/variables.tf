# Rancher specific variable section.
variable rancher_api_url {}
variable rancher_admin_bearer_token {}
variable cloud_credential_name {}

# AWS specific variables.
variable aws_access_key {}
variable aws_secret_key {}
variable aws_ami_w_docker {}
variable aws_instance_type {}
variable aws_root_size {}
variable aws_region {}
variable aws_security_group_name {}
variable aws_subnet_id {}
variable aws_vpc_id {}
variable aws_zone_letter {}

# RKE1 specific variables.
variable cluster_name {}
variable network_plugin {}
variable node_template_name {}
variable node_hostname_prefix {}

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