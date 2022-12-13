variable "rancher_api_url" {
  type        = string
  default     = ""
  description = "Rancher API URL"
}

variable "rancher_token_key" {
  type        = string
  default     = ""
  description = "Rancher Token Key"
}

variable "aws_ami" {
  type        = string
  default     = ""
  description = "AWS ami for provisioning instances"
}

variable "aws_ami_user" {
  type        = string
  default     = ""
  description = "User for the selected ami (used for ssh provisioning)"
}

variable "aws_subnet" {
  type        = string
  default     = ""
  description = "AWS subnet for instances"
}

variable "aws_security_groups" {
  type        = list
  default     = []
  description = "List of security group ids"
}

variable "aws_vpc_id" {
  type        = string
  default     = ""
  description = "AWS VPC ID"
}

variable "aws_zone" {
  type        = string
  default     = ""
  description = "AWS Zone"
}

variable "aws_root_size" {
  type        = string
  default     = "80"
  description = "AWS Root Size"
}

variable "aws_volume_type" {
  type        = string
  default     = "gp2"
  description = "AWS Volume Type"
}

variable "aws_instance_type" {
  type        = string
  default     = "t3a.xlarge"
  description = "AWS Instance Type"
}

variable "aws_access_key" {
  type        = string
  description = "AWS Access Key"
  sensitive   = true
}

variable "aws_secret_key" {
  type        = string
  description = "AWS Secret Key"
  sensitive   = true
}

variable "aws_region" {
  type        = string
  default     = ""
  description = "AWS Region"
}
variable "cluster_name" {
  type        = string
  default     = "tf-v2charts"
  description = "Cluster name"
}
variable "node_pool_name_cp" {
  type        = string
  default     = "tf-controlplane-0"
  description = "Hostname Prefix for EC2 Instances"
}

variable "node_pool_name_etcd" {
  type        = string
  default     = "tf-etcd-0"
  description = "Hostname Prefix for EC2 Instances"
}

variable "node_pool_name_worker" {
  type        = string
  default     = "tf-worker-0"
  description = "Hostname Prefix for EC2 Instances"
}

variable "worker_count" {
  type        = number
  default     = 3
  description = "Worker count for cluster"
}

variable "rancher_k8s_version" {
  type        = string
  default     = ""
  description = "Kubernetes version"
}

variable "cluster_provider" {
  type        = string
  default     = ""
  description = "Cluster provider"
}
variable "rancher_monitoring_version" {
  type        = string
  default     = ""
  description = "Monitoring version"
}

variable "rancher_alerting_version" {
  type        = string
  default     = ""
  description = "Alerting version"
}
variable "rancher_kiali_version" {
  type        = string
  default     = ""
  description = "Kiali version"
}
variable "rancher_tracing_version" {
  type        = string
  default     = ""
  description = "Rancher Tracing version"
}

variable "rancher_istio_version" {
  type        = string
  default     = ""
  description = "Istio version"
}

variable "rancher_logging_version" {
  type        = string
  default     = ""
  description = "Rancher Logging version"
}

variable "rancher_cis_version" {
  type        = string
  default     = ""
  description = "CIS Benchmark version"
}

variable "rancher_gatekeeper_version" {
  type        = string
  default     = ""
  description = "OPA Gatekeeper version"
}
variable "rancher_backup_version" {
  type        = string
  default     = ""
  description = "Rancher Backups version"
}

variable "rancher_longhorn_version" {
  type        = string
  default     = ""
  description = "Longhorn version"
}

variable "rancher_longhorn_prereq_version" {
  type        = string
  default     = "v1.3.1"
  description = "Longhorn pre-req version"
}

variable "rancher_neuvector_version" {
  type        = string
  default     = ""
  description = "Neuvector version"
}

variable "install_rancher_neuvector" {
  type        = number
  default     = 1
  description = "Not in scope for 2.5x"
}

variable "install_rancher_backups" {
  type        = number
  default     = 1
  description = "rancher_token_key requires admin permissions"
}