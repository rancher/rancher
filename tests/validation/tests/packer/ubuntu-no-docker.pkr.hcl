variable "os_version" {
  type = string
  default = "${env("OS_VERSION")}"
}

variable "ami_name" {
  type = string
  default = "ubuntu-${env("OS_VERSION")}-no-docker"
}

locals { timestamp = regex_replace(timestamp(), "[- TZ:]", "") }

source "amazon-ebs" "ubuntu" {
  ami_name      = "${var.ami_name}"
  instance_type = "t3.medium"
  ami_regions   = ["us-east-1", "us-east-2", "us-west-1", "us-west-2"]
  force_deregister = true
  force_delete_snapshot = true
  source_ami_filter {
    filters = {
      name                = "ubuntu/images/*/ubuntu-*-${var.os_version}-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
      architecture        = "x86_64"
    }
    most_recent = true
    owners      = ["aws-marketplace"]
  }
  launch_block_device_mappings {
    device_name = "/dev/sda1"
    volume_size = 35
    delete_on_termination = true
  }
  ssh_username = "ubuntu"
  run_tags = {
    "Name" = "ubuntu-${var.os_version}-packer-${local.timestamp}"
  }
}


build {
  sources = ["source.amazon-ebs.ubuntu"]

  provisioner "shell" {
    inline = [
      "sudo ufw disable",
      "sudo apt-get remove docker docker-engine docker.io containerd runc"
    ]
  }
}