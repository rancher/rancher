variable "os_version" {
  type = string
  default = "${env("OS_VERSION")}"
}

variable "docker_version" {
  type = string
  default = "${env("DOCKER_VERSION")}"
}

variable "ami_name" {
  type = string
  default = "oraclelinux-${env("OS_VERSION")}-docker-${env("DOCKER_VERSION")}-selinux-on-rpm"
}

locals { timestamp = regex_replace(timestamp(), "[- TZ:]", "") }

source "amazon-ebs" "oraclelinux" {
  ami_name      = "${var.ami_name}"
  instance_type = "t3.medium"
  ami_regions   = ["us-east-1", "us-east-2", "us-west-1", "us-west-2"]
  force_deregister = true
  force_delete_snapshot = true
  source_ami_filter {
    filters = {
      name                = "*OL${var.os_version}-*x86_64*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
      architecture        = "x86_64"
      is-public           = true
    }
    most_recent = true
    # oraclelinux AMI Owner
    owners      = ["131827586825"]
  }
  launch_block_device_mappings {
    device_name = "/dev/sda1"
    volume_size = 35
    delete_on_termination = true
  }
  ssh_username = "ec2-user"
  run_tags = {
    "Name" = "oraclelinux-${var.os_version}-packer-${local.timestamp}"
  }
}


build {
  sources = ["source.amazon-ebs.oraclelinux"]

  provisioner "shell" {
    inline       = [
      "sudo systemctl stop firewalld",
      "sudo systemctl disable firewalld",
      "sudo systemctl stop iptables",
      "sudo systemctl disable iptables",
      "sudo sed -i \"s/SELINUX=permissive/SELINUX=enforcing/\" /etc/selinux/config",
      "curl https://releases.rancher.com/install-docker/${var.docker_version}.sh | sh",
      "sudo usermod -aG docker $USER",
      "sudo systemctl enable docker",
      "echo '{\"selinux-enabled\": true}' | sudo tee /etc/docker/daemon.json",
      "sudo service docker start",
      "sudo getenforce",
      "sudo systemctl status docker",
      "sudo docker info",
      "sudo bash -c \"cat << EOF > /etc/yum.repos.d/rancher-testing.repo\n[rancher-testing]\nname=Rancher Testing\nbaseurl=https://rpm-testing.rancher.io/rancher/testing/centos/8/noarch\nenabled=1\ngpgcheck=1\ngpgkey=https://rpm-testing.rancher.io/public.key\nEOF\"",
      "sudo yum -y install rancher-selinux",
      "sudo reboot"
    ]
    expect_disconnect = true
  }
  provisioner "shell" {
    inline = [
      "selstatus=$(sudo getenforce) && if (($selstatus != \"Enforcing\")); then exit 1; fi",
      "echo 'SELINUX ENABLED'"
    ]
    pause_before = "60s"
    max_retries  = 2
  }
}
