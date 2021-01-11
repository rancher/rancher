terraform {
    required_providers {
        rancher2 = {
            source  = "rancher/rancher2"
            version = "1.10.6"
        }
    }
}

provider "aws" {
    region  = "us-east-2"
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
    name = "custom-cluster"
    description = "custom-cluster from terraform"
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
        upgrade_strategy {
            drain = false
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
        "kubernetes.io/cluster/long-living" = "shared"
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