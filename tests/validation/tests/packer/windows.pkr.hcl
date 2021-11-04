variable "os_version" {
  type = string
  default = "${env("OS_VERSION")}"
}

variable "ami_name" {
  type = string
  default = "Windows-${env("OS_VERSION")}-docker-${env("DOCKER_VERSION")}-packer"
}

variable "docker_version" {
  type = string
  default = "${env("DOCKER_VERSION")}"
}

variable "winrm_username" {
  type = string
  default = "Administrator"
}

variable "aws_ssh_key" {
  type = string
  description = "This is the public ssh key: ssh-keygen -y -f aws.pem > your_pub_key.pub"
  default = "${env("AWS_SSH_KEY")}"
}

locals { timestamp = regex_replace(timestamp(), "[- TZ:]", "") }

source "amazon-ebs" "windows" {
  ami_name          = "${var.ami_name}"
  instance_type     = "t3.xlarge"
  ami_regions       = ["us-east-1", "us-east-2", "us-west-1", "us-west-2"]
  force_deregister = true
  force_delete_snapshot = true
  source_ami_filter {
    filters = {
      name                = "*Windows*Server-${var.os_version}*ContainersLatest*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
      architecture        = "x86_64"
      is-public           = true
    }
    most_recent = true
    # Windows AMI Owner
    owners      = ["amazon"]
  }
  launch_block_device_mappings {
    device_name           = "/dev/sda1"
    volume_size           = 50
    delete_on_termination = true
  }
  user_data_file = "./scripts/user_data.ps1"
  communicator   = "winrm"
  winrm_username = "${var.winrm_username}"
  winrm_port     = 5986
  winrm_timeout  = "5m"
  winrm_use_ssl  = true
  winrm_insecure = true
  run_tags = {
    "Name" = "Windows-${var.os_version}-packer-${local.timestamp}"
  }
}


build {
  sources = ["source.amazon-ebs.windows"]

  provisioner "powershell" {
    inline = [
      "Set-Content -Path C:\\ProgramData\\ssh\\administrators_authorized_keys -Value '${var.aws_ssh_key}'",
      "$acl = Get-Acl C:\\ProgramData\\ssh\\administrators_authorized_keys",
      "$acl.SetAccessRuleProtection($true, $false)",
      "$administratorsRule = New-Object system.security.accesscontrol.filesystemaccessrule(\"Administrators\",\"FullControl\",\"Allow\")",
      "$systemRule = New-Object system.security.accesscontrol.filesystemaccessrule(\"SYSTEM\",\"FullControl\",\"Allow\")",
      "$acl.SetAccessRule($administratorsRule)",
      "$acl.SetAccessRule($systemRule)",
      "$acl | Set-Acl",
      "Get-WindowsCapability -Online | ? Name -like 'OpenSSH*'",
      "Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0",
      "Start-Service sshd",
      "Set-Service -Name sshd -StartupType 'Automatic'",      
      "$launchConfig = Get-Content -Path C:\\ProgramData\\Amazon\\EC2-Windows\\Launch\\Config\\LaunchConfig.json | ConvertFrom-Json",
      "$launchConfig.adminPasswordType = 'Random'",
      "$launchConfig",
      "Set-Content -Value ($launchConfig | ConvertTo-Json) -Path C:\\ProgramData\\Amazon\\EC2-Windows\\Launch\\Config\\LaunchConfig.json",
      "iwr -outf docker-install.ps1 https://gist.githubusercontent.com/thxCode/cd8ec26795a56eb120b57675f0c067cf/raw/34b05ce095d99e7539d330760c2d880179cc971a/docker-install.ps1; ./docker-install.ps1 -Version ${var.docker_version}",
    ]
    elevated_user = "SYSTEM"
    elevated_password = ""
  }
  provisioner "powershell" {
    inline = [
      "C:\\ProgramData\\Amazon\\EC2-Windows\\Launch\\Scripts\\InitializeInstance.ps1 -Schedule",
      "C:\\ProgramData\\Amazon\\EC2-Windows\\Launch\\Scripts\\SysprepInstance.ps1"
    ]
    pause_before = "60s"
    max_retries  = 2  
  }
}
