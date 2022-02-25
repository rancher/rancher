variable "rancher_api_url" {
  type    = string
  default = ""
}

variable "rancher_token_key" {
  type    = string
  default = ""
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