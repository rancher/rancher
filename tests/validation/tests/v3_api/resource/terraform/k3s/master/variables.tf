variable "access_key" {}
variable "access_key_name" {}
variable "aws_ami" {}
variable "aws_user" {}
variable "region" {}
variable "qa_space" {}
variable "sg_id" {}
variable "subnets" {}
variable "vpc_id" {}
variable "availability_zone" {}
variable "ec2_instance_class" {}
variable "volume_size" {}
variable "resource_name" {}

variable "username" {}
variable "password" {}
variable "install_mode" {}
variable "k3s_version" {}
variable "k3s_channel" {}
variable "no_of_server_nodes" {}
variable "server_flags" {}
variable "cluster_type" {}
variable "node_os" {}

variable "create_lb" {
  description = "Create Network Load Balancer if set to true"
  type = bool
}

variable "external_db" {}
variable "external_db_version" {}
variable "db_group_name" {}
variable "db_username" {}
variable "db_password" {}
variable "environment" {}
variable "engine_mode" {}
variable "instance_class" {}
variable "max_connections" {}
variable "optional_files" {}