variable "aws_ami" {}
variable "aws_user" {}
variable "region" {}
variable "access_key" {}
variable "vpc_id" {}
variable "subnets" {}
variable "qa_space" {}
variable "resource_name" {}
variable "rke2_version" {}
variable "server_flags" {}
variable "ec2_instance_class" {}
variable "availability_zone" {}
variable "sg_id" {}
variable "no_of_server_nodes" {}
variable "username" {
  default = "username"
}
variable "password" {
  default = "password"
}
variable "ctype" {
  default = "centos"
}
variable "iam_role" {
  default = ""
  type = string
}
