variable "aws_ami" {}
variable "aws_user" {}
variable "region" {}
variable "subnets" {}
variable "vpc_id" {}
variable "rke2_version" {}
variable "ec2_instance_class" {}
variable "access_key" {}
variable "no_of_worker_nodes" {}
variable "worker_flags" {}
variable "resource_name" {}
variable "dependency" {
  type    = any
  default = null
}
variable "availability_zone" {}
variable "sg_id" {}
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
