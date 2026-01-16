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
variable "docker_org_token_username" {}
variable "docker_org_token_password" {}
variable "ec2_instance_class" {}
variable "volume_size" {}
variable "iam_role" {}
variable "node_os" {}
variable "no_of_worker_nodes" {}
variable "password" {
  default = "password"
}
variable "region" {}
variable "resource_name" {}
variable "rke2_version" {}
variable "install_mode" {}
variable "install_method" {}
variable "rke2_channel" {}
variable "sg_id" {}
variable "subnets" {}
variable "username" {
  default = "username"
}
variable "vpc_id" {}
variable "worker_flags" {}
