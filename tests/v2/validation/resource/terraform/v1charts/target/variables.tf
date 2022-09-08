variable "rancher_api_url" {
  type    = string
  default = ""
}

variable "rancher_token_key" {
  type    = string
  default = ""
}

variable "logging_endpoint" {
  type        = string
  default     = ""
  description = "Default loggin endpoint"
}

variable "cluster_id" {
  default = ""
}

variable "project_id" {
  default = ""
}

variable "istio_version" {
  type        = string
  default     = "1.5.901"
  description = "Default Istio version"
}