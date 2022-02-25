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

resource "rancher2_cluster" "v2charts" {
  name = var.cluster_name
  description = "Rancher v2 Charts Cluster"
  rke_config {
    kubernetes_version = var.k8s_version
    network {
      plugin = "canal"
    }
  }  
}

resource "rancher2_node_template" "node-template" {
  name = "tf-do-node-template"
  description = "TF DO node template"
  engine_install_url = "https://releases.rancher.com/install-docker/20.10.sh"
  
  digitalocean_config {
    access_token = var.do_access_token
    region = var.region
    size = var.size
    image = var.image
  }
}

resource "rancher2_node_pool" "cp-node-pool" {
  cluster_id =  rancher2_cluster.v2charts.id
  name = "tf-cp-node-pool"
  hostname_prefix = var.hostname_prefix_cp
  node_template_id = rancher2_node_template.node-template.id
  quantity = 1
  control_plane = true
  etcd = false
  worker = false
}

resource "rancher2_node_pool" "etcd-node-pool" {
  cluster_id =  rancher2_cluster.v2charts.id
  name = "tf-etcd-node-pool"
  hostname_prefix = var.hostname_prefix_etcd
  node_template_id = rancher2_node_template.node-template.id
  quantity = 1
  control_plane = false
  etcd = true
  worker = false
}

resource "rancher2_node_pool" "worker-node-pool" {
  cluster_id =  rancher2_cluster.v2charts.id
  name = "tf-worker-node-pool"
  hostname_prefix = var.hostname_prefix_worker
  node_template_id = rancher2_node_template.node-template.id
  quantity = var.worker_count
  control_plane = false
  etcd = false
  worker = true
}

resource "rancher2_cluster_sync" "v2charts-sync" {
  cluster_id = rancher2_cluster.v2charts.id
  wait_catalogs = true
  state_confirm = 30
}

resource "rancher2_app_v2" "monitoring" {
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-monitoring"
  namespace = "cattle-monitoring-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-monitoring"
  chart_version = var.monitoring_version
  wait = true
}

resource "rancher2_app_v2" "rancher-kiali-server-crd" {
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-kiali-server-crd"
  namespace = "istio-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-kiali-server-crd"
  chart_version = var.kiali_version
   depends_on = [rancher2_app_v2.monitoring]
}

resource "rancher2_app_v2" "istio" {
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-istio"
  namespace = "istio-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-istio"
  chart_version = var.istio_version
  depends_on = [rancher2_app_v2.monitoring,rancher2_app_v2.rancher-kiali-server-crd]
}

resource "rancher2_app_v2" "logging" {
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-logging"
  namespace = "cattle-logging-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-logging"
  chart_version = var.logging_version
  wait = true
}

resource "rancher2_app_v2" "cis-benchmark" {
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-cis-benchmark"
  namespace = "cis-operator-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-cis-benchmark"
  chart_version = var.cis_version
  wait = true
}


resource "rancher2_app_v2" "rancher-gatekeeper" {  
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
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
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-longhorn"
  namespace = "longhorn-system"
  repo_name = "rancher-charts"
  chart_name = "longhorn"
  chart_version = var.longhorn_version
  wait = true
}
