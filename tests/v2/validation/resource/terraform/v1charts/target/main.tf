#Target cluster for v1 charts installation
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

resource "rancher2_cluster_sync" "v1target-sync" {
  cluster_id =  var.cluster_id
}

resource "local_file" "sync_kube_config" {
  content  = rancher2_cluster_sync.v1target-sync.kube_config
  filename = "kube_config_target_cluster.yaml"

  depends_on = [
    rancher2_cluster_sync.v1target-sync
  ]
}


resource "rancher2_namespace" "longhorn-system-namespace" {
  name = "longhorn-system"
  project_id = var.project_id
  description = "longhorn-system namespace"

  depends_on = [
    rancher2_cluster_sync.v1target-sync,
    local_file.sync_kube_config
  ]
}


resource "null_resource" "longhorn-iscsi-nfs" {
  provisioner "local-exec" {
    environment = {
      KUBECONFIG = "kube_config_target_cluster.yaml"
    }
    command = "kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/${var.rancher_longhorn_prereq_version}/deploy/prerequisite/longhorn-iscsi-installation.yaml --namespace=longhorn-system && kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/${var.rancher_longhorn_prereq_version}/deploy/prerequisite/longhorn-nfs-installation.yaml --namespace=longhorn-system"
  }

  depends_on = [
    rancher2_cluster_sync.v1target-sync,
    local_file.sync_kube_config,
    rancher2_namespace.longhorn-system-namespace
  ]
}


resource "rancher2_app" "longhorn-system" {
  catalog_name = "library"
  name = "longhorn"
  description = "Terraform app acceptance test"
  project_id = var.project_id
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

resource "null_resource" "longhorn-statefulset-example" {
  provisioner "local-exec" {
    environment = {
      KUBECONFIG = "kube_config_target_cluster.yaml"
    }
    command = "kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/master/examples/statefulset.yaml"
  }

  depends_on = [
    rancher2_app.longhorn-system
  ]
}

resource "rancher2_cluster_logging" "cluster-logging" {
  name = "cluster-logging"
  cluster_id = var.cluster_id
  kind = "syslog"
  syslog_config {
    endpoint = var.rancher_logging_endpoint
    protocol = "udp"
    severity = "notice"
    ssl_verify = false
  }
}

resource "rancher2_namespace" "istio-namespace" {
  name = "istio-system"
  project_id = var.project_id
  description = "istio namespace"
}


resource "rancher2_app" "istio" {
  catalog_name = "system-library"
  name = "cluster-istio"
  description = "Terraform app acceptance test"
  project_id = var.project_id
  template_name = "rancher-istio"
  template_version = var.rancher_istio_version
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
    "global.rancher.clusterId" = var.cluster_id
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