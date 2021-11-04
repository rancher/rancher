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
  default = "centos-${env("OS_VERSION")}-docker-${env("DOCKER_VERSION")}"
}

locals { timestamp = regex_replace(timestamp(), "[- TZ:]", "") }

source "amazon-ebs" "centos" {
  ami_name      = "${var.ami_name}"
  instance_type = "t3.medium"
  ami_regions   = ["us-east-1", "us-east-2", "us-west-1", "us-west-2"]
  force_deregister = true
  force_delete_snapshot = true
  source_ami_filter {
    filters = {
      name                = "*CentOS*${var.os_version}.*x86_64*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
      architecture        = "x86_64"
      is-public           = true
    }
    most_recent = true
    # CentOS AMI Owner
    owners      = ["125523088429"]
  }
  launch_block_device_mappings {
    device_name = "/dev/sda1"
    volume_size = 35
    delete_on_termination = true
  }
  ssh_username = "centos"
  run_tags = {
    "Name" = "centos-${var.os_version}-packer-${local.timestamp}"
  }
}


build {
  sources = ["source.amazon-ebs.centos"]

  provisioner "shell" {
    inline = [
      "curl https://releases.rancher.com/install-docker/${var.docker_version}.sh | sh",
      "sudo usermod -aG docker $USER",
      "sudo systemctl enable docker && sudo systemctl start docker",
      "sudo docker info"
    ]
  }
}
