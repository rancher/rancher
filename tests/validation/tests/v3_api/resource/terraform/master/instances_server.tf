resource "aws_db_instance" "mydb" {
  identifier = "${var.resource_name}"
  allocated_storage    = 20
  storage_type         = "gp2"
  engine               = var.external_db
  engine_version       = var.external_db_version
  instance_class       = var.instance_class
  name                 = "mydb"
  parameter_group_name = var.db_group_name
  username             = var.username
  password             = var.password
  tags = {
    Environment = "dev"
  }
 skip_final_snapshot = true
}

resource "aws_instance" "master-ha" {
  ami           = "${var.aws_ami}"
  instance_type = "t2.medium"
  connection {
    type        = "ssh"
    user        = "${var.aws_user}"
    host        = self.public_ip
    private_key = "${file(var.access_key)}"
  }
  key_name = "jenkins-rke-validation"
  tags = {
    Name = "${var.resource_name}-multinode-server"
  }
  provisioner "remote-exec" {
    inline = [
              "sudo curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=${var.k3s_version} INSTALL_K3S_EXEC=${var.server_flags} sh -s - --datastore-endpoint='mysql://${aws_db_instance.mydb.username}:${aws_db_instance.mydb.password}@tcp(${aws_db_instance.mydb.endpoint})/${aws_db_instance.mydb.name}'",
              "sudo cat /var/lib/rancher/k3s/server/node-token >/tmp/multinode_nodetoken",
              "sudo cat /etc/rancher/k3s/k3s.yaml >/tmp/multinode_kubeconfig",
    ]
  }
  provisioner "local-exec" {
    command = "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ${var.access_key} ${var.aws_user}@${aws_instance.master-ha.public_ip}:/tmp/multinode_nodetoken /tmp/"
  } 
  provisioner "local-exec" {
    command = "echo ${aws_instance.master-ha.public_ip} >/tmp/multinode_ip"
  }
  provisioner "local-exec" {
    command = "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ${var.access_key} ${var.aws_user}@${aws_instance.master-ha.public_ip}:/tmp/multinode_kubeconfig /tmp/"
  }
}

resource "aws_instance" "master2-ha" {
  ami           = "${var.aws_ami}"
  instance_type = "t2.medium"
  count         = var.no_of_server_nodes
  connection {
    type        = "ssh"
    user        = "${var.aws_user}"
    host        = self.public_ip
    private_key = "${file(var.access_key)}"
  }
  key_name = "jenkins-rke-validation"
  tags = {
    Name = "${var.resource_name}-multinode-server"
  }
  provisioner "remote-exec" {
    inline = [
              "sudo curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=${var.k3s_version} INSTALL_K3S_EXEC=${var.server_flags} sh -s - --datastore-endpoint='mysql://${aws_db_instance.mydb.username}:${aws_db_instance.mydb.password}@tcp(${aws_db_instance.mydb.endpoint})/${aws_db_instance.mydb.name}'",
    ]
  }
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment" {
  target_group_arn = "${aws_lb_target_group.aws_tg.arn}"
  target_id        = "${aws_instance.master-ha.id}"
  port             = 6443
  depends_on       = ["aws_instance.master-ha"]
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment2" {
  target_group_arn = "${aws_lb_target_group.aws_tg.arn}"
  count            = length(aws_instance.master2-ha)
  target_id        = "${aws_instance.master2-ha[count.index].id}"
  port             = 6443
  depends_on       = ["aws_instance.master-ha"]
}

resource "aws_lb_target_group" "aws_tg" {
  port             = 6443
  protocol         = "TCP"
  vpc_id           = "${var.vpc_id}"
  name             = "${var.resource_name}-multinode-tg"
}

resource "aws_lb" "aws_nlb" {
  internal           = false
  load_balancer_type = "network"
  subnets            = ["${var.subnets}"] 
  name               = "${var.resource_name}-multinode-nlb"
}

resource "aws_lb_listener" "aws_nlb_listener" {
  load_balancer_arn = "${aws_lb.aws_nlb.arn}"
  port              = "6443"
  protocol          = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.aws_tg.arn}"
  }
}

resource "aws_route53_record" "aws_route53" {
  zone_id            = "${data.aws_route53_zone.selected.zone_id}"
  name               = "${var.resource_name}-multinode-route53"
  type               = "CNAME"
  ttl                = "300"
  records            = ["${aws_lb.aws_nlb.dns_name}"]
  depends_on         = ["aws_lb_listener.aws_nlb_listener"]
}

data "aws_route53_zone" "selected" {
  name               = "${var.qa_space}"
  private_zone       = false
}

resource "null_resource" "update_kubeconfig" {
  provisioner "local-exec" {
    command = "sed s/127.0.0.1/\"${aws_route53_record.aws_route53.fqdn}\"/g /tmp/multinode_kubeconfig>/tmp/multinode_kubeconfig1"
  }
  depends_on = ["aws_instance.master-ha"]
}

resource "null_resource" "store_fqdn" {
  provisioner "local-exec" {
    command = "echo \"${aws_route53_record.aws_route53.fqdn}\" >/tmp/multinode_ip"
  }
  depends_on = ["aws_instance.master-ha"]
}

output "Route53_info" {
  value       = aws_route53_record.aws_route53.*
  description = "List of DNS records"
}

output "db_instance_name" {
  value = "${aws_db_instance.mydb.name}"
}

output "db_instance_username" {
  value = "${aws_db_instance.mydb.username}"
}

output "db_instance_password" {
  value = "${aws_db_instance.mydb.password}"
}

output "rds_instance_endpoint" {
  value = "${aws_db_instance.mydb.endpoint}"
}

output "hostnames" {
  value       = aws_route53_record.aws_route53.*.fqdn
  description = "List of DNS records"
}

output "public_ip" {
  value = "${aws_instance.master-ha.*.public_ip}"
  description = "The public IP of the AWS node"
}
