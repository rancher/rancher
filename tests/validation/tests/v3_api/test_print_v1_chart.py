import re
import os

import pytest
from .common import *
from .test_istio import enable_monitoring_istio
from .test_istio import get_system_client
from lib.aws import AmazonWebServices
from .test_custom_host_reg import test_deploy_rancher_server
from .test_custom_host_reg import create_custom_cluster

system_project_client = None
istio_ns = ""
monitoring_ns = ""
RANCHER_SERVER_VERSION = os.environ.get("RANCHER_SERVER_VERSION", '')
CATTLE_URL = os.environ.get('CATTLE_TEST_URL', '')
CATTLE_API_URL = CATTLE_URL + "/v3"
USER_TOKEN = os.environ.get('USER_TOKEN', "None")
HOST_NAME = os.environ.get('RANCHER_HOST_NAME', 'testsa')
RANCHER_SERVER_URL = os.environ.get('CATTLE_TEST_URL', '')
K8S_VERSION = os.environ.get('RANCHER_K8S_VERSION', "")
CLUSTER_CLEANUP =  ast.literal_eval(os.environ.get('RANCHER_CLEANUP_CLUSTER', "False"))

additional_answers = {
    "kiali.enabled": "true",
    "tracing.enabled": "true",
    "gateways.istio-egressgateway.enabled": "true",
    "gateways.istio-ilbgateway.enabled": "true",
    "gateways.istio-ingressgateway.sds.enabled": "true",
    "global.proxy.accessLogFile": "/dev/stdout",
    "istiocoredns.enabled": "true",
    "kiali.prometheusAddr": "http://prometheus:9090",
    "nodeagent.enabled": "true",
    "nodeagent.env.CA_ADDR": "istio-citadel:8060",
    "nodeagent.env.CA_PROVIDER": "Citadel",
    "prometheus.enabled": "true",
}
istio_expected_images = {"istio-citadel": "v1.5.9",
                         "istio-galley": "v1.5.9",
                         "istio-proxyv2": "v1.5.9",
                         "istio-pilot": "v1.5.9",
                         "istio-mixer": "v1.5.9",
                         "istio-sidecar_injector": "v1.5.9",
                         "jaegertracing-all-in-one": "v1.14",
                         "kiali-kiali": "v1.17",
                         "istio-node-agent-k8s": "v1.5.9",
                         "coredns-coredns": "v1.6.9",
                         "prom-prometheus": "v2.18.2"
                         }
monitoring_expected_images = {"prometheus": "v2.18.2",
                              "prometheus-operator": "v0.39.0",
                              "prometheus-config-reloader": "v0.39.0",
                              "prometheus-auth": "v0.2.1",
                              "node-exporter": "v1.0.1",
                              "kube-state-metrics": "v1.9.7",
                              "configmap-reload": "v0.3.0",
                              "grafana": "v7.1.5"}


def get_wk(ns_):
    data_ = system_project_client.list_workload(namespaceId=ns_).data
    container_images = list(set([c.image for wks in data_ for c in wks.containers if len(data_) >= 0 if c.image]))

    wk_image = dict()
    # splitting them on ":" to separate them to image and version
    for image in container_images:
        if image.startswith('rancher/'):
            img, ver = image.replace('rancher/', '').split(":")
            # Add 'v' to version if missing
            # For v1 versions >= 1.5.920 image names start with mirrored
            if img.startswith('mirrored'):
                img = "-".join(img.split("-")[1:])
            wk_image[img] = ver if ver.startswith("v") else 'v' + ver
    return wk_image


def print_wk(wk, expected_images):
    for w, ver in wk.items():
        if w in expected_images.keys():
            try:
                print("\nVersion of the image {} is {}".format(w, ver) + "\n")
            except Exception as e:
                print('\n ERROR \n')
                print("\n Error reported {}".format(e) + "\n")
                continue


@pytest.mark.skipif(False, reason="Only testing monitoring")
def test_istio():
    if istio_ns == '':
        raise Exception("Istio namespace is not found")
    istio_wk = get_wk(istio_ns)
    print_wk(istio_wk, istio_expected_images)


@pytest.mark.skipif(False, reason="Only testing istio")
def test_monitoring():
    image_name = 'prometheus-auth'
    if monitoring_ns == '':
        raise Exception("No namespace is detected uh-oh!")
    monitoring_wk = get_wk(monitoring_ns)
    monitoring_wk_final = {}
    # Some of the images names in release testing document differ
    # from that of the installed. To be in unison, removing the extra string in the installed
    for image in monitoring_wk.keys():
        if image != image_name:
            img = re.sub(r'^.*?-', '', image)
            monitoring_wk_final[img] = monitoring_wk[image]
        else:
            monitoring_wk_final[image] = monitoring_wk[image]
    print_wk(monitoring_wk_final, monitoring_expected_images)


def get_rancher_cluster_details(rancher_server_details):
    rancher_server_details = rancher_server_details.replace("'", "")
    env = rancher_server_details.replace("\n", ";")[:-1]
    env_details = dict(x.split("=") for x in env.split(";"))
    return env_details


