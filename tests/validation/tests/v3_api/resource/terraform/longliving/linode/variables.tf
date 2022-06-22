variable "linode_token" {
  type        = string
  description = "Linode Token"
}

variable "linode_resource_prefix" {
  type        = string
  description = "Linode Resource Prefix"
}

variable "linode_root_password" {
  type        = string
  description = "root password to use by default in linodes"
}

variable "ssh_authorized_key" {
  type        = string
  description = "Public SSH key to add into linodes"
}

variable "ssh_private_key" {
  type        = string
  description = "The private key ssh key path to retrieve the kubeconfig and token from k3s"
}

variable "docker_version" {
  type        = string
  default     = "20.10"
  description = "The default Docker version to use for the long living environment"
}

variable "rancher_version_super" {
  type        = string
  default     = "2.6.5"
  description = "The Super Rancher version"
}

variable "rancher_version_hosted1" {
  type        = string
  default     = "2.6.8"
  description = "The Hosted Rancher version, set same as super if not using different branches"
}

variable "rancher_version_hosted2" {
  type        = string
  default     = "2.5.14"
  description = "The Hosted Rancher version, set same as super if not using different branches"
}

variable "rancher_version_hosted3" {
  type        = string
  default     = "2.6.8"
  description = "The Hosted Rancher version, set same as super if not using different branches"
}

variable "rke2_cluster_version_hosted1" {
  type        = string
  default     = "v1.21.4+rke2r2"
  description = "The kubernetes version for the downstream cluster on the hosted1 if Rancher 2.6+"
}

variable "rke2_cluster_version_hosted2" {
  type        = string
  default     = "v1.21.4+rke2r2"
  description = "The kubernetes version for the downstream cluster on the hosted2 if Rancher 2.6+"
}

variable "rke2_cluster_version_hosted3" {
  type        = string
  default     = "v1.21.4+rke2r2"
  description = "The kubernetes version for the downstream cluster on the hosted3 if Rancher 2.6+"
}

variable "k3s_version_super" {
  type        = string
  default     = "v1.22.9+k3s1"
  description = "The Super Rancher version"
}

variable "k3s_version_hosted1" {
  type        = string
  default     = "v1.22.9+k3s1"
  description = "The Hosted 1 k3s version"
}

variable "k3s_version_hosted2" {
  type        = string
  default     = "v1.20.12+k3s1"
  description = "The Hosted 2 k3s version"
}

variable "k3s_version_hosted3" {
  type        = string
  default     = "v1.20.12+k3s1"
  description = "The Hosted 2 k3s version"
}

variable "rancher_password" {
  type        = string
  description = "The password that will be used to bootstrap Rancher"
}

variable "rancher_github_client_id_super" {
  type        = string
  description = "The client id to enable github authentication in the super Rancher"
}

variable "rancher_github_client_secret_super" {
  type        = string
  description = "The secret id to enable github authentication in the super Rancher"
}

variable "rancher_github_client_id_hosted1" {
  type        = string
  description = "The client id to enable github authentication in the Hosted1 Rancher"
}

variable "rancher_github_client_secret_hosted1" {
  type        = string
  description = "The secret id to enable github authentication in the Hosted1 Rancher"
}

variable "rancher_github_client_id_hosted2" {
  type        = string
  description = "The client id to enable github authentication in the Hosted2 Rancher"
}

variable "rancher_github_client_secret_hosted2" {
  type        = string
  description = "The secret id to enable github authentication in the Hosted2 Rancher"
}

variable "rancher_github_client_id_hosted3" {
  type        = string
  description = "The client id to enable github authentication in the Hosted3 Rancher"
}

variable "rancher_github_client_secret_hosted3" {
  type        = string
  description = "The secret id to enable github authentication in the Hosted3 Rancher"
}

variable "zone_id" {
  type        = string
  description = "The zone id to create DNS records on"
}

variable "load_balancers_domain" {
  type        = string
  description = "Rancher load balancers domain"
}

variable "super_load_balancer_subdomain" {
  type        = string
  description = "Super Rancher load balancer subdomain"
}

variable "hosted1_load_balancer_subdomain" {
  type        = string
  description = "Rancher load balancer subdomain of the hosted1 cluster"
}

variable "hosted2_load_balancer_subdomain" {
  type        = string
  description = "Rancher load balancer subdomain of the hosted2 cluster"
}

variable "hosted3_load_balancer_subdomain" {
  type        = string
  description = "Rancher load balancer subdomain of the hosted3 cluster"
}

variable "fullchain" {
  type        = string
  description = "The base64 fullchain certificate"
}

variable "privkey" {
  type        = string
  description = "The base64 private key certificate"
}