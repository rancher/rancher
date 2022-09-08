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

variable "is_admin" {
  type        = number
  default     = 1
  description = "Count for v2 Rancher Backups resource, local cluster access requires admin permissions for token key"
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
variable "monitoring_version" {
  type        = string
  default     = ""
  description = "Monitoring version"
}

variable "kiali_version" {
  type        = string
  default     = ""
  description = "Kiali version"
}

variable "tracing_version" {
  type        = string
  default     = ""
  description = "Tracing version"
}

variable "istio_version" {
  type        = string
  default     = ""
  description = "Istio version"
}

variable "logging_version" {
  type        = string
  default     = ""
  description = "Logging version"
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

variable "alerting_version" {
  type        = string
  default     = ""
  description = "Alerting version"
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

variable "docker_install" {
  type        = string
  default     = "true"
  description = "Controls cluster type for v2 Neuvector resource, true for RKE1"
}

variable "k3s_install" {
  type        = string
  default     = "false"
  description = "Controls cluster type for v2 Neuvector resource, true for RKE2"
}