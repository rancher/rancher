#Target cluster for v2 charts install
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

resource "rancher2_cluster_sync" "target-sync" {
  cluster_id =  var.cluster_id
  wait_catalogs = true
  state_confirm = 30
}

resource "local_file" "sync_kube_config" {
  content  = rancher2_cluster_sync.target-sync.kube_config
  filename = "kube_config_target_cluster.yaml"
  
  depends_on = [
    rancher2_cluster_sync.target-sync
  ]
}

resource "rancher2_app_v2" "monitoring" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-monitoring"
  namespace = "system"
  repo_name = "rancher-charts"
  chart_name = "rancher-monitoring"
  chart_version = var.monitoring_version
  
  depends_on = [
    rancher2_cluster_sync.target-sync
  ]
}

resource "rancher2_app_v2" "rancher-kiali-server-crd" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-kiali-server-crd"
  namespace = "istio-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-kiali-server-crd"
  chart_version = var.kiali_version
  wait = true
  
  depends_on = [
    rancher2_app_v2.monitoring
  ]
}

resource "rancher2_app_v2" "rancher-tracing" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-tracing"
  namespace = "istio-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-tracing"
  chart_version = var.tracing_version
  wait = true

  depends_on = [
    rancher2_app_v2.monitoring
  ]
}

resource "rancher2_app_v2" "istio" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-istio"
  namespace = "istio-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-istio"
  chart_version = var.istio_version
  wait = true

  depends_on = [
    rancher2_app_v2.monitoring,
    rancher2_app_v2.rancher-kiali-server-crd
  ]
}

resource "rancher2_app_v2" "logging" {
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-logging"
  namespace = "system"
  repo_name = "rancher-charts"
  chart_name = "rancher-logging"
  chart_version = var.logging_version
  wait = true

  depends_on = [
    rancher2_cluster_sync.target-sync
  ]
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

  depends_on = [
    rancher2_cluster_sync.target-sync
  ]
}

resource "rancher2_app_v2" "rancher-gatekeeper" {  
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-gatekeeper"
  namespace = "cattle-gatekeeper-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-gatekeeper"
  chart_version = var.gatekeeper_version
  wait = true
  
  depends_on = [
    rancher2_app_v2.monitoring
  ]
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
  count = var.is_admin
}

resource "null_resource" "longhorn-iscsi-nfs" {
  provisioner "local-exec" {
    environment = {
      KUBECONFIG = "kube_config_target_cluster.yaml"
    }
    command = "kubectl create namespace longhorn-system && kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/${var.longhorn_prereq_version}/deploy/prerequisite/longhorn-iscsi-installation.yaml --namespace=longhorn-system && kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/${var.longhorn_prereq_version}/deploy/prerequisite/longhorn-nfs-installation.yaml --namespace=longhorn-system"
  }

  depends_on = [
    local_file.sync_kube_config
  ]
}

resource "rancher2_app_v2" "longhorn" {  
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-longhorn"
  namespace = "longhorn-system"
  repo_name = "rancher-charts"
  chart_name = "longhorn"
  chart_version = var.longhorn_version
  
  depends_on = [
    null_resource.longhorn-iscsi-nfs
  ]
}


resource "rancher2_app_v2" "rancher-alerting-drivers" {  
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "rancher-alerting-drivers"
  namespace = "cattle-monitoring-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-alerting-drivers"
  chart_version = var.alerting_version
  values = <<EOF
  global:
    cattle:
      systemDefaultRegistry: ''
    kubectl:
      repository: rancher/kubectl
      tag: v1.20.2
    namespaceOverride: ''
  prom2teams:
    enabled: false
  sachet:
    enabled: true
  EOF

  depends_on = [
    rancher2_app_v2.monitoring
  ]
}

resource "null_resource" "longhorn-statefulset-example" {
  provisioner "local-exec" {
    environment = {
      KUBECONFIG = "kube_config_target_cluster.yaml"
    }
    command = "kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/master/examples/statefulset.yaml"
  }

  depends_on = [
    rancher2_app_v2.longhorn
  ]
}


resource "rancher2_app_v2" "neuvector" {  
  cluster_id = var.cluster_id
  project_id = var.project_id
  name = "neuvector"
  namespace = "cattle-neuvector-system"
  repo_name = "rancher-charts"
  chart_name = "neuvector"
  chart_version = var.neuvector_version
  values = <<EOF
  admissionwebhook:
    type: ClusterIP
  crdwebhook:
    enabled: true
    type: ClusterIP
  docker:
    path: /var/run/docker.sock
    enabled: ${var.docker_install}
  k3s:
    enabled: ${var.k3s_install}
    runtimePath: /run/k3s/containerd/containerd.sock
  oem: null
  openshift: false
  psp: false
  rbac: true
  registry: docker.io
  resources: {}
  serviceAccount: neuvector
  global:
    cattle:
      clusterId: ${var.cluster_id}
      rkePathPrefix: ''
      rkeWindowsPathPrefix: ''
      systemDefaultRegistry: ''
      systemProjectId: ${var.project_id}
      url: ${var.rancher_api_url}
    systemDefaultRegistry: ''
  EOF
  count = var.rancher_version_26_or_higher
  
  depends_on = [
    rancher2_cluster_sync.target-sync
  ]
}