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
variable expected_node_count_3 {}
variable expected_provider {}
variable expected_state_active {}
variable expected_kubernetes_version_12210 {}
variable expected_node_count_8 {}
variable expected_kubernetes_version_1237 {}
variable expected_node_count_6 {} 