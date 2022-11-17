# Rancher specific variables
variable rancher_api_url {
  description = "host url to Rancher server"
}
variable rancher_admin_bearer_token {
  description = "rancher admin bearer token"
}

# AWS specific variables
variable aws_access_key {
  description = "AWS Access Key"
}
variable aws_secret_key {
  description = "AWS Secret Key"
}
variable cloud_credential_name {
  description = "Unique name for cloud credentials, which will be created"
}
variable aws_instance_type {
  description = "AWS Instance Type"
}
variable aws_region {
  description = "AWS Region"
}
variable aws_subnets {
  description = "AWS Subnets - format each subnet in quotes and separate with commas"
}
variable aws_security_groups {
  description = "AWS Security Groups- format each security group in quotes and separate with commas"
}

# EKS specific variables
variable cluster_name {
  description = "Unique name for the cluster, which will be created"
}
variable hostname_prefix {
    description = "Hostname prefix for the cluster resources, which will be created"
}
variable public_access {
    description = "Enable public access to the cluster: 'true' or 'false'"
}
variable private_access {
    description = "Enable private access to the cluster: 'true' or 'false'"
}