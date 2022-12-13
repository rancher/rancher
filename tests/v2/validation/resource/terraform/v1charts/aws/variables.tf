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
  default     = "ami-0452941f41a1b0608"
  description = "AWS ami for provisioning instances"
}

variable "ami_user" {
  type        = string
  default     = "ec2-user"
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
  default     = ""
  description = "AWS Access Key"
}

variable "aws_secret_key" {
  type        = string
  default     = ""
  description = "AWS Secret Key"
}

variable "aws_region" {
  type        = string
  default     = "us-east-2"
  description = "AWS Region"
}
variable "cluster_name" {
  type        = string
  default     = "aws-tf-v1charts"
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
  default     = "v1.20.15-rancher1-1"
  description = "Kubernetes version"
}

variable "logging_endpoint" {
  type        = string
  default     = ""
  description = "Logging endpoint"
}

variable "istio_version" {
  type        = string
  default     = "1.5.901"
  description = "Istio version"
}

variable "longhorn_prereq_version" {
  type        = string
  default     = "v1.2.4"
  description = "Longhorn prereq version"
}