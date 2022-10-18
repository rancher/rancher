resource "aws_instance" "master" {
  ami           =  var.aws_ami
  instance_type =  var.ec2_instance_class
  iam_instance_profile = "${var.iam_role}"
  connection {
    type        = "ssh"
    user        = "${var.aws_user}"
    host        = self.public_ip
    private_key = "${file(var.access_key)}"
  }
  root_block_device {
    volume_size = var.volume_size
    volume_type = "standard"
  }
  subnet_id = var.subnets
  availability_zone = var.availability_zone
  vpc_security_group_ids = ["${var.sg_id}"]
  key_name = "jenkins-rke-validation"
  tags = {
    Name = "${var.resource_name}-server"
    "kubernetes.io/cluster/clusterid" = "owned"
  }
  provisioner "file" {
    source      = "define_node_role.sh"
    destination = "/tmp/define_node_role.sh"
  }
  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/define_node_role.sh",
      "sudo /tmp/define_node_role.sh -1 \"${var.role_order}\" ${var.all_role_nodes} ${var.etcd_only_nodes} ${var.etcd_cp_nodes} ${var.etcd_worker_nodes} ${var.cp_only_nodes} ${var.cp_worker_nodes}",
    ]
  }
  provisioner "file" {
    source      = "install_rke2_master.sh"
    destination = "/tmp/install_rke2_master.sh"
  }
  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/install_rke2_master.sh",
      "sudo /tmp/install_rke2_master.sh ${var.node_os} ${var.create_lb ? aws_route53_record.aws_route53[0].fqdn : "fake.fqdn.value"} ${var.rke2_version} ${self.public_ip} ${var.rke2_channel} ${var.cluster_type} \"${var.server_flags}\" ${var.install_mode} ${var.username} ${var.password} \"${var.install_method}\"",
    ]
  }
  provisioner "local-exec" {
    command = "echo ${aws_instance.master.public_ip} >/tmp/${var.resource_name}_master_ip"
  }
  provisioner "local-exec" {
    command = "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ${var.access_key} ${var.aws_user}@${aws_instance.master.public_ip}:/tmp/nodetoken /tmp/${var.resource_name}_nodetoken"
  }
  provisioner "local-exec" {
    command = "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ${var.access_key} ${var.aws_user}@${aws_instance.master.public_ip}:/tmp/config /tmp/${var.resource_name}_config"
  }
  provisioner "local-exec" {
    command = "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ${var.access_key} ${var.aws_user}@${aws_instance.master.public_ip}:/tmp/joinflags /tmp/${var.resource_name}_joinflags"
  }
  provisioner "local-exec" {
    command = "sed s/127.0.0.1/\"${var.create_lb ? aws_route53_record.aws_route53[0].fqdn : aws_instance.master.public_ip}\"/g /tmp/${var.resource_name}_config >/tmp/${var.resource_name}_kubeconfig"
  }
}

resource "aws_instance" "master2" {
  ami           =  var.aws_ami
  instance_type =  var.ec2_instance_class
  iam_instance_profile = "${var.iam_role}"
  count         = var.no_of_server_nodes
  connection {
    type        = "ssh"
    user        = "${var.aws_user}"
    host        = self.public_ip
    private_key = "${file(var.access_key)}"
  }
  root_block_device {
    volume_size = var.volume_size
    volume_type = "standard"
  }
  subnet_id = var.subnets
  availability_zone = var.availability_zone
  vpc_security_group_ids = ["${var.sg_id}"]
  key_name = "jenkins-rke-validation"
  tags = {
    Name = "${var.resource_name}-servers"
    "kubernetes.io/cluster/clusterid" = "owned"
  }
  depends_on       = ["aws_instance.master"]
  provisioner "file" {
    source      = "define_node_role.sh"
    destination = "/tmp/define_node_role.sh"
  }
  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/define_node_role.sh",
      "sudo /tmp/define_node_role.sh ${count.index} \"${var.role_order}\" ${var.all_role_nodes} ${var.etcd_only_nodes} ${var.etcd_cp_nodes} ${var.etcd_worker_nodes} ${var.cp_only_nodes} ${var.cp_worker_nodes}",
    ]
  }
  provisioner "file" {
    source      = "join_rke2_master.sh"
    destination = "/tmp/join_rke2_master.sh"
  }
  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/join_rke2_master.sh",
      "sudo /tmp/join_rke2_master.sh ${var.node_os} ${var.create_lb ? aws_route53_record.aws_route53[0].fqdn : aws_instance.master.public_ip} ${aws_instance.master.public_ip} ${local.node_token} ${var.rke2_version} ${self.public_ip} ${var.rke2_channel} ${var.cluster_type} \"${var.server_flags}\" ${var.install_mode} ${var.username} ${var.password} \"${var.install_method}\"",
    ]
  }
}

data "local_file" "token" {
  filename = "/tmp/${var.resource_name}_nodetoken"
  depends_on = [aws_instance.master]
}

locals {
  node_token = trimspace("${data.local_file.token.content}")
}

resource "local_file" "master_ips" {
  content     = join("," , aws_instance.master.*.public_ip,aws_instance.master2.*.public_ip)
  filename = "/tmp/${var.resource_name}_master_ips"
}

resource "aws_lb_target_group" "aws_tg_6443" {
  port             = 6443
  protocol         = "TCP"
  vpc_id           = "${var.vpc_id}"
  name             = "${var.resource_name}-tg-6443"
  count            = var.create_lb ? 1 : 0
}

resource "aws_lb_target_group" "aws_tg_9345" {
  port             = 9345
  protocol         = "TCP"
  vpc_id           = "${var.vpc_id}"
  name             = "${var.resource_name}-tg-9345"
  count            = var.create_lb ? 1 : 0
}

