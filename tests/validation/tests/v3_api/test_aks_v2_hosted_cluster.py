import os
from .common import *
import pytest

AZURE_SUBSCRIPTION_ID = os.environ.get("AZURE_SUBSCRIPTION_ID")
AZURE_CLIENT_ID = os.environ.get("AZURE_CLIENT_ID")
AZURE_CLIENT_SECRET = os.environ.get("AZURE_CLIENT_SECRET")
RANCHER_AKS_K8S_VERSION = os.environ.get("RANCHER_AKS_K8S_VERSION", "1.21.2")
RANCHER_AKS_DNS_PREFIX = os.environ.get("RANCHER_AKS_DNS_PREFIX")
RANCHER_AKS_RESOURCE_GROUP = os.environ.get("RANCHER_AKS_RESOURCE_GROUP")
RANCHER_AKS_RESOURCE_LOCATION = os.environ.get("RANCHER_AKS_RESOURCE_LOCATION")
aks_credentials = pytest.mark.skipif(not (AZURE_SUBSCRIPTION_ID and AZURE_CLIENT_ID and AZURE_CLIENT_SECRET),
                                     reason='Azure credentials not provided, '
                                            'cannot create cluster')
aks_resources = pytest.mark.skipif(not (RANCHER_AKS_DNS_PREFIX and RANCHER_AKS_RESOURCE_GROUP and RANCHER_AKS_RESOURCE_LOCATION),
                                   reason='AKS resources not provided, '
                                          'cannot create cluster')
DEFAULT_TIMEOUT_AKS = 600
cluster_details = {}

aks_config = {
    "imported": False,
    "dnsPrefix": RANCHER_AKS_DNS_PREFIX,
    "kubernetesVersion": RANCHER_AKS_K8S_VERSION,
    "linuxAdminUsername": "azureuser",
    "loadBalancerSku": "Standard",
    "networkPlugin": "kubenet",
    "privateCluster": False,
    "resourceGroup": RANCHER_AKS_RESOURCE_GROUP,
    "resourceLocation": RANCHER_AKS_RESOURCE_LOCATION,
    "tags": {},
    "type": "aksclusterconfigspec",
    "nodePools": [
        {
            "availabilityZones": ["1", "2", "3"],
            "count": 3,
            "enableAutoScaling": False,
            "maxPods": 110,
            "mode": "System",
            "name": "agentpool",
            "orchestratorVersion": RANCHER_AKS_K8S_VERSION,
            "osDiskSizeGB": 128,
            "osDiskType": "Managed",
            "osType": "Linux",
            "type": "aksnodepool",
            "vmSize": "Standard_DS2_v2",
            "isNew": True
        }
    ]
}


@aks_credentials
@aks_resources
def test_aks_v2_hosted_cluster_create_basic():
    """
    Create a hosted AKS v2 cluster with basic UI parameters
    """
    client = get_user_client()
    cluster_name = random_test_name("test-auto-aks")
    aks_config_temp = get_aks_config(cluster_name)
    cluster_config = {
        "aksConfig": aks_config_temp,
        "name": cluster_name,
        "type": "cluster",
        "dockerRootDir": "/var/lib/docker",
        "enableNetworkPolicy": False,
        "enableClusterAlerting": False,
        "enableClusterMonitoring": False
    }
    cluster = create_and_validate_aks_cluster(cluster_config)
    hosted_cluster_cleanup(client, cluster, cluster_name)


def get_aks_config(cluster_name):
    """
    Adding required params for a basic AKS v2 cluster
    :param cluster_name:
    :return: aks_config_temp
    """
    azure_cloud_credential = get_azure_cloud_credential()
    global aks_config
    aks_config_temp = aks_config.copy()
    aks_config_temp["clusterName"] = cluster_name
    aks_config_temp["azureCredentialSecret"] = azure_cloud_credential.id
    return aks_config_temp


def get_azure_cloud_credential():
    """
    Create an Azure cloud credentials
    :return:  azure_cloud_credential
    """
    client = get_user_client()
    azure_cloud_credential_config = {
        "subscriptionId": AZURE_SUBSCRIPTION_ID,
        "clientId": AZURE_CLIENT_ID,
        "clientSecret": AZURE_CLIENT_SECRET
    }
    azure_cloud_credential = client.create_cloud_credential(
        azurecredentialConfig=azure_cloud_credential_config
    )
    return azure_cloud_credential


def create_and_validate_aks_cluster(cluster_config, imported=False):
    """
    Create and validate an AKS cluster
    :param cluster_config: config of the cluster
    :param imported: imported is true when user creates an imported cluster
    :return: client, cluster
    """
    client = get_user_client()
    print("Creating AKS cluster")
    print("\nAKS Configuration: {}".format(cluster_config))
    cluster = client.create_cluster(cluster_config)
    print(cluster)
    cluster_details[cluster["name"]] = cluster
    intermediate_state = False if imported else True
    cluster = validate_cluster(client, cluster,
                               check_intermediate_state=intermediate_state,
                               skipIngresscheck=True,
                               timeout=DEFAULT_TIMEOUT_AKS)
    return client, cluster