package components

var AzureCloudCredentials = `resource "rancher2_cloud_credential" "rancher2_cloud_credential" {
  name = var.cloud_credential_name
  azure_credential_config {
  client_id       = var.azure_client_id
	client_secret   = var.azure_client_secret
	subscription_id = var.azure_subscription_id
  }
}

`