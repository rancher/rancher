from .common import *   # NOQA
import requests
import pytest

CREDENTIALS = os.environ.get('RANCHER_GKE_CREDENTIAL', "")
GKE_MASTER_VERSION = os.environ.get('RANCHER_GKE_MASTER_VERSION', "")

gkecredential = pytest.mark.skipif(not CREDENTIALS, reason='GKE Credentials '
                                   'not provided, cannot create cluster')
CUSTOM_INGRESS_DOMAIN = "foo.bar"


@gkecredential
def test_create_gke_cluster():

    client = get_user_client()
    gkeConfig = get_gke_config()

    print("Cluster creation")
    cluster = client.create_cluster(gkeConfig)
    print(cluster)
    cluster = validate_cluster(client, cluster, check_intermediate_state=True,
                               skipIngresscheck=True)

    # validate settings are propagated to cluster level api
    test_cluster_ingress_ip_domain_setting(client, cluster)

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


def readDataFile(data_dir, name):
    fname = os.path.join(data_dir, name)
    print(fname)
    is_file = os.path.isfile(fname)
    assert is_file
    with open(fname) as f:
        return f.read()


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


def test_cluster_ingress_ip_domain_setting(client, cluster):

    # set custom ingress-ip-domain setting
    ingress_ip_domain = client.by_id_setting('ingress-ip-domain')
    print(ingress_ip_domain)
    updated_ingress_ip_domain = client.update_by_id_setting(
        id='ingress-ip-domain', value=CUSTOM_INGRESS_DOMAIN)
    assert updated_ingress_ip_domain.value == CUSTOM_INGRESS_DOMAIN

    # create new namespace for ingress custom domain test
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testingressdoamin")
    p_client = get_project_client_for_token(p, USER_TOKEN)

    name = random_str()
    workload = p_client.create_workload(
        name=name,
        namespaceId=ns.id,
        scale=1,
        containers=[{
            'name': 'foo',
            'image': 'nginx',
            'ports': [
                {
                    'containerPort': 80,
                    'kind': 'ClusterIP',
                    'protocol': 'TCP',
                }
            ]
        }])
    wait_state(p_client, workload, "active")

    name = random_str()
    ingress = p_client.create_ingress(name=name,
                                      namespaceId=ns.id,
                                      rules=[{
                                          'host': CUSTOM_INGRESS_DOMAIN,
                                          'paths': [
                                              {
                                                  'targetPort': 80,
                                                  'workloadIds':
                                                      [workload.id],
                                              },
                                          ]},
                                      ])

    assert len(ingress.rules) == 1
    assert ingress.rules[0].host == CUSTOM_INGRESS_DOMAIN
    path = ingress.rules[0].paths[0]
    assert path.targetPort == 80
    assert path.workloadIds == [workload.id]
    assert path.serviceId is None

    # check the ingress generated service
    obj = wait_state(p_client, ingress, "active")
    assert len(obj.publicEndpoints) == 1
    service_id = obj.publicEndpoints[0].serviceId
    assert service_id != ""
    time.sleep(5)

    # https://github.com/rancher/rancher/issues/24018
    ingress_generated_svc = p_client.by_id_service(service_id)
    assert ingress_generated_svc is not None
    p_client.delete(ns)
