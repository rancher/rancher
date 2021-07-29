import re
import os

import pytest
from .common import create_kubeconfig
from .common import get_cluster_client_for_token
from .common import get_project_client_for_token
from .common import get_user_client
from .common import get_cluster_by_name
from .common import USER_TOKEN
from .test_rke_cluster_provisioning import create_and_validate_custom_host
from .common import validate_app_deletion
from .test_istio import enable_monitoring_istio
from .test_istio import get_system_client


user_token = get_system_client(USER_TOKEN)
istio_ns = ""
monitoring_ns = ""
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
    data_ = user_token.list_workload(namespaceId=ns_).data
    container_images = list(set([c.image for wks in data_ for c in wks.containers if len(data_) >= 0 if c.image]))

    wk_image = dict()
    # splitting them on ":" to separate them to image and version
    for image in container_images:
        if image.startswith('rancher/'):
            img, ver = image.replace('rancher/', '').split(":")
            # Add 'v' to version if missing
            wk_image[img] = ver if ver.startswith("v") else 'v' + ver
    return wk_image


def print_wk(wk, expected_images):
    for w, ver in wk.items():
        if w in expected_images.keys():
            try:
                assert ver == expected_images[w]
                print( "\n\nVersion of the image {} is {}".format(w, ver)+"\n\n" )
            except AssertionError:
                print('\033[2;31;43 ERROR')
                print("\n\nVersion of the image {} is {} but the expected version is {}".format(w, ver,expected_images[w])+"\n\n")
                continue


@pytest.mark.skipif(False, reason="Only testing monitoring")
def test_istio():
    if istio_ns == '':
        raise Exception("No namespace is detected uh-oh!")
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


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    global user_token
    global istio_ns
    global monitoring_ns

    client = get_user_client()

    cluster_name = os.environ.get('RANCHER_CLUSTER_NAME')

    if cluster_name == "":
        node_roles = [["controlplane"], ["etcd"],
                      ["worker"], ["worker"], ["worker"], ["worker"]]
        cluster,nodes = create_and_validate_custom_host(node_roles)
    else:
        cluster = get_cluster_by_name(client,cluster_name)

    create_kubeconfig(cluster)
    projects = client.list_project(name='System', clusterId=cluster.id)
    if len(projects.data) == 0:
        raise AssertionError(
            "System project not found in the cluster " + cluster.name)
    p = projects.data[0]
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)

    istio_enabled = enable_monitoring_istio(client, c_client, p_client, cluster, p,
                                            additional_answers=additional_answers)
    istio_ns = istio_enabled["ns"].name
    monitoring_ns = istio_enabled["monitoring_ns"]

    def fin():
        client = get_user_client()
        # delete istio app
        app = p_client.list_app(name='cluster-istio').data[0]
        p_client.delete(app)
        validate_app_deletion(p_client, app.id)
        # delete istio ns
        p_client.delete(istio_ns)
        # disable cluster monitoring
        c = client.reload(cluster)
        if c["enableClusterMonitoring"] is True:
            client.action(c, "disableMonitoring")
    request.addfinalizer(fin)