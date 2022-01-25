variable "access_key" {}
variable "availability_zone" {}
variable "aws_ami" {}
variable "aws_user" {}
variable "cluster_type" {}
variable "ec2_instance_class" {}
variable "volume_size" {}
variable "iam_role" {}
variable "node_os" {}
variable "no_of_server_nodes" {}
variable "password" {
  default = "password"
}
variable "qa_space" {}
variable "region" {}
variable "resource_name" {}
variable "rke2_version" {}
variable "rke2_channel" {}
variable "server_flags" {}
variable "sg_id" {}
variable "subnets" {}
variable "vpc_id" {}
variable "username" {
  default = "username"
}
variable "create_lb" {
  description = "Create Network Load Balancer if set to true"
  type = bool
}
