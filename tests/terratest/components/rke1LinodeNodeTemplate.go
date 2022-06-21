package components

var RKE1LinodeNodeTemplate = `resource "rancher2_node_template" "rancher2_node_template" {
	name               = var.node_template_name
	linode_config {
	  image     = var.image
	  region    = var.region
	  root_pass = var.root_pass
	  token     = var.linode_token
	}
  }
`