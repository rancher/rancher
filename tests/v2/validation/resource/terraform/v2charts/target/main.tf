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

resource "rancher2_app_v2" "monitoring" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-monitoring"
  namespace = "cattle-monitoring-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-monitoring"
  chart_version = var.monitoring_version
  wait = true
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
  wait = true
}

resource "rancher2_app_v2" "istio" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-istio"
  namespace = "istio-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-istio"
  chart_version = var.istio_version

  depends_on = [rancher2_app_v2.monitoring,rancher2_app_v2.rancher-kiali-server-crd]
  wait = true
}

resource "rancher2_app_v2" "logging" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-logging"
  namespace = "cattle-logging-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-logging"
  chart_version = var.logging_version
  wait = true
}

resource "rancher2_app_v2" "cis-benchmark" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-cis-benchmark"
  namespace = "cis-operator-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-cis-benchmark"
  chart_version = var.cis_version
  wait = true
}

resource "rancher2_app_v2" "rancher-gatekeeper" {  
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-gatekeeper"
  namespace = "cattle-gatekeeper-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-gatekeeper"
  chart_version = var.gatekeeper_version
  depends_on = [rancher2_app_v2.monitoring]
  wait = true
}

resource "rancher2_app_v2" "rancher-backup" {  
  cluster_id = "local"
  project_id = ""
  name = "rancher-backup"
  namespace = "cattle-resources-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-backup"
  chart_version = var.backup_version
  values = <<EOF
persistence:
  enabled: false
  size: 2Gi
  storageClass: '-'
  volumeName: ""
s3:
  bucketName: ""
  credentialSecretName: ""
  credentialSecretNamespace: ""
  enabled: false
  endpoint: ""
  endpointCA: ""
  folder: ""
  insecureTLSSkipVerify: false
  region: ""
EOF
}

resource "rancher2_app_v2" "longhorn" {  
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-longhorn"
  namespace = "longhorn-system"
  repo_name = "rancher-charts"
  chart_name = "longhorn"
  chart_version = var.longhorn_version
  //wait = true
}
