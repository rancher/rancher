resource "aws_db_instance" "mydb" {
  identifier = "${var.resource_name}-multinode-db"
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

resource "aws_instance" "master" {
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
              "sudo cat /var/lib/rancher/k3s/server/node-token >/tmp/multinode_nodetoken",
              "sudo cat /etc/rancher/k3s/k3s.yaml >/tmp/multinode_kubeconfig",
    ]
  }
  provisioner "local-exec" {
    command = "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ${var.access_key} ${var.aws_user}@${aws_instance.master[count.index].public_ip}:/tmp/multinode_nodetoken /tmp/"
  } 
  provisioner "local-exec" {
    command = "echo ${aws_instance.master[count.index].public_ip} >/tmp/multinode_ip"
  }
  provisioner "local-exec" {
    command = "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ${var.access_key} ${var.aws_user}@${aws_instance.master[count.index].public_ip}:/tmp/multinode_kubeconfig /tmp/"
  }
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_80" {
  target_group_arn = "${aws_lb_target_group.aws_tg_80.arn}"
  count            = length(aws_instance.master)
  target_id        = "${aws_instance.master[count.index].id}"
  port             = 80
  depends_on       = ["aws_instance.master"]
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_443" {
  target_group_arn = "${aws_lb_target_group.aws_tg_443.arn}"
  count            = length(aws_instance.master)
  target_id        = "${aws_instance.master[count.index].id}"
  port             = 443
  depends_on       = ["aws_instance.master"]
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_6443" {
  target_group_arn = "${aws_lb_target_group.aws_tg_6443.arn}"
  count            = length(aws_instance.master)
  target_id        = "${aws_instance.master[count.index].id}"
  port             = 6443
  depends_on       = ["aws_instance.master"]
}

resource "aws_lb_target_group" "aws_tg_80" {
  port             = 80
  protocol         = "TCP"
  vpc_id           = "${var.vpc_id}"
  name             = "${var.resource_name}-k3s-tg-80"
}

resource "aws_lb_target_group" "aws_tg_443" {
  port             = 443
  protocol         = "TCP"
  vpc_id           = "${var.vpc_id}"
  name             = "${var.resource_name}-k3s-tg-443"
}

resource "aws_lb_target_group" "aws_tg_6443" {
  port             = 6443
  protocol         = "TCP"
  vpc_id           = "${var.vpc_id}"
  name             = "${var.resource_name}-k3s-tg-6443"
}

resource "aws_lb" "aws_nlb" {
  internal           = false
  load_balancer_type = "network"
  subnets            = ["${var.subnets}"] 
  name               = "${var.resource_name}-multinode-nlb"
}

resource "aws_lb_listener" "aws_nlb_listener_80" {
  load_balancer_arn = "${aws_lb.aws_nlb.arn}"
  port              = "80"
  protocol          = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.aws_tg_80.arn}"
  }
}

resource "aws_lb_listener" "aws_nlb_listener_443" {
  load_balancer_arn = "${aws_lb.aws_nlb.arn}"
  port              = "443"
  protocol          = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.aws_tg_443.arn}"
  }
}

resource "aws_lb_listener" "aws_nlb_listener_6443" {
  load_balancer_arn = "${aws_lb.aws_nlb.arn}"
  port              = "6443"
  protocol          = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.aws_tg_6443.arn}"
  }
}


resource "aws_route53_record" "aws_route53" {
  zone_id            = "${data.aws_route53_zone.selected.zone_id}"
  name               = "${var.resource_name}-multinode-route53"
  type               = "CNAME"
  ttl                = "300"
  records            = ["${aws_lb.aws_nlb.dns_name}"]
  depends_on         = ["aws_lb_listener.aws_nlb_listener_6443"]
}

data "aws_route53_zone" "selected" {
  name               = "${var.qa_space}"
  private_zone       = false
}

resource "null_resource" "update_kubeconfig" {
  provisioner "local-exec" {
    command = "sed s/127.0.0.1/\"${aws_route53_record.aws_route53.fqdn}\"/g /tmp/multinode_kubeconfig>/tmp/multinode_kubeconfig1"
  }
  depends_on = ["aws_instance.master"]
}

resource "null_resource" "store_fqdn" {
  provisioner "local-exec" {
    command = "echo \"${aws_route53_record.aws_route53.fqdn}\" >/tmp/multinode_ip"
  }
  depends_on = ["aws_instance.master"]
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
  value = "${aws_instance.master.*.public_ip}"
  description = "The public IP of the AWS node"
}
