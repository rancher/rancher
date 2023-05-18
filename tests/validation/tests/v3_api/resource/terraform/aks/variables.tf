variable "kubernetes_version" {
  description = "Azure Kubernetes Cluster K8s version"
  default = "1.23.5"
}

variable "location" {
  description = "Azure Kubernetes location (US Central, US East, etc)"
  default = "East US 2"
}

variable "node_count" {
  description = "Number of nodes in the Cluster"
  default = 3
}

variable "vm_size" {
  description = "VM Size for the nodes in the cluster"
  default = "Standard_D3_v2"
}

variable "disk_capacity" {
  description = "VM System Volume size"
  default = 30
}

variable "cluster_name" {
  default = "testautoaks"
}

variable "sku_tier" {
  default = "Paid"
}