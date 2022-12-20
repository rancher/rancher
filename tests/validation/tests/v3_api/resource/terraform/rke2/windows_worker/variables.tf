variable "access_key" {}
variable "access_key_name" {}
variable "availability_zone" {}
variable "aws_ami" {}
variable "aws_user" {}
variable "cluster_type" {}
variable "dependency" {
  type    = any
  default = null
}
variable "ec2_instance_class" {}
variable "iam_role" {}
variable "node_os" {}
variable "no_of_windows_worker_nodes" {}
variable "password" {
  default = "password"
}
variable "region" {}
variable "resource_name" {}
variable "rke2_version" {}
variable "sg_id" {}
variable "subnets" {}
variable "username" {
  default = "username"
}
variable "vpc_id" {}
