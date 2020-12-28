from .common import *   # NOQA
import requests
import pytest

CREDENTIALS = os.environ.get('RANCHER_GKE_CREDENTIAL', "")
GKE_MASTER_VERSION = os.environ.get('RANCHER_GKE_MASTER_VERSION', "")

gkecredential = pytest.mark.skipif(not CREDENTIALS, reason='GKE Credentials '
                                   'not provided, cannot create cluster')


@gkecredential
def test_create_gke_cluster():
    # Obtain GKE config data
    gke_version, credential_data = get_gke_version_credentials()
    client, cluster = create_and_validate_gke_cluster("test-auto-gke",
                                                      gke_version,
                                                      credential_data)
    cluster_cleanup(client, cluster)


def create_and_validate_gke_cluster(name, version, credential_data):
    gke_config = get_gke_config(name, version, credential_data)
    client = get_user_client()
    print("Cluster creation")
    cluster = client.create_cluster(gke_config)
    print(cluster)
    cluster = validate_cluster(client, cluster, check_intermediate_state=True,
                               skipIngresscheck=True)
    return client, cluster


def get_gke_version_credentials(multiple_versions=False):
    credfilename = "credential.txt"
    PATH = os.path.dirname(os.path.realpath(__file__))
    credfilepath = PATH + "/" + credfilename

    # The json GKE credentials file is being written to a file and then re-read

    f = open(credfilepath, "w")
    f.write(CREDENTIALS)
    f.close()

    credentialdata = readDataFile(os.path.dirname(os.path.realpath(__file__)) +
                                  "/", credfilename)
    print(credentialdata)

    if not GKE_MASTER_VERSION:
        data_test = {
            "credentials": credentialdata,
            "zone": "us-central1-f",
            "projectId": "rancher-qa"
        }
        headers = {"Content-Type": "application/json",
                   "Accept": "application/json",
                   "Authorization": "Bearer " + USER_TOKEN}

        gke_version_url = CATTLE_TEST_URL + "/meta/gkeVersions"
        print(gke_version_url)
        response = requests.post(gke_version_url, json=data_test,
                                 verify=False, headers=headers)

        assert response.status_code == 200
        assert response.content is not None
        print(response.content)
        json_response = json.loads(response.content)
        validMasterVersions = json_response["validMasterVersions"]
        if multiple_versions and len(validMasterVersions) > 1:
            gkemasterversion = [validMasterVersions[0],
                                validMasterVersions[-1]]
        else:
            gkemasterversion = validMasterVersions[0]
    else:
        gkemasterversion = GKE_MASTER_VERSION
    print(gkemasterversion)
    return gkemasterversion, credentialdata


def get_gke_config(name, version, credential_data):

    # Get GKE configuration
    gke_config = {
        "googleKubernetesEngineConfig": {
            "diskSizeGb": 100,
            "enableAlphaFeature": False,
            "enableHorizontalPodAutoscaling": True,
            "enableHttpLoadBalancing": True,
            "enableKubernetesDashboard": False,
            "enableLegacyAbac": False,
            "enableNetworkPolicyConfig": True,
            "enableStackdriverLogging": True,
            "enableStackdriverMonitoring": True,
            "masterVersion": version,
            "machineType": "g1-small",
            "type": "googleKubernetesEngineConfig",
            "nodeCount": 3,
            "zone": "us-central1-f",
            "clusterIpv4Cidr": " ",
            "credential": credential_data,
            "projectId": "rancher-qa",

        },
        "name": name,
        "type": "cluster"
    }

    return gke_config
