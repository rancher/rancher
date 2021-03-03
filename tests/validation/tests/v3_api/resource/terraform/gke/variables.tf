variable "region" {
  default = "us-central1"
}

variable "location" {
  default = "us-central1-f"
}

variable "kubernetes_version" {
  default = "1.20.8-gke.2100"
}

variable "disk_size" {
  default = 50
}

variable "machine_type" {
  default = "e2-standard-2"
}

variable "cluster_name" {
  default = "qaautogke"
}

variable "credentials" {
  default = ""
}