resource "aws_lb_target_group" "aws_tg_80" {
  port             = 80
  protocol         = "TCP"
  vpc_id           = "${var.vpc_id}"
  name             = "${var.resource_name}-tg-80"
  health_check {
        protocol = "HTTP"
        port = "traffic-port"
        path = "/ping"
        interval = 10
        timeout = 6
        healthy_threshold = 3
        unhealthy_threshold = 3
        matcher = "200-399"
  }
  count            = var.create_lb ? 1 : 0
}

resource "aws_lb_target_group" "aws_tg_443" {
  port             = 443
  protocol         = "TCP"
  vpc_id           = "${var.vpc_id}"
  name             = "${var.resource_name}-tg-443"
  health_check {
        protocol = "HTTP"
        port = 80
        path = "/ping"
        interval = 10
        timeout = 6
        healthy_threshold = 3
        unhealthy_threshold = 3
        matcher = "200-399"
  }
  count            = var.create_lb ? 1 : 0
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_6443" {
  target_group_arn = "${aws_lb_target_group.aws_tg_6443[0].arn}"
  target_id        = "${aws_instance.master.id}"
  port             = 6443
  count            = var.create_lb ? 1 : 0
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_6443_2" {
  target_group_arn = "${aws_lb_target_group.aws_tg_6443[0].arn}"
  count            = var.create_lb ? length(aws_instance.master2) : 0
  target_id        = "${aws_instance.master2[count.index].id}"
  port             = 6443
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_9345" {
  target_group_arn = "${aws_lb_target_group.aws_tg_9345[0].arn}"
  target_id        = "${aws_instance.master.id}"
  port             = 9345
  count            = var.create_lb ? 1 : 0
}
resource "aws_lb_target_group_attachment" "aws_tg_attachment_9345_2" {
  target_group_arn = "${aws_lb_target_group.aws_tg_9345[0].arn}"
  count            = var.create_lb ? length(aws_instance.master2) : 0
  target_id        = "${aws_instance.master2[count.index].id}"
  port             = 9345
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_80" {
  target_group_arn = "${aws_lb_target_group.aws_tg_80[0].arn}"
  target_id        = "${aws_instance.master.id}"
  port             = 80
  depends_on       = [aws_instance.master]
  count            = var.create_lb ? 1 : 0
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_80_2" {
  target_group_arn = "${aws_lb_target_group.aws_tg_80[0].arn}"
  count            = var.create_lb ? length(aws_instance.master2) : 0
  target_id        = "${aws_instance.master2[count.index].id}"
  port             = 80
  depends_on       = [aws_instance.master]
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_443" {
  target_group_arn = "${aws_lb_target_group.aws_tg_443[0].arn}"
  target_id        = "${aws_instance.master.id}"
  port             = 443
  depends_on       = [aws_instance.master]
  count            = var.create_lb ? 1 : 0
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_443_2" {
  target_group_arn = "${aws_lb_target_group.aws_tg_443[0].arn}"
  count            = var.create_lb ? length(aws_instance.master2) : 0
  target_id        = "${aws_instance.master2[count.index].id}"
  port             = 443
  depends_on       = [aws_instance.master]
}

resource "aws_lb" "aws_nlb" {
  internal           = false
  load_balancer_type = "network"
  subnets            = ["${var.subnets}"]
  name               = "${var.resource_name}-nlb"
  count              = var.create_lb ? 1 : 0
}

resource "aws_lb_listener" "aws_nlb_listener_6443" {
  load_balancer_arn = "${aws_lb.aws_nlb[0].arn}"
  port              = "6443"
  protocol          = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.aws_tg_6443[0].arn}"
  }
  count             = var.create_lb ? 1 : 0
}

resource "aws_lb_listener" "aws_nlb_listener_9345" {
  load_balancer_arn = "${aws_lb.aws_nlb[0].arn}"
  port              = "9345"
  protocol          = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.aws_tg_9345[0].arn}"
  }
  count             = var.create_lb ? 1 : 0
}

resource "aws_lb_listener" "aws_nlb_listener_80" {
  load_balancer_arn = "${aws_lb.aws_nlb[0].arn}"
  port              = "80"
  protocol          = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.aws_tg_80[0].arn}"
  }
  count             = var.create_lb ? 1 : 0
}

resource "aws_lb_listener" "aws_nlb_listener_443" {
  load_balancer_arn = "${aws_lb.aws_nlb[0].arn}"
  port              = "443"
  protocol          = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.aws_tg_443[0].arn}"
  }
  count             = var.create_lb ? 1 : 0
}

resource "aws_route53_record" "aws_route53" {
  zone_id            = "${data.aws_route53_zone.selected.zone_id}"
  name               = "${var.resource_name}"
  type               = "CNAME"
  ttl                = "300"
  records            = ["${aws_lb.aws_nlb[0].dns_name}"]
  count              = var.create_lb ? 1 : 0
}

data "aws_route53_zone" "selected" {
  name               = "${var.qa_space}"
  private_zone       = false
}

resource "null_resource" "update_kubeconfig" {
  provisioner "local-exec" {
    command = "sed s/127.0.0.1/\"${var.create_lb ? aws_route53_record.aws_route53[0].fqdn : aws_instance.master.public_ip}\"/g /tmp/\"${var.resource_name}_config\" >/tmp/${var.resource_name}_kubeconfig"
  }
  depends_on = [aws_instance.master]
}

resource "null_resource" "store_fqdn" {
  provisioner "local-exec" {
    command = "echo \"${var.create_lb ? aws_route53_record.aws_route53[0].fqdn : aws_instance.master.public_ip}\" >/tmp/${var.resource_name}_fixed_reg_addr"
  }
  depends_on = [aws_instance.master]
}