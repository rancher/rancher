#Cluster creation with v1 charts installed
terraform {
  required_version = ">= 0.14"
  required_providers {
    rancher2 = {
      source = "rancher/rancher2"
    }
  }
}

provider "rancher2" {
  api_url   = var.rancher_api_url
  token_key = var.rancher_token_key
}

resource "rancher2_cluster" "v1charts" {
  name = var.cluster_name
  description = "Rancher v1 Charts Cluster"
  rke_config {
    kubernetes_version = var.k8s_version
    network {
      plugin = "canal"
    }
  }
  enable_cluster_alerting = true  
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
  }
}

resource "rancher2_node_template" "node-template" {
  name = "tf-aws-node-template"
  description = "TF AWS node template"

  amazonec2_config {
    access_key = var.aws_access_key
    secret_key = var.aws_secret_key
    ami =  var.ami
    region = var.aws_region
    security_group = var.security_groups
    subnet_id = var.subnet
    vpc_id = var.vpc_id
    zone = var.zone
    root_size = var.root_size
    instance_type = var.instance_type
  }
}

resource "rancher2_node_pool" "cp-node-pool" {
  cluster_id =  rancher2_cluster.v1charts.id
  name = "tf-cp-node-pool"
  hostname_prefix = var.hostname_prefix_cp
  node_template_id = rancher2_node_template.node-template.id
  quantity = 1
  control_plane = true
  etcd = false
  worker = false
}

resource "rancher2_node_pool" "etcd-node-pool" {
  cluster_id =  rancher2_cluster.v1charts.id
  name = "tf-etcd-node-pool"
  hostname_prefix = var.hostname_prefix_etcd
  node_template_id = rancher2_node_template.node-template.id
  quantity = 1
  control_plane = false
  etcd = true
  worker = false
}

resource "rancher2_node_pool" "worker-node-pool" {
  cluster_id =  rancher2_cluster.v1charts.id
  name = "tf-worker-node-pool"
  hostname_prefix = var.hostname_prefix_worker
  node_template_id = rancher2_node_template.node-template.id
  quantity = var.worker_count
  control_plane = false
  etcd = false
  worker = true
}

resource "rancher2_cluster_sync" "v1charts-sync" {
  cluster_id = rancher2_cluster.v1charts.id
}

resource "rancher2_namespace" "istio-namespace" {
  name = "istio-system"
  project_id = rancher2_cluster_sync.v1charts-sync.system_project_id
  description = "istio namespace"
}

resource "rancher2_app" "istio" {
  catalog_name = "system-library"
  name = "cluster-istio"
  description = "Terraform app acceptance test"
  project_id = rancher2_cluster_sync.v1charts-sync.system_project_id
  template_name = "rancher-istio"
  template_version = "1.5.901"
  target_namespace = rancher2_namespace.istio-namespace.id
  answers = {
    "certmanager.enabled" = false
    "enableCRDs" = true
    "galley.enabled" = true
    "gateways.enabled" = false
    "gateways.istio-ingressgateway.resources.limits.cpu" = "2000m"
    "gateways.istio-ingressgateway.resources.limits.memory" = "1024Mi"
    "gateways.istio-ingressgateway.resources.requests.cpu" = "100m"
    "gateways.istio-ingressgateway.resources.requests.memory" = "128Mi"
    "gateways.istio-ingressgateway.type" = "NodePort"
    "global.monitoring.type" = "cluster-monitoring"
    "global.rancher.clusterId" = rancher2_cluster_sync.v1charts-sync.cluster_id
    "istio_cni.enabled" = false
    "istiocoredns.enabled" = false
    "kiali.enabled" = true
    "mixer.enabled" = true
    "mixer.policy.enabled" = true
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

resource "rancher2_cluster_logging" "cluster-logging" {
  name = "cluster-logging"
  cluster_id = rancher2_cluster.v1charts.id
  kind = "syslog"
  syslog_config {
    endpoint = var.logging_endpoint
    protocol = "udp"
    severity = "notice"
    ssl_verify = false
  }
}

resource "rancher2_namespace" "longhorn-system-namespace" {
  name = "longhorn-system"
  project_id = rancher2_cluster_sync.v1charts-sync.system_project_id
  description = "longhorn-system namespace"
}

resource "rancher2_app" "longhorn-system" {
  catalog_name = "library"
  name = "longhorn"
  description = "Terraform app acceptance test"
  project_id = rancher2_cluster_sync.v1charts-sync.system_project_id
  template_name = "longhorn"
  target_namespace = rancher2_namespace.longhorn-system-namespace.id
  answers = {
    "image.defaultImage" = true
    "privateRegistry.registryUrl" = ""
    "privateRegistry.registryUser" = ""
    "privateRegistry.registryPasswd" = ""
    "privateRegistry.registrySecret" = ""
    "longhorn.default_setting" = false
    "persistence.defaultClass" = true
    "persistence.reclaimPolicy" = "Delete"
    "persistence.defaultClassReplicaCount" = "3"
    "persistence.recurringJobSelector.enable" = false
    "persistence.backingImage.enable" = false
    "ingress.enabled" = false
    "service.ui.type" = "Rancher-Proxy"
    "enablePSP" = true
  }
}
