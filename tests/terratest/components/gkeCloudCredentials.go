package components

var GkeCloudCredentials = `resource "rancher2_cloud_credential" "rancher2_cloud_credential" {
  name = var.cloud_credential_name
  google_credential_config {
	auth_encoded_json = jsonencode(var.google_auth_encoded_json)
  }
}
`