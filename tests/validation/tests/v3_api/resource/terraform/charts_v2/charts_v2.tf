terraform {
    required_providers {
        rancher2 = {
            source  = "rancher/rancher2"
            version = "1.10.6"
        }
    }
}

provider "rancher2" {
    api_url    = var.api_url
    token_key  = var.token_key
    insecure   = true
}


resource "rancher2_app_v2" "monitoring-crd" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-monitoring-crd"
  namespace = "cattle-monitoring-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-monitoring-crd"
  chart_version = var.monitoring_version
}

resource "rancher2_app_v2" "monitoring" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-monitoring"
  namespace = "cattle-monitoring-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-monitoring"
  chart_version = var.monitoring_version
  values = templatefile("${var.values_path}/charts_values/values_monitoring.yaml", {})

  depends_on = [rancher2_app_v2.monitoring-crd]
}

resource "rancher2_app_v2" "rancher-kiali-server-crd" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-kiali-server-crd"
  namespace = "istio-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-kiali-server-crd"
  chart_version = var.kiali_version
   depends_on = [rancher2_app_v2.monitoring]
}

resource "rancher2_app_v2" "istio" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-istio"
  namespace = "istio-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-istio"
  chart_version = var.istio_version
  values = templatefile("${var.values_path}/charts_values/values_istio.yaml", {})

  depends_on = [rancher2_app_v2.monitoring,rancher2_app_v2.rancher-kiali-server-crd]
}

resource "rancher2_app_v2" "logging-crd" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-logging-crd"
  namespace = "cattle-logging-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-logging-crd"
  chart_version = var.logging_version
}

resource "rancher2_app_v2" "logging" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-logging"
  namespace = "cattle-logging-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-logging"
  chart_version = var.logging_version
  values = templatefile("${var.values_path}/charts_values/values_logging.yaml", {cluster_provider = var.cluster_provider})

  depends_on = [rancher2_app_v2.logging-crd]
}

resource "rancher2_app_v2" "cis-benchmark-crd" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-cis-benchmark-crd"
  namespace = "cis-operator-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-cis-benchmark-crd"
  chart_version = var.cis_version
}

resource "rancher2_app_v2" "cis-benchmark" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-cis-benchmark"
  namespace = "cis-operator-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-cis-benchmark"
  chart_version = var.cis_version
  values = templatefile("${var.values_path}/charts_values/values_cis_benchmark.yaml", {})

  depends_on = [rancher2_app_v2.cis-benchmark-crd]
}
