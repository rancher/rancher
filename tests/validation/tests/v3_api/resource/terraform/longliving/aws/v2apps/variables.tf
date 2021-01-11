variable "ssh_keys" {
  type        = list
  default     = []
  description = "SSH keys to inject into the EC2 instances"
}

variable "ami" {
  type        = string
  default     = ""
  description = "ami to use"
}

variable "cattle_test_url" {
  type        = string
  default     = ""
  description = "rancher instance to use"
}

variable "admin_token" {
  type        = string
  default     = ""
  description = "admin token to use"
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

variable "path_to_key" {
  type        = string
  default     = ""
  description = "absolute path to pem key for ssh"
}

variable "ssh_key_name" {
  type        = string
  default     = ""
  description = "AWS ssh key name"
}

variable "node_name" {
  type        = string
  default     = "rancher-load"
  description = "AWS ssh key name"
}

variable "minio_ca" {
  type        = string
  default     = ""
  description = "Base64 encoded CA cert for Minio (s3 backup)"
}

variable minio_endpoint {}
variable minio_access_key {}
variable minio_secret_key {}