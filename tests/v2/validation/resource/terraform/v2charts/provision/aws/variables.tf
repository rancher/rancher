variable "rancher_api_url" {
  type    = string
  default = ""
}

variable "rancher_token_key" {
  type    = string
  default = ""
}

variable "ami" {
  type        = string
  default     = "ami-001ed96da090fdb2c"
  description = "ami to use"
}

variable "subnet" {
  type        = string
  default     = ""
  description = "subnet to use"
}

variable "security_groups" {
  type        = list
  default     = []
  description = "security group ids to use"
}

variable "ami_user" {
  type        = string
  default     = "ubuntu"
  description = "User for the ami (used for ssh provisioning)"
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
  default     = "t3.xlarge"
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
  description = "Kubernetes Version"
}

variable "cluster_id" {
  default = ""
}
variable "monitoring_version" {
  default = ""
}

variable "kiali_version" {
  default = ""
}

variable "istio_version" {
  default = ""
}

variable "logging_version" {
  default = ""
}

variable "cis_version" {
  default = ""
}

variable "gatekeeper_version" {
  default = ""
}
variable "backup_version" {
  default = ""
}

variable "longhorn_version" {
  default = ""
}

variable "project_id" {
  default = ""
}

variable "cluster_provider" {
  default = ""
}