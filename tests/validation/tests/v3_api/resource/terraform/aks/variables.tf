variable "kubernetes_version" {
  description = "Azure Kubernetes Cluster K8s version"
  default = "1.19.6"
}

variable "location" {
  description = "Azure Kubernetes location (US Central, US East, etc)"
  default = "Central US"
}

variable "node_count" {
  description = "Number of nodes in the Cluster"
  default = 3
}

variable "vm_size" {
  description = "VM Size for the nodes in the cluster"
  default = "Standard_D2_v2"
}

variable "disk_capacity" {
  description = "VM System Volume size"
  default = 30
}
variable "client_id" {
  default = ""
}
variable "client_secret" {
  default = ""
}

variable "cluster_name" {
  default = "testautoaks"
}
