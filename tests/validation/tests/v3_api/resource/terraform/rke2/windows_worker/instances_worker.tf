resource "aws_instance" "windows_worker" {
  depends_on = [
    var.dependency
  ]
  ami                    = var.aws_ami
  instance_type          = var.ec2_instance_class
  count                  = var.no_of_windows_worker_nodes
  iam_instance_profile   = "${var.iam_role}"
  get_password_data      = true
  user_data              = templatefile("join-rke2.tftpl", {serverIP: "${local.master_ip}", token: "${local.node_token}", version: "${var.rke2_version}"})
  subnet_id              = var.subnets
  availability_zone      = var.availability_zone
  vpc_security_group_ids = ["${var.sg_id}"]
  key_name               = var.access_key_name
  tags = {
    Name = "${var.resource_name}-windows-worker"
    "kubernetes.io/cluster/clusterid" = "owned"
  }
}

data "local_file" "master_ip" {
  depends_on = [var.dependency]
  filename = "/tmp/${var.resource_name}_master_ip"
}

locals {
  master_ip = trimspace("${data.local_file.master_ip.content}")
}

data "local_file" "token" {
  depends_on = [var.dependency]
  filename = "/tmp/${var.resource_name}_nodetoken"
}

locals {
  node_token = trimspace("${data.local_file.token.content}")
}
