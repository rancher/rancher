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

variable "logging_endpoint" {
  type        = string
  default     = ""
  description = "Logging endpoint"
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

variable "istio_version" {
  type        = string
  default     = "1.5.901"
  description = "Istio version"
}

variable "longhorn_prereq_version" {
  type        = string
  default     = "v1.2.4"
  description = "Longhorn pre-req version"
}