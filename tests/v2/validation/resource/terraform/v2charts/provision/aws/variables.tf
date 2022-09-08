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

variable "ami" {
  type        = string
  default     = "ami-001ed96da090fdb2c"
  description = "AWS ami for provisioning instances"
}

variable "ami_user" {
  type        = string
  default     = "ubuntu"
  description = "User for the selected ami (used for ssh provisioning)"
}

variable "subnet" {
  type        = string
  default     = ""
  description = "AWS subnet for instances"
}

variable "security_groups" {
  type        = list
  default     = []
  description = "List of security group ids"
}

variable "node_name" {
  type        = string
  default     = "tf-charts"
  description = "AWS node name"
}

variable "vpc_id" {
  type        = string
  default     = "vpc-bfccf4d7"
  description = "AWS VPC ID"
}

variable "zone" {
  type        = string
  default     = ""
  description = "AWS Zone"
}

variable "root_size" {
  type        = string
  default     = "80"
  description = "AWS Root Size"
}

variable "instance_type" {
  type        = string
  default     = "t3a.xlarge"
  description = "AWS Instance Type"
}

variable "aws_access_key" {
  type        = string
  description = "AWS Access Key"
}

variable "aws_secret_key" {
  type        = string
  description = "AWS Secret Key"
}

variable "aws_region" {
  type        = string
  default     = "us-east-2"
  description = "AWS Region"
}
variable "cluster_name" {
  type        = string
  default     = "aws-tf-v2charts"
  description = "Cluster name"
}
variable "hostname_prefix_cp" {
  type        = string
  default     = "tf-controlplane-0"
  description = "Hostname Prefix for EC2 Instances"
}

variable "hostname_prefix_etcd" {
  type        = string
  default     = "tf-etcd-0"
  description = "Hostname Prefix for EC2 Instances"
}

variable "hostname_prefix_worker" {
  type        = string
  default     = "tf-worker-0"
  description = "Hostname Prefix for EC2 Instances"
}

variable "worker_count" {
  type        = number
  default     = 3
  description = "Worker count for cluster"
}

variable "k8s_version" {
  type        = string
  default     = ""
  description = "Kubernetes version"
}

variable "cluster_provider" {
  type        = string
  default     = ""
  description = "Cluster provider"
}
variable "monitoring_version" {
  type        = string
  default     = ""
  description = "Monitoring version"
}

variable "alerting_version" {
  type        = string
  default     = ""
  description = "Alerting version"
}
variable "kiali_version" {
  type        = string
  default     = ""
  description = "Kiali version"
}
variable "tracing_version" {
  type        = string
  default     = ""
  description = "Rancher Tracing version"
}

variable "istio_version" {
  type        = string
  default     = ""
  description = "Istio version"
}

variable "logging_version" {
  type        = string
  default     = ""
  description = "Rancher Logging version"
}

variable "cis_version" {
  type        = string
  default     = ""
  description = "CIS Benchmark version"
}

variable "gatekeeper_version" {
  type        = string
  default     = ""
  description = "OPA Gatekeeper version"
}
variable "backup_version" {
  type        = string
  default     = ""
  description = "Rancher Backups version"
}

variable "longhorn_version" {
  type        = string
  default     = ""
  description = "Longhorn version"
}

variable "longhorn_prereq_version" {
  type        = string
  default     = "v1.3.1"
  description = "Longhorn pre-req version"
}

variable "neuvector_version" {
  type        = string
  default     = ""
  description = "Neuvector version"
}

variable "rancher_version_26_or_higher" {
  type        = number
  default     = 1
  description = "Controls count for v2 Neuvector resource, not in scope for 2.5x"
}

variable "is_admin" {
  type        = number
  default     = 1
  description = "Count for v2 Rancher Backups resource, local cluster access requires admin permissions"
}