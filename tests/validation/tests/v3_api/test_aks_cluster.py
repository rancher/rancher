from .common import *   # NOQA
import pytest
import requests

AKS_CLUSTER_VERSION = os.environ.get('RANCHER_AKS_CLUSTER_VERSION', '')
SSH_KEY = os.environ.get('RANCHER_SSH_KEY', "")
SUBSCRIPTION_ID = os.environ.get('RANCHER_AKS_SUBSCRIPTION_ID', '')
TENANT_ID = os.environ.get('RANCHER_AKS_TENANT_ID', '')
CLIENT_ID = os.environ.get('RANCHER_AKS_CLIENT_ID', '')
SECRET_KEY = os.environ.get('RANCHER_AKS_SECRET_KEY', '')
RESOURCE_GROUP = os.environ.get('RANCHER_AKS_RESOURCE_GROUP', '')
AKS_REGION = os.environ.get('RANCHER_AKS_REGION', 'eastus')
akscredential = pytest.mark.skipif(not (SUBSCRIPTION_ID and TENANT_ID and
                                   CLIENT_ID and SECRET_KEY),
                                   reason='AKS Credentials not provided, '
                                          'cannot create cluster')


@akscredential
def test_create_aks_cluster():
    version = get_aks_version()
    client, cluster = create_and_validate_aks_cluster(version)
    cluster_cleanup(client, cluster)


def create_and_validate_aks_cluster(version):
    client = get_user_client()
    aks_config = get_aks_config(version)

    print("Cluster creation")
    cluster = client.create_cluster(aks_config)
    cluster = validate_cluster(client, cluster, check_intermediate_state=True,
                               skipIngresscheck=True)
    return client, cluster


def get_aks_version(multiple_versions=False):

    if not AKS_CLUSTER_VERSION:
        data_test = {
            "region": "eastus",
            "subscriptionId": SUBSCRIPTION_ID,
            "tenantId": TENANT_ID,
            "clientId": CLIENT_ID,
            "clientSecret": SECRET_KEY
        }
        headers = {"Content-Type": "application/json",
                   "Accept": "application/json",
                   "Authorization": "Bearer " + USER_TOKEN}

        aks_version_url = CATTLE_TEST_URL + "/meta/aksVersions"
        print(aks_version_url)
        response = requests.post(aks_version_url, json=data_test,
                                 verify=False, headers=headers)

        assert response.status_code == 200
        assert response.content is not None
        print("JSON RESPONSE IS")
        print(response.content)
        json_response = json.loads(response.content)

        if multiple_versions and len(json_response) > 1:
            aksclusterversion = [json_response[0], json_response[-1]]
        else:
            aksclusterversion = json_response[-1]
    else:
        aksclusterversion = AKS_CLUSTER_VERSION

    print(aksclusterversion)
    return aksclusterversion


def get_aks_config(version):
    aks_config = {
        "azureKubernetesServiceConfig": {
            "adminUsername": "azureuser",
            "agentPoolName": "rancher",
            "agentVmSize": "Standard_D2_v2",
            "clientId": CLIENT_ID,
            "clientSecret": SECRET_KEY,
            "count": 3,
            "dnsServiceIp": None,
            "dockerBridgeCidr": None,
            "kubernetesVersion": version,
            "location": AKS_REGION,
            "osDiskSizeGb": 100,
            "resourceGroup": RESOURCE_GROUP,
            "serviceCidr": None,
            "sshPublicKeyContents": SSH_KEY,
            "subnet": None,
            "subscriptionId": SUBSCRIPTION_ID,
            "tenantId": TENANT_ID,
            "type": "azureKubernetesServiceConfig",
            "virtualNetwork": None,
            "virtualNetworkResourceGroup": None,
            "dockerRootDir": "/var/lib/docker",
            "enableNetworkPolicy": False,
        },
        "name": random_test_name("test-auto-aks"),
        "type": "cluster"
    }
    return aks_config
