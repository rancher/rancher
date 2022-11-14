# Rancher specific variables
variable rancher_api_url {}
variable rancher_admin_bearer_token {}

# GKE Variables
variable "google_auth_encoded_json" {}
variable "cloud_credential_name" {}
variable "cluster_name" {}
variable "gke_region" {}
variable "gke_project_id" {}
variable "gke_network" {}
variable "gke_subnetwork" {}
variable "hostname_prefix" {}