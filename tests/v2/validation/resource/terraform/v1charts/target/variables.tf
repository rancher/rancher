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

variable "rancher_longhorn_prereq_version" {
  type        = string
  default     = "v1.2.6"
  description = "Longhorn pre-req version"
}