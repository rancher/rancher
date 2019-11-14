import pytest, copy
from .common import *  # NOQA

RANCHER_VSPHERE_USERNAME = os.environ.get("RANCHER_VSPHERE_USERNAME", "")
RANCHER_VSPHERE_PASSWORD = os.environ.get("RANCHER_VSPHERE_PASSWORD", "")
RANCHER_VSPHERE_VCENTER = os.environ.get("RANCHER_VSPHERE_VCENTER", "")
RANCHER_VSPHERE_VCENTER_PORT = \
    os.environ.get("RANCHER_VSPHERE_VCENTER_PORT", 443)
RANCHER_CLEANUP_CLUSTER = \
    ast.literal_eval(os.environ.get('RANCHER_CLEANUP_CLUSTER', "True"))
CLUSTER_NAME = os.environ.get("RANCHER_CLUSTER_NAME",
                              random_name() + "-cluster")
ENGINE_INSTALL_URL = os.environ.get("RANCHER_ENGINE_INSTALL_URL",
                                    "https://get.docker.com/")
CLONE_FROM = \
    os.environ.get("RANCHER_CLONE_FROM",
                   "/RNCH-HE-FMT/vm/ubuntu-bionic-18.04-cloudimg")
RESOURCE_POOL = \
    os.environ.get("RANCHER_RESOURCE_POOL",
                   "/RNCH-HE-FMT/host/FMT2.R620.1/Resources/validation-tests")
DATASTORE = \
    os.environ.get("RANCHER_DATASTORE",
                   "/RNCH-HE-FMT/datastore/ranch01-silo01-vm01")

DATASTORE_CLUSTER = \
    os.environ.get("RANCHER_DATASTORE_CLUSTER",
                   "/RNCH-HE-FMT/datastore/ds_cluster")

CLOUD_CONFIG = \
    os.environ.get("RANCHER_CLOUD_CONFIG",
                   "#cloud-config\r\npackages:\r\n - redis-server")

rke_config = {
    "addonJobTimeout": 30,
    "authentication":
        {"strategy": "x509",
         "type": "authnConfig"},
    "ignoreDockerVersion": True,
    "ingress":
        {"provider": "nginx",
         "type": "ingressConfig"},
    "monitoring":
        {"provider": "metrics-server",
         "type": "monitoringConfig"},
    "network":
        {"plugin": "canal",
         "type": "networkConfig",
         "options": {"flannelBackendType": "vxlan"}},
    "services": {
        "etcd": {
            "extraArgs":
                {"heartbeat-interval": 500,
                 "election-timeout": 5000},
            "snapshot": False,
            "backupConfig":
                {"intervalHours": 12, "retention": 6, "type": "backupConfig"},
            "creation": "12h",
            "retention": "72h",
            "type": "etcdService"},
        "kubeApi": {
            "alwaysPullImages": False,
            "podSecurityPolicy": False,
            "serviceNodePortRange": "30000-32767",
            "type": "kubeAPIService"}},
    "sshAgentAuth": False}

vsphereConfig = {
    "cfgparam": ["disk.enableUUID=TRUE"],
    "cloneFrom": CLONE_FROM,
    "cloudinit": "",
    "contentLibrary": "",
    "cpuCount": "4",
    "creationType": "vm",
    "customAttribute": ["203=CustomA", "204=CustomB"],
    "datacenter": "/RNCH-HE-FMT",
    "datastore": "",
    "datastoreCluster": "",
    "diskSize": "20000",
    "folder": "/",
    "hostsystem": "",
    "memorySize": "16000",
    "network": ["/RNCH-HE-FMT/network/Private Range 172.16.128.1-21"],
    "password": "",
    "pool": RESOURCE_POOL,
    "sshPassword": "tcuser",
    "sshPort": "22",
    "sshUser": "docker",
    "sshUserGroup": "staff",
    "tag": [
        "urn:vmomi:InventoryServiceTag:04ffafd0-d7de-440c-a32c-5cd98761f812:GLOBAL",
        "urn:vmomi:InventoryServiceTag:d00f1cf2-6822-46a0-9602-679ea56efd57:GLOBAL"
    ],
    "type": "vmwarevsphereConfig",
    "username": "",
    "vappIpallocationpolicy": "",
    "vappIpprotocol": "",
    "vappProperty": "",
    "vappTransport": "",
    "vcenter": "",
    "vcenterPort": "443",
}


def test_valid_environment_variables():
    assert RANCHER_VSPHERE_USERNAME != '', \
        "vSphere User is required to make a cloud credential"
    assert RANCHER_VSPHERE_PASSWORD != '', \
        "vSphere Password is required to make a cloud credential"
    assert RANCHER_VSPHERE_VCENTER != '', \
        "vCenter URL is required to make a cloud credential"


@pytest.mark.usefixtures("create_cluster")
def test_nodes_ready():
    client = get_client_for_token(ADMIN_TOKEN)
    cluster = get_cluster_by_name(client=client, name=CLUSTER_NAME)
    nodes = client.list_node(clusterId=cluster.id).data
    assert 4 == len(nodes)
    validate_cluster_state(client, cluster)


def test_ingress():
    namespace = create_namespace(ADMIN_TOKEN)
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    cluster = namespace["cluster"]
    node_count = len(get_schedulable_nodes(cluster))
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        scale=node_count,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})

    validate_workload(p_client, workload, "daemonSet", ns.name,
                      node_count)

    host = "test1.com"
    path = "/name.html"
    rule = {"host": host,
            "paths": [{"workloadIds": [workload.id], "targetPort": "80"}]}
    p_client.create_ingress(name=name,
                            namespaceId=ns.id,
                            rules=[rule])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], host, path)


@pytest.fixture(scope='module', autouse="True")
def create_cluster(request):
    client = get_client_for_token(ADMIN_TOKEN)
    cloud_cred = create_vsphere_credential(client)
    nt = create_vsphere_nodetemplate(
        client, cloud_cred, datastore=DATASTORE)
    ntcc = create_vsphere_nodetemplate(
        client, cloud_cred, datastore=DATASTORE, cloud_config=CLOUD_CONFIG)
    ntdsc = create_vsphere_nodetemplate(
        client, cloud_cred, datastore_cluster=DATASTORE_CLUSTER)
    cluster = client.create_cluster(
        name=CLUSTER_NAME,
        rancherKubernetesEngineConfig=rke_config)

    request.addfinalizer(cluster_cleanup)
    master_pool = client.create_node_pool({
        "type": "nodetemplate",
        "clusterId": cluster.id,
        "controlPlane": True,
        "etcd": True,
        "hostnamePrefix": CLUSTER_NAME + "-master",
        "nodeTemplateId": nt.id,
        "quantity": 1,
        "worker": False,
    })

    worker_pool1 = client.create_node_pool({
        "type": "nodetemplate",
        "clusterId": cluster.id,
        "controlPlane": False,
        "etcd": False,
        "hostnamePrefix": CLUSTER_NAME + "-worker",
        "nodeTemplateId": nt.id,
        "quantity": 1,
        "worker": True,
    })

    worker_pool2 = client.create_node_pool({
        "type": "nodetemplate",
        "clusterId": cluster.id,
        "controlPlane": False,
        "etcd": False,
        "hostnamePrefix": CLUSTER_NAME + "-worker-cc",
        "nodeTemplateId": ntcc.id,
        "quantity": 1,
        "worker": True,
    })

    worker_pool3 = client.create_node_pool({
        "type": "nodetemplate",
        "clusterId": cluster.id,
        "controlPlane": False,
        "etcd": False,
        "hostnamePrefix": CLUSTER_NAME + "-worker-dsc",
        "nodeTemplateId": ntdsc.id,
        "quantity": 1,
        "worker": True,
    })

    client.wait_success(master_pool)
    client.wait_success(worker_pool1)
    client.wait_success(worker_pool2)
    client.wait_success(worker_pool3)

    wait_for_cluster_node_count(client, cluster, 4, timeout=900)


def create_vsphere_credential(client):
    return client.create_cloud_credential(
        name=random_name(),
        vmwarevspherecredentialConfig={
            "username": RANCHER_VSPHERE_USERNAME,
            "password": RANCHER_VSPHERE_PASSWORD,
            "vcenter": RANCHER_VSPHERE_VCENTER,
            "vcenterPort": RANCHER_VSPHERE_VCENTER_PORT,
        }
    )


def cluster_cleanup():
    if not RANCHER_CLEANUP_CLUSTER:
        return
    client = get_client_for_token(ADMIN_TOKEN)
    cluster = get_cluster_by_name(client=client, name=CLUSTER_NAME)
    nodes = get_schedulable_nodes(cluster)
    delete_cluster(client, cluster)
    for node in nodes:
        wait_for_node_to_be_deleted(client, node)


def create_vsphere_nodetemplate(
        client, cloud_cred, cloud_config="", datastore="", datastore_cluster=""):
    vc = copy.copy(vsphereConfig)
    if cloud_config != "":
        vc["cloudConfig"] = cloud_config
    if datastore != "":
        vc["datastore"] = datastore
    if datastore_cluster != "":
        vc["datastoreCluster"] = datastore_cluster
    return client.create_node_template({
        "vmwarevsphereConfig": vc,
        "name": random_name(),
        "namespaceId": "fixme",
        "useInternalIpAddress": True,
        "driver": "vmwarevsphere",
        "engineInstallURL": ENGINE_INSTALL_URL,
        "cloudCredentialId": cloud_cred.id,
    })


def create_namespace(token):
    client = get_client_for_token(token)
    cluster = get_cluster_by_name(client=client, name=CLUSTER_NAME)
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(
        token, cluster, random_test_name("testworkload"))
    p_client = get_project_client_for_token(p, token)
    return {
        "p_client": p_client,
        "ns": ns,
        "cluster": cluster,
        "project": p,
    }
