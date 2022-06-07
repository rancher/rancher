package components

var RKE1NodeTemplate = `resource "rancher2_node_template" "rancher2_node_template" {
  name = var.node_template_name
  amazonec2_config {
    access_key     = var.aws_access_key
	  secret_key     = var.aws_secret_key
	  ami            = var.aws_ami_w_docker
	  region         = var.aws_region
	  security_group = [var.aws_security_group_name]
	  subnet_id      = var.aws_subnet_id
	  vpc_id         = var.aws_vpc_id
	  zone           = var.aws_zone_letter
	  root_size      = var.aws_root_size
	  instance_type  = var.aws_instance_type
  }
}

`