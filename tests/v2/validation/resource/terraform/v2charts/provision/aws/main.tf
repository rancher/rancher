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
    kubernetes_version = var.rancher_k8s_version
    network {
      plugin = "canal"
    }
  }  
}

resource "rancher2_node_template" "node-template" {
  name = "tf-aws-node-template"
  description = "TF AWS node template"

  amazonec2_config {
    access_key = var.aws_access_key
    secret_key = var.aws_secret_key
    ami =  var.aws_ami
    ssh_user = var.aws_ami_user
    region = var.aws_region
    security_group = var.aws_security_groups
    subnet_id = var.aws_subnet
    vpc_id = var.aws_vpc_id
    zone = var.aws_zone
    root_size = var.aws_root_size
    instance_type = var.aws_instance_type
    volume_type = var.aws_volume_type
  }
}

resource "rancher2_node_pool" "cp-node-pool" {
  cluster_id =  rancher2_cluster.v2charts.id
  name = "tf-cp-node-pool"
  hostname_prefix = var.node_pool_name_cp
  node_template_id = rancher2_node_template.node-template.id
  quantity = 2
  control_plane = true
  etcd = false
  worker = false
}

resource "rancher2_node_pool" "etcd-node-pool" {
  cluster_id =  rancher2_cluster.v2charts.id
  name = "tf-etcd-node-pool"
  hostname_prefix = var.node_pool_name_etcd
  node_template_id = rancher2_node_template.node-template.id
  quantity = 3
  control_plane = false
  etcd = true
  worker = false
}

resource "rancher2_node_pool" "worker-node-pool" {
  cluster_id =  rancher2_cluster.v2charts.id
  name = "tf-worker-node-pool"
  hostname_prefix = var.node_pool_name_worker
  node_template_id = rancher2_node_template.node-template.id
  quantity = var.worker_count
  control_plane = false
  etcd = false
  worker = true
}

resource "rancher2_cluster_sync" "v2charts-sync" {
  cluster_id = rancher2_cluster.v2charts.id
  wait_catalogs = true
  state_confirm = 90
}

resource "local_file" "sync_kube_config" {
  content  = rancher2_cluster_sync.v2charts-sync.kube_config
  filename = "kube_config_provisioned_cluster.yaml"
  
  depends_on = [
    rancher2_cluster_sync.v2charts-sync
  ]
}

resource "null_resource" "longhorn-iscsi-nfs" {
  provisioner "local-exec" {
    environment = {
      KUBECONFIG = "kube_config_provisioned_cluster.yaml"
    }
    command = "kubectl create namespace longhorn-system && kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/${var.rancher_longhorn_prereq_version}/deploy/prerequisite/longhorn-iscsi-installation.yaml --namespace=longhorn-system && kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/${var.rancher_longhorn_prereq_version}/deploy/prerequisite/longhorn-nfs-installation.yaml --namespace=longhorn-system"
  }

  depends_on = [
    local_file.sync_kube_config
  ]
}

resource "rancher2_app_v2" "monitoring" {
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-monitoring"
  namespace = "cattle-monitoring-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-monitoring"
  chart_version = var.rancher_monitoring_version
  wait = true
}

resource "rancher2_app_v2" "rancher-kiali-server-crd" {
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-kiali-server-crd"
  namespace = "istio-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-kiali-server-crd"
  chart_version = var.rancher_kiali_version
  wait = true

  depends_on = [
    rancher2_app_v2.monitoring
  ]
}

resource "rancher2_app_v2" "rancher-tracing" {
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-tracing"
  namespace = "istio-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-tracing"
  chart_version = var.rancher_tracing_version
  wait = true

  depends_on = [
    rancher2_app_v2.monitoring
  ]
}

resource "rancher2_app_v2" "istio" {
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-istio"
  namespace = "istio-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-istio"
  chart_version = var.rancher_istio_version
  wait = true

  depends_on = [
    rancher2_app_v2.monitoring,
    rancher2_app_v2.rancher-kiali-server-crd
  ]
}

resource "rancher2_app_v2" "logging" {
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-logging"
  namespace = "cattle-logging-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-logging"
  chart_version = var.rancher_logging_version
  wait = true
}

resource "rancher2_app_v2" "cis-benchmark" {
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-cis-benchmark"
  namespace = "cis-operator-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-cis-benchmark"
  chart_version = var.rancher_cis_version
  wait = true
}

resource "rancher2_app_v2" "rancher-gatekeeper" {  
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-gatekeeper"
  namespace = "cattle-gatekeeper-system"
  repo_name = "rancher-charts"
  chart_name = "rancher-gatekeeper"
  chart_version = var.rancher_gatekeeper_version
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
  chart_version = var.rancher_backup_version
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
  count = var.install_rancher_backups
}

resource "rancher2_app_v2" "longhorn" {  
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-longhorn"
  namespace = "longhorn-system"
  repo_name = "rancher-charts"
  chart_name = "longhorn"
  chart_version = var.rancher_longhorn_version
  wait = true

  depends_on = [
    null_resource.longhorn-iscsi-nfs
  ]
}

resource "rancher2_app_v2" "rancher-alerting-drivers" {  
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "rancher-alerting-drivers"
  namespace = "default"
  repo_name = "rancher-charts"
  chart_name = "rancher-alerting-drivers"
  chart_version = var.rancher_alerting_version
  values = <<EOF
  global:
    cattle:
      systemDefaultRegistry: ''
    kubectl:
      repository: rancher/kubectl
    namespaceOverride: ''
  prom2teams:
    enabled: false
  sachet:
    enabled: true
  EOF
  wait = true
}

resource "null_resource" "longhorn-statefulset-example" {
  provisioner "local-exec" {
    environment = {
      KUBECONFIG = "kube_config_provisioned_cluster.yaml"
    }
    command = "kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/master/examples/statefulset.yaml"
  }

  depends_on = [
    rancher2_app_v2.longhorn
  ]
}

resource "rancher2_app_v2" "neuvector" {  
  cluster_id = rancher2_cluster.v2charts.id
  project_id = rancher2_cluster_sync.v2charts-sync.system_project_id
  name = "neuvector"
  namespace = "cattle-neuvector-system"
  repo_name = "rancher-charts"
  chart_name = "neuvector"
  chart_version = var.rancher_neuvector_version
  values = <<EOF
  admissionwebhook:
    type: ClusterIP
  crdwebhook:
    enabled: true
    type: ClusterIP
  docker:
    path: /var/run/docker.sock
    enabled: true
  oem: null
  openshift: false
  psp: false
  rbac: true
  registry: docker.io
  resources: {}
  serviceAccount: neuvector
  global:
    cattle:
      clusterId: ${rancher2_cluster.v2charts.id}
      clusterName: ${var.cluster_name}
      rkePathPrefix: ''
      rkeWindowsPathPrefix: ''
      systemDefaultRegistry: ''
      systemProjectId: ${rancher2_cluster_sync.v2charts-sync.system_project_id}
      url: ${var.rancher_api_url}
    systemDefaultRegistry: ''
  EOF
  wait = true
  count = var.install_rancher_neuvector
}