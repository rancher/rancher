variable "os_version" {
  type = string
  default = "${env("OS_VERSION")}"
}

variable "ami_name" {
  type = string
  default = "rhel-${env("OS_VERSION")}-no-selinux-no-docker"
}

locals { timestamp = regex_replace(timestamp(), "[- TZ:]", "") }

source "amazon-ebs" "rhel" {
  ami_name      = "${var.ami_name}"
  instance_type = "t3.medium"
  ami_regions   = ["us-east-1", "us-east-2", "us-west-1", "us-west-2"]
  force_deregister = true
  force_delete_snapshot = true
  source_ami_filter {
    filters = {
      name                = "*RHEL-${var.os_version}*x86_64*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
      architecture        = "x86_64"
      is-public           = true
    }
    most_recent = true
    # rhel AMI Owner
    owners      = ["309956199498"]
  }
  launch_block_device_mappings {
    device_name = "/dev/sda1"
    volume_size = 35
    delete_on_termination = true
  }
  ssh_username = "ec2-user"
  run_tags = {
    "Name" = "rhel-${var.os_version}-packer-${local.timestamp}"
  }
}


build {
  sources = ["source.amazon-ebs.rhel"]

  provisioner "shell" {
    inline = [
      "sudo yum remove docker docker-client docker-client-latest docker-common docker-latest docker-latest-logrotate docker-logrotatedocker-engine",
      "sudo sed -i \"s/SELINUX=enforcing/SELINUX=disabled/\" /etc/selinux/config",
      "sudo setenforce 0",
      "sudo reboot",
    ]
    expect_disconnect = true
  }
  provisioner "shell" {
    inline = [
      "selstatus=$(sudo getenforce) && if (($selstatus != \"Disabled\")); then exit 1; fi",
      "echo 'SELINUX DISABLED'"
    ]
    pause_before = "60s"
    max_retries  = 2
  }
}
