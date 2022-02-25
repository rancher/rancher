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