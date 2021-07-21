terraform {
    required_providers {
        rancher2 = {
            source  = "rancher/rancher2"
            version = "1.10.6"
        }
    }
}

provider "aws" {
    region  = "us-west-2"
    profile = "rancher-eng"
}

provider "rancher2" {
    api_url    = var.cattle_test_url
    token_key  = var.admin_token
    insecure   = true
}

locals {
    node_config = ["controlplane", "etcd", "worker", "worker", "worker"]
}

// create custom cluster in Rancher
resource "rancher2_cluster" "custom-cluster" {
    name = "${var.node_name}"
    description = "Created from Terraform"
    rke_config {
        addon_job_timeout = 45
        cloud_provider {
            name = "aws"
            aws_cloud_provider {
            }
        }
        network {
            plugin = "canal"
        }
        services {
            etcd {
                backup_config {
                    enabled = true
                    retention = 3
                    interval_hours = 6
                    s3_backup_config {
                        bucket_name = var.minio_bucket
                        endpoint = var.minio_endpoint
                        access_key = var.minio_access_key
                        secret_key = var.minio_secret_key
                        custom_ca = var.minio_ca
                    }
                }
            }
            kube_api {
                secrets_encryption_config {
                    enabled = true
                }
            }
        }      
    }
}

// Create a new rancher2 Cluster Sync for custom-cluster cluster
resource "rancher2_cluster_sync" "custom-cluster" {
  cluster_id        = rancher2_cluster.custom-cluster.id
  state_confirm     = 3
}

resource "rancher2_app_v2" "monitoring" {
  cluster_id = rancher2_cluster_sync.custom-cluster.cluster_id
  name = "rancher-monitoring"
  namespace = "cattle-monitoring-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-monitoring"
  chart_version = "9.4.200"
}

resource "rancher2_app_v2" "istio" {
  cluster_id = rancher2_cluster_sync.custom-cluster.cluster_id
  name = "rancher-istio"
  namespace = "istio-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-istio"
  chart_version = "1.7.300"
}

resource "rancher2_app_v2" "logging" {
  cluster_id = rancher2_cluster_sync.custom-cluster.cluster_id
  name = "rancher-logging"
  namespace = "cattle-logging-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-logging"
  chart_version = "3.6.001"
}

// create aws resource & run docker run
resource "aws_instance" "custom_nodes" {
    count                   = length(local.node_config)
    ebs_optimized           = true
    instance_type           = "t3.xlarge"
    ami                     = var.ami
    subnet_id               = var.subnet
    vpc_security_group_ids  = var.security_groups
    key_name                = var.ssh_key_name
    iam_instance_profile    = "RancherK8SUnrestrictedCloudProviderRoleUS"

    connection {
        type        = "ssh"
        user        = var.ami_user
        host        = self.public_ip
        private_key = file(var.path_to_key)
    }

    user_data = templatefile("${path.module}/files/node_userdata.tmpl",
    {
        ssh_keys = var.ssh_keys
    })

    tags = {
        Name = "${var.node_name}-${count.index}"
        Owner = "rancher-qa"
        DoNotDelete = "true"
        "kubernetes.io/cluster/long-living" = "owned"
    }

    root_block_device {
        volume_size = "32"
    }
  
    depends_on = [rancher2_cluster.custom-cluster]

    provisioner "remote-exec" {
        inline = [
            "${rancher2_cluster.custom-cluster.cluster_registration_token[0].node_command}  --${local.node_config[count.index]}"
        ]
    }
}