def get_k8s_versionlist(admin_token, cattle_url):
    # Get the list of K8s version supported by the rancher server
    headers = {"Content-Type": "application/json",
               "Accept": "application/json",
               "Authorization": "Bearer " + admin_token}
    json_data = {
        'responseType': 'json'
    }
    settings_url = cattle_url + "/v3/settings/k8s-versions-current"
    response = requests.get(settings_url, json=json_data,
                            verify=False, headers=headers)
    json_response = (json.loads(response.content))
    k8sversionstring = json_response['value']
    k8sversionlist = k8sversionstring.split(",")
    assert len(k8sversionlist) > 1
    return k8sversionlist


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    global system_project_client, USER_TOKEN, RANCHER_SERVER_URL, K8S_VERSION
    global istio_ns, monitoring_ns
    global CATTLE_API_URL, CLUSTER_NAME
    cluster_name = os.environ.get('RANCHER_CLUSTER_NAME','')
    rancher_version = None
    k8sv1 = False
    # If rancher server is not installed, we deploy a new rancher server
    # Custom cluster will also be created for the istio and monitoring v1 installation.
    if CATTLE_URL == '':
        print("Creating a rancher server as the cattle URL is none")
        node_roles = [["controlplane"], ["etcd"],
                          ["worker"], ["worker"], ["worker"], ["worker"], ["worker"]]
        if rancher_version in 'v2.6':
            k8sv1 = True
        rancher_ = test_deploy_rancher_server(node_custom_roles = node_roles, k8sv1=k8sv1)
        rancher_cluster_details = get_rancher_cluster_details(rancher_)
        CATTLE_API_URL = os.environ['CATTLE_API_URL'] = rancher_cluster_details['env.CATTLE_TEST_URL'] + '/v3'
        cluster_name = rancher_cluster_details['env.CLUSTER_NAME']
        USER_TOKEN = rancher_cluster_details['env.USER_TOKEN']
        client = get_client_for_token(USER_TOKEN, url=CATTLE_API_URL)

        # RANCHER_SERVER_URL = os.environ['CATTLE_TEST_URL'] = rancher_details['CATTLE_TEST_URL']
        # print(os.environ.get('CATTLE_TEST_URL'))
        # USER_TOKEN = rancher_details['USER_TOKEN']
        # rancher_version = RANCHER_SERVER_VERSION
        # client = rancher_details['client']
    else:
        client = get_user_client()

    # If cluster is not provided by the user, a cluster will be created
    if cluster_name == "":
        node_roles = [["controlplane"], ["etcd"],
                      ["worker"], ["worker"], ["worker"], ["worker"], ["worker"]]
        if rancher_version == None:
            rancher_version = get_setting_value_by_name('server-version')
            rancher_version = ''.join(rancher_version.split(".")[:2]) if rancher_version.startswith('v') else None
        # for v2.6 and above rancher versions, k8s v1.21 is not recommended as v1 charts are deprecated

        if K8S_VERSION == '':
            if rancher_version in 'v2.6':
                k8s_list = sorted(get_k8s_versionlist(USER_TOKEN, RANCHER_SERVER_URL), reverse=True)
                for version in k8s_list:
                    version = '.'.join(version.split(".")[:2])
                    if version.parse(version) < version.parse('v1.21'):
                        K8S_VERSION = version
                        break

        cluster = create_custom_cluster(RANCHER_SERVER_URL, USER_TOKEN, node_roles,
                                        k8s_version=K8S_VERSION)
        os.environ['CLUSTER_NAME']= cluster.name
    else:
        cluster = get_cluster_by_name(client, cluster_name)
    create_kubeconfig(cluster)
    kubeconfig_k8s = execute_kubectl_cmd('version')['serverVersion']
    kubectl_version = '.'.join(kubeconfig_k8s['gitVersion'].split(".")[:2])

    # From rancher version v2.6.x+, k8s versions 1.21+ istio, monitoring charts are deprecated
    # Verifying if the provided cluster version is below v1.21 else the script raises error.
    # Get system project client as the v1 apps are installed in system project
    # Get cluster client for the istio and monitoring installation
    if kubectl_version >= 'v1.21':
        raise AssertionError("For istio and monitoring v1 charts, kubernetes version should be less than v1.21")
    projects = client.list_project(name='System', clusterId=cluster.id)
    system_project_client = get_system_client(USER_TOKEN, projects=projects)
    if len(projects.data) == 0:
        raise AssertionError(
            "System project not found in the cluster " + cluster.name)
    p = projects.data[0]
    cluster = client.reload(cluster)
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)

    istio_enabled = enable_monitoring_istio(client, c_client, p_client, cluster, p,
                                            additional_answers=additional_answers)
    istio_ns = istio_enabled["ns"].name
    monitoring_ns = istio_enabled["monitoring_ns"]

    def fin():
        client = get_client_for_token(USER_TOKEN, url=CATTLE_API_URL)
        # delete istio app
        app = p_client.list_app(name='cluster-istio').data[0]
        p_client.delete(app)
        validate_app_deletion(p_client, app.id)
        # delete istio ns
        p_client.delete(istio_ns)
        # disable cluster monitoring
        cluster = client.list_cluster(name=CLUSTER_NAME).data[0]
        c = client.reload(cluster)
        if c["enableClusterMonitoring"] is True:
            client.action(c, "disableMonitoring")
        if CLUSTER_CLEANUP:
            cluster_cleanup(client, cluster)
    request.addfinalizer(fin)
