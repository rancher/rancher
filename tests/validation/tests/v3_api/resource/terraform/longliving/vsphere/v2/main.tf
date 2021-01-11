terraform {
    required_providers {
        rancher2 = {
            source  = "rancher/rancher2"
            version = "1.10.6"
        }
    }
}

provider "vsphere" {
  user           = var.vsphere_user
  password       = var.vsphere_password
  vsphere_server = var.vsphere_server

  # If you have a self-signed cert
  allow_unverified_ssl = true
}

provider "rancher2" {
    api_url    = var.cattle_url
    token_key  = var.cattle_token
    insecure   = true
}

locals {
    node_config = ["controlplane", "etcd", "worker", "worker", "worker"]
}

// create custom cluster in Rancher
resource "rancher2_cluster" "custom-cluster" {
    name = var.cluster_name
    description = "Created from Terraform"
    rke_config {
        addon_job_timeout = 45
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
      cloud_provider {
        name = "vsphere"
        vsphere_cloud_provider {
          global {
            insecure_flag = true
            soap_roundtrip_count = 0
          }
          virtual_center {
            name = var.vsphere_server
            user = var.vsphere_user
            password = var.vsphere_password
            port = 443
            datacenters = var.vsphere_datacenter
          }
          workspace {
            server = var.vsphere_server
            folder = var.vsphere_folder
            default_datastore = var.vsphere_datastore
            datacenter = var.vsphere_datacenter
            resourcepool_path = var.vsphere_resource_pool
          }
          disk {
            scsi_controller_type = "pvscsi"
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
  chart_version = var.monitoring_version
}

resource "rancher2_app_v2" "istio" {
  cluster_id = rancher2_cluster_sync.custom-cluster.cluster_id
  name = "rancher-istio"
  namespace = "istio-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-istio"
  chart_version = var.istio_version
}

resource "rancher2_app_v2" "logging" {
  cluster_id = rancher2_cluster_sync.custom-cluster.cluster_id
  name = "rancher-logging"
  namespace = "cattle-logging-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-logging"
  chart_version = var.logging_version
}

resource "vsphere_virtual_machine" "from_template" {
  count                      = length(local.node_config)
  name                       = "${var.node_name}-${count.index}"
  num_cpus                   = 2
  memory                     = 4096
  resource_pool_id           = data.vsphere_resource_pool.pool.id
  datastore_id               = data.vsphere_datastore.datastore.id
  folder                     = var.vsphere_folder
  wait_for_guest_net_timeout = 5
  guest_id                   = data.vsphere_virtual_machine.template.guest_id
  enable_disk_uuid           = true

  cdrom {
    client_device = true
  }

  vapp {
    properties = {
      "user-data" = base64encode(file("${path.module}/templates/userdata.tmpl"))
      "hostname" = "${var.node_name}-${count.index}"
    }
  }

  disk {
    label            = "disk0"
    size             = data.vsphere_virtual_machine.template.disks.0.size
    eagerly_scrub    = data.vsphere_virtual_machine.template.disks.0.eagerly_scrub
    thin_provisioned = data.vsphere_virtual_machine.template.disks.0.thin_provisioned
  }

  network_interface {
    network_id   = data.vsphere_network.network.id
    adapter_type = data.vsphere_virtual_machine.template.network_interface_types[0]
  }

  clone {
    template_uuid = data.vsphere_virtual_machine.template.id
  }

  depends_on = [rancher2_cluster.custom-cluster]

  connection {
        type        = "ssh"
        user        = "rancher"
        host        = self.default_ip_address
        private_key = file(var.ssh_key_path)
    }

    provisioner "remote-exec" {
        inline = [
            "curl https://releases.rancher.com/install-docker/19.03.sh | sh",
            "${rancher2_cluster.custom-cluster.cluster_registration_token[0].node_command}  --${local.node_config[count.index]}"
        ]
    }
}