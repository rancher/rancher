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
  default = "rancheros-${env("OS_VERSION")}-docker-${env("DOCKER_VERSION")}"
}

locals { timestamp = regex_replace(timestamp(), "[- TZ:]", "") }

source "amazon-ebs" "rancheros" {
  ami_name      = "${var.ami_name}"
  instance_type = "t3.medium"
  ami_regions   = ["us-east-1", "us-east-2", "us-west-1", "us-west-2"]
  force_deregister = true
  force_delete_snapshot = true
  source_ami_filter {
    filters = {
      name                = "*rancheros-v${var.os_version}-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["605812595337"]
  }
  launch_block_device_mappings {
    device_name = "/dev/sda1"
    volume_size = 35
    delete_on_termination = true
  }
  ssh_username = "rancher"
  run_tags = {
    "Name" = "rancheros-${var.os_version}-packer-${local.timestamp}"
  }
}


build {
  sources = ["source.amazon-ebs.rancheros"]

  provisioner "shell" {
    inline = [
      "sudo ros engine switch docker-${var.docker_version}",
      "sudo ros engine enable docker-${var.docker_version}"
    ]
  }
}
