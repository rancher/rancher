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

if_vsphere_var_present = pytest.mark.skipif(
    RANCHER_VSPHERE_USERNAME == '' or
    RANCHER_VSPHERE_PASSWORD == '' or
    RANCHER_VSPHERE_VCENTER == '',
    reason='required env variables are not present')


@if_vsphere_var_present
@pytest.mark.usefixtures("create_cluster")
def test_vsphere_provisioning():
    client = get_client_for_token(USER_TOKEN)
    cluster = get_cluster_by_name(client=client, name=CLUSTER_NAME)
    nodes = client.list_node(clusterId=cluster.id).data
    assert 4 == len(nodes)
    validate_cluster(client, cluster, skipIngresscheck=False)


@pytest.fixture(scope='module', autouse="True")
def create_cluster(request):
    client = get_client_for_token(USER_TOKEN)
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
    # Allow sometime for the "cluster_owner" CRTB to take effect
    time.sleep(5)

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
    client = get_client_for_token(USER_TOKEN)
    cluster = get_cluster_by_name(client=client, name=CLUSTER_NAME)
    nodes = get_schedulable_nodes(cluster)
    delete_cluster(client, cluster)
    for node in nodes:
        wait_for_node_to_be_deleted(client, node)


def create_vsphere_nodetemplate(
        client, cloud_cred, cloud_config="", datastore="",
        datastore_cluster=""):
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
