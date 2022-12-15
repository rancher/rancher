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

variable "install_rancher_backups" {
  type        = number
  default     = 1
  description = "rancher_token_key requires admin permissions, controls resource count"
}

variable "cluster_id" {
  type        = string
  default     = ""
  description = "Format: c-xxxxx or local"
}

variable "project_id" {
  type        = string
  default     = ""
  description = "Format: c-xxxxx:p-xxxxx"
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

variable "rancher_kiali_version" {
  type        = string
  default     = ""
  description = "Kiali version"
}

variable "rancher_tracing_version" {
  type        = string
  default     = ""
  description = "Tracing version"
}

variable "rancher_istio_version" {
  type        = string
  default     = ""
  description = "Istio version"
}

variable "rancher_logging_version" {
  type        = string
  default     = ""
  description = "Logging version"
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

variable "rancher_alerting_version" {
  type        = string
  default     = ""
  description = "Alerting version"
}

variable "rancher_neuvector_version" {
  type        = string
  default     = ""
  description = "Neuvector version"
}

variable "install_rancher_neuvector" {
  type        = number
  default     = 1
  description = "Not in scope for 2.5x, controls resource count"
}

variable "neuvector_docker_runtime" {
  type        = bool
  default     = true
  description = "docker runtime for v2 Neuvector resource, true for RKE1"
}

variable "neuvector_k3s_runtime" {
  type        = bool
  default     = false
  description = "k3s runtime for v2 Neuvector resource, true for RKE2"
}

variable "neuvector_crio_runtime" {
  type        = bool
  default     = false
  description = "cri-o runtime for v2 Neuvector resource"
}

variable "neuvector_containerd_runtime" {
  type        = bool
  default     = false
  description = "containerd runtime for v2 Neuvector resource"
}