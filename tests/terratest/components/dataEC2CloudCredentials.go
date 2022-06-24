package components

var DataEC2CloudCredentials = `data "rancher2_cloud_credential" "rancher2_cloud_credential" {
  name = var.cloud_credential_name
}

`