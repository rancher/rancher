variable "rancher_api_url" {
  type    = string
  default = ""
}
variable "rancher_token_key" {
  type    = string
  default = ""
}
variable "do_access_token" {
  type    = string
  default = ""
}
variable "region" {
  type        = string
  default     = "sfo3"
  description = "DO zone"
}
variable "size" {
  type        = string
  default     = "s-4vcpu-8gb"
  description = "DO instance size"
}
variable "image" {
  type        = string
  default     = "ubuntu-20-04-x64"
  description = "DO image for droplet"
}
variable "cluster_name" {
  type        = string
  default     = "do-tf-v1charts"
  description = "Cluster name"
}
variable "hostname_prefix_cp" {
  type        = string
  default     = "tf-controlplane-0"
  description = "Hostname Prefix for control plane droplet"
}
variable "hostname_prefix_etcd" {
  type        = string
  default     = "tf-etcd-0"
  description = "Hostname Prefix for etcd droplet"
}

variable "hostname_prefix_worker" {
  type        = string
  default     = "tf-worker-0"
  description = "Hostname Prefix for worker droplet"
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

variable "logging_endpoint" {
  type        = string
  default     = ""
  description = "Logging endpoint url"
}