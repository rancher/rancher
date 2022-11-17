package components

var V2LinodeMachineConfig = `resource "rancher2_machine_config_v2" "rancher2_machine_config_v2" {
  generate_name = var.machine_config_name
  linode_config {
	image     = var.image
	region    = var.region
	root_pass = var.root_pass
	token     = var.linode_token
  }
}
` 