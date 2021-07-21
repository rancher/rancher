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
          }
        }      
    }
    enable_cluster_monitoring = true
    cluster_monitoring_input {
        answers = {
        "exporter-kubelets.https" = true
        "exporter-node.enabled" = true
        "exporter-node.ports.metrics.port" = 9796
        "exporter-node.resources.limits.cpu" = "200m"
        "exporter-node.resources.limits.memory" = "200Mi"
        "grafana.persistence.enabled" = false
        "grafana.persistence.size" = "10Gi"
        "grafana.persistence.storageClass" = "default"
        "operator.resources.limits.memory" = "500Mi"
        "prometheus.persistence.enabled" = "false"
        "prometheus.persistence.size" = "50Gi"
        "prometheus.persistence.storageClass" = "default"
        "prometheus.persistent.useReleaseName" = "true"
        "prometheus.resources.core.limits.cpu" = "1000m",
        "prometheus.resources.core.limits.memory" = "1500Mi"
        "prometheus.resources.core.requests.cpu" = "750m"
        "prometheus.resources.core.requests.memory" = "750Mi"
        "prometheus.retention" = "12h"
        }
        version = "0.1.4"
    }
}

// Create a new rancher2 Cluster Sync for custom-cluster cluster
resource "rancher2_cluster_sync" "custom-cluster" {
  cluster_id        = rancher2_cluster.custom-cluster.id
  wait_monitoring   = rancher2_cluster.custom-cluster.enable_cluster_monitoring
  state_confirm     = 3
}

}

// Create a new rancher2 Namespace for istio
resource "rancher2_namespace" "istio" {
  name          = "istio-system"
  project_id    = rancher2_cluster_sync.custom-cluster.system_project_id
  description   = "istio namespace"
}

// Create a new rancher2 App deploying istio (should wait until monitoring is up and running)
resource "rancher2_app" "istio" {
  catalog_name      = "system-library"
  name              = "cluster-istio"
  description       = "Terraform app acceptance test"
  project_id        = rancher2_namespace.istio.project_id
  template_name     = "rancher-istio"
  template_version  = "1.5.900"
  target_namespace  = rancher2_namespace.istio.id
  answers = {
    "certmanager.enabled" = false
    "enableCRDs" = true
    "galley.enabled" = true
    "gateways.enabled" = true
    "gateways.istio-ingressgateway.resources.limits.cpu" = "2000m"
    "gateways.istio-ingressgateway.resources.limits.memory" = "1024Mi"
    "gateways.istio-ingressgateway.resources.requests.cpu" = "100m"
    "gateways.istio-ingressgateway.resources.requests.memory" = "128Mi"
    "gateways.istio-ingressgateway.type" = "NodePort"
    "gateways.istio-ingressgateway.ports[0].nodePort" = 31380
    "gateways.istio-ingressgateway.ports[0].port" = 80
    "gateways.istio-ingressgateway.ports[0].targetPort" = 80
    "gateways.istio-ingressgateway.ports[0].name" = "http2"
    "global.monitoring.type" = "cluster-monitoring"
    "global.rancher.clusterId" = rancher2_cluster_sync.custom-cluster.cluster_id
    "istio_cni.enabled" = "false"
    "istiocoredns.enabled" = "false"
    "kiali.enabled" = "true"
    "mixer.enabled" = "true"
    "mixer.policy.enabled" = "true"
    "mixer.policy.resources.limits.cpu" = "4800m"
    "mixer.policy.resources.limits.memory" = "4096Mi"
    "mixer.policy.resources.requests.cpu" = "1000m"
    "mixer.policy.resources.requests.memory" = "1024Mi"
    "mixer.telemetry.resources.limits.cpu" = "4800m",
    "mixer.telemetry.resources.limits.memory" = "4096Mi"
    "mixer.telemetry.resources.requests.cpu" = "1000m"
    "mixer.telemetry.resources.requests.memory" = "1024Mi"
    "mtls.enabled" = false
    "nodeagent.enabled" = false
    "pilot.enabled" = true
    "pilot.resources.limits.cpu" = "1000m"
    "pilot.resources.limits.memory" = "4096Mi"
    "pilot.resources.requests.cpu" = "500m"
    "pilot.resources.requests.memory" = "2048Mi"
    "pilot.traceSampling" = "1"
    "security.enabled" = true
    "sidecarInjectorWebhook.enabled" = true
    "tracing.enabled" = true
    "tracing.jaeger.resources.limits.cpu" = "500m"
    "tracing.jaeger.resources.limits.memory" = "1024Mi"
    "tracing.jaeger.resources.requests.cpu" = "100m"
    "tracing.jaeger.resources.requests.memory" = "100Mi"
  }
}

resource "rancher2_cluster_logging" "logging" {
  name = "cluster-logging"
  cluster_id = rancher2_cluster_sync.custom-cluster.id
  kind = "fluentd"
  fluentd_config {
    fluent_servers {
        endpoint = var.logging_endpoint
        weight = 100
    }
  }
}

resource "vsphere_virtual_machine" "from_template" {
  count                      = length(local.node_config)
  name                       = "${var.node_name}-${count.index}"
  num_cpus                   = 2
  memory                     = 4096
  resource_pool_id           = data.vsphere_resource_pool.pool.id
  datastore_id               = data.vsphere_datastore.datastore.id
  wait_for_guest_net_timeout = 5
  guest_id                   = data.vsphere_virtual_machine.template.guest_id

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