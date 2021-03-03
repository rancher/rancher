variable "region" {
  default = "us-central1"
}

variable "location" {
  default = "us-central1-f"
}

variable "kubernetes_version" {
  default = "1.18.15-gke.1500"
}

variable "disk_size" {
  default = 50
}

variable "machine_type" {
  default = "e2-standard-2"
}

variable "cluster_name" {
  default = "testautogke"
}

variable "credentials" {
  default = ""
}
