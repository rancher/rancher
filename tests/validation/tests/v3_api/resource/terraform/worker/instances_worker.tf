resource "aws_instance" "mysql-worker" {
  ami           = "${var.aws_ami}"
  instance_type = "t2.medium"
  count         = var.no_of_worker_nodes
  connection {
    type        = "ssh"
    user        = "${var.aws_user}"
    host        = self.public_ip
    private_key = "${file(var.access_key)}"
  }
  key_name = "jenkins-rke-validation"
  tags          = {
    Name = "${var.resource_name}-multinode-worker"
  }
  provisioner "remote-exec" {
    inline      = [
              "sudo curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=${var.k3s_version} INSTALL_K3S_EXEC=${var.worker_flags} sh -s -  --server https://${local.master_ip}:6443 --token ${local.node_token}"
    ]
  }
}

data "local_file" "master_ip" {
  filename = "/tmp/multinode_ip"
}

locals {
  master_ip = trimspace("${data.local_file.master_ip.content}")
}

output "master_ip" {
  value = "${data.local_file.master_ip.content}"
}

data "local_file" "token" {
  filename = "/tmp/multinode_nodetoken"
}

locals {
  node_token = trimspace("${data.local_file.token.content}")
}

output "node_token" {
  value = "${data.local_file.token.content}"
}

output "public_ip" {
  value = "${aws_instance.mysql-worker.*.public_ip}"
  description = "The public IP of the AWS node"
}
