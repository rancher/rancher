from .common import *   # NOQA
import requests
import pytest

CREDENTIALS = os.environ.get('RANCHER_GKE_CREDENTIAL', "")
GKE_MASTER_VERSION = os.environ.get('RANCHER_GKE_MASTER_VERSION', "")

gkecredential = pytest.mark.skipif(not CREDENTIALS, reason='GKE Credentials '
                                   'not provided, cannot create cluster')


@gkecredential
def test_create_gke_cluster():

    client = get_user_client()
    gkeConfig = get_gke_config()

    print("Cluster creation")
    cluster = client.create_cluster(gkeConfig)
    print(cluster)
    cluster = validate_cluster(client, cluster, check_intermediate_state=True,
                               skipIngresscheck=True)

    cluster_cleanup(client, cluster)


def get_gke_version_credentials():
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
        gkemasterversion = validMasterVersions[0]
    else:
        gkemasterversion = GKE_MASTER_VERSION
    print(gkemasterversion)
    return gkemasterversion, credentialdata


def get_gke_config():

    # Obtain GKE master version and credentials
    gkemasterversion, credentialdata = get_gke_version_credentials()
    # Get GKE configuration
    gkeConfig = {
        "type": "cluster",
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
            "masterVersion": gkemasterversion,
            "machineType": "g1-small",
            "type": "googleKubernetesEngineConfig",
            "nodeCount": 3,
            "zone": "us-central1-f",
            "clusterIpv4Cidr": " ",
            "credential": credentialdata,
            "projectId": "rancher-qa",

        },
        "name": "test-auto-gke",
        "type": "cluster"
    }

    return gkeConfig
