resource "aws_instance" "worker" {
  depends_on = [
    var.dependency
  ]
  ami                    = var.aws_ami
  instance_type          = var.ec2_instance_class
  count                  = var.no_of_worker_nodes
  connection {
    type                 = "ssh"
    user                 = var.aws_user
    host                 = self.public_ip
    private_key          = file(var.access_key)
  }
  root_block_device {
    volume_size          = var.volume_size
    volume_type          = "standard"
  }
  subnet_id              = var.subnets
  availability_zone      = var.availability_zone
  vpc_security_group_ids = [var.sg_id]
  key_name               = var.access_key_name
  tags = {
    Name                 = "${var.resource_name}-worker"
  }
  provisioner "file" {
    source = "join_k3s_agent.sh"
    destination = "/tmp/join_k3s_agent.sh"
  }
  provisioner "file" {
    source = "cis_workerconfig.yaml"
    destination = "/tmp/cis_workerconfig.yaml"
  }


  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/join_k3s_agent.sh",
      "sudo /tmp/join_k3s_agent.sh ${var.node_os} ${var.install_mode} ${var.k3s_version} ${local.master_ip} ${local.node_token} ${self.public_ip} \"${var.worker_flags}\" ${var.username} ${var.password} ${var.k3s_channel} ",
    ]
  }
}

data "local_file" "master_ip" {
  depends_on = [var.dependency]
  filename = "/tmp/${var.resource_name}_master_ip"
}

locals {
  master_ip = trimspace(data.local_file.master_ip.content)
}

data "local_file" "token" {
  depends_on = [var.dependency]
  filename = "/tmp/${var.resource_name}_nodetoken"
}

locals {
  node_token = trimspace(data.local_file.token.content)
}