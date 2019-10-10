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

    client = get_user_client()
    aksConfig = get_aks_config()

    print("Cluster creation")
    cluster = client.create_cluster(aksConfig)
    cluster = validate_cluster(client, cluster, check_intermediate_state=True,
                               skipIngresscheck=True)
    cluster_cleanup(client, cluster)


def get_aks_version():

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
        versionarray_length = len(json_response)
        aksclusterversion = json_response[versionarray_length-1]
        print(aksclusterversion)
    else:
        aksclusterversion = AKS_CLUSTER_VERSION

    print(aksclusterversion)
    return aksclusterversion


def get_aks_config():

    # Generate the config for AKS cluster
    aksclusterversion = get_aks_version()

    print(aksclusterversion)

    aksConfig = {
        "azureKubernetesServiceConfig": {

            "adminUsername": "azureuser",
            "agentPoolName": "rancher",
            "agentVmSize": "Standard_D2_v2",
            "clientId": CLIENT_ID,
            "clientSecret": SECRET_KEY,
            "count": 3,
            "dnsServiceIp": None,
            "dockerBridgeCidr": None,
            "kubernetesVersion": aksclusterversion,
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

    return aksConfig
