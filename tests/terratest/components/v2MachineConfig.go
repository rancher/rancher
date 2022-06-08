package components

var V2MachineConfig = `resource "rancher2_machine_config_v2" "rancher2_machine_config_v2" {
  generate_name = var.machine_config_name
  amazonec2_config {
	ami            = var.aws_ami
	region         = var.aws_region
	security_group = [var.aws_security_group_name]
	subnet_id      = var.aws_subnet_id
	vpc_id         = var.aws_vpc_id
	zone           = var.aws_zone_letter
  }
}

`