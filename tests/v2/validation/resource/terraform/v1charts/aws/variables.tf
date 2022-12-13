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

variable "aws_instance_type" {
  type        = string
  default     = "t3.xlarge"
  description = "AWS Instance Type"
}

variable "aws_volume_type" {
  type        = string
  default     = "gp2"
  description = "AWS Volume Type"
}

variable "aws_access_key" {
  type        = string
  default     = ""
  description = "AWS Access Key"
  sensitive   = true
}

variable "aws_secret_key" {
  type        = string
  default     = ""
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
  default     = "aws-tf-v1charts"
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
  default     = "v1.24.13-rancher2-1"
  description = "Kubernetes version"
}

variable "rancher_longhorn_prereq_version" {
  type        = string
  default     = "v1.2.6"
  description = "Longhorn prereq version"
}