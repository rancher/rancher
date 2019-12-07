import os

import pytest
import time

from subprocess import CalledProcessError

from .common import check_condition
from .common import create_kubeconfig
from .common import create_project_and_ns
from .common import create_ns
from .common import DEFAULT_TIMEOUT
from .common import execute_kubectl_cmd
from .common import get_cluster_client_for_token
from .common import get_project_client_for_token
from .common import get_user_client
from .common import get_user_client_and_cluster
from .common import random_test_name
from .common import run_command as run_command_common
from .common import USER_TOKEN
from .common import wait_for_condition
from .common import wait_for_pod_to_running
from .common import wait_for_pods_in_workload
from .common import wait_for_wl_to_active

from .test_monitoring import C_MONITORING_ANSWERS

ISTIO_PATH = os.path.join(
    os.path.dirname(os.path.realpath(__file__)), "resource/istio")
ISTIO_TEMPLATE_ID = "cattle-global-data:system-library-rancher-istio"
ISTIO_VERSION = os.environ.get('RANCHER_ISTIO_VERSION', "")
ISTIO_INGRESSGATEWAY_NODEPORT = os.environ.get(
    'RANCHER_ISTIO_INGRESSGATEWAY_NODEPORT', 31380)
ISTIO_BOOKINFO_QUERY_RESULT = "<title>Simple Bookstore App</title>"

namespace = {"app_client": None, "app_ns": None, "gateway_url": None}


def test_istio_resources():
    app_client = namespace["app_client"]
    app_ns = namespace["app_ns"]
    gateway_url = namespace["gateway_url"]

    create_and_test_bookinfo_services(app_client, app_ns)
    create_bookinfo_virtual_service(app_client, app_ns)
    create_and_test_bookinfo_gateway(app_client, app_ns, gateway_url)
    create_and_test_bookinfo_routing(app_client, app_ns, gateway_url)


def create_and_verify_istio_app(p_client, ns, project, version):
    answers = {
        "enableCRDs": "true",
        "gateways.enabled": "true",
        "gateways.istio-ingressgateway.type": "NodePort",
        "gateways.istio-ingressgateway.ports[0].nodePort":
            ISTIO_INGRESSGATEWAY_NODEPORT,
        "gateways.istio-ingressgateway.ports[0].port": 80,
        "gateways.istio-ingressgateway.ports[0].targetPort": 80,
        "gateways.istio-ingressgateway.ports[0].name": "http2",
    }
    external_id = "catalog://?catalog=system-library" + \
                  "&template=rancher-istio" + \
                  "&version=" + version
    print("creating istio catalog app")
    app = p_client.create_app(
        name="cluster-istio",
        externalId=external_id,
        targetNamespace=ns.name,
        projectId=project.id,
        answers=answers
    )
    print("Verify istio app installed condition")
    wait_for_condition(
        p_client, app, check_condition('Installed', 'True'), 120)

    print("Verify istio app deployment condition")
    wait_for_condition(
        p_client, app, check_condition('Deployed', 'True'), 600)
    return app


def verify_admission_webhook():
    has_admission_webhook = execute_kubectl_cmd(
        'api-versions | grep admissionregistration', False)
    if len(has_admission_webhook) == 0:
        raise AssertionError(
            "MutatingAdmissionWebhook and ValidatingAdmissionWebhook plugins "
            "are not listed in the kube-apiserver --enable-admission-plugins")


def add_istio_label_to_ns(c_client, ns):
    labels = {
        "istio-injection": "enabled"
    }
    ns = c_client.update_by_id_namespace(ns.id, labels=labels)
    return ns


def create_and_test_bookinfo_services(p_client, ns, timeout=DEFAULT_TIMEOUT):
    book_info_file_path = ISTIO_PATH + '/bookinfo.yaml'
    execute_kubectl_cmd('apply -f ' + book_info_file_path + ' -n '
                        + ns.name, False)
    result = execute_kubectl_cmd('get deployment -n ' + ns.name, True)

    for deployment in result['items']:
        wl = p_client.list_workload(id='deployment:'
                                    + deployment['metadata']['namespace']
                                    + ':'
                                    + deployment['metadata']['name']).data[0]
        wl = wait_for_wl_to_active(p_client, wl, 60)
        wl_pods = wait_for_pods_in_workload(p_client, wl, 1)
        wait_for_pod_to_running(p_client, wl_pods[0])

    rating_pod = execute_kubectl_cmd('get pod -l app=ratings -n' + ns.name)
    assert len(rating_pod['items']) == 1

    rating_pod_name = rating_pod['items'][0]['metadata']['name']
    result = execute_kubectl_cmd(
        'exec -it -n ' + ns.name + ' ' + rating_pod_name
        + ' -c ratings -- curl productpage:9080/productpage'
        + ' | grep -o "<title>.*</title>"', False)

    start = time.time()
    while result.rstrip() != ISTIO_BOOKINFO_QUERY_RESULT:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out and failed to get bookinfo service ready")
        time.sleep(.5)
        result = execute_kubectl_cmd(
            'exec -it -n ' + ns.name + ' ' + rating_pod_name
            + ' -c ratings -- curl productpage:9080/productpage'
            + ' | grep -o "<title>.*</title>"', False)
    assert result.rstrip() == ISTIO_BOOKINFO_QUERY_RESULT
    return result


def create_and_test_bookinfo_gateway(app_client, namespace,
                                     gateway_url, timeout=DEFAULT_TIMEOUT):
    servers = [{
        "hosts": ["*"],
        "port": {
            "number": "80",
            "protocol": "HTTP",
            "name": "http"
        }
    }]
    selector = {"istio": "ingressgateway"}
    app_client.create_gateway(name="bookinfo-gateway",
                              namespaceId=namespace.id,
                              selector=selector,
                              servers=servers)

    gateways = execute_kubectl_cmd('get gateway -n' + namespace.name, True)
    assert len(gateways['items']) == 1

    curl_cmd = 'curl -s http://' + gateway_url \
               + '/productpage | grep -o "<title>.*</title>"'

    result = run_command(curl_cmd)

    start = time.time()
    while result is None or result.rstrip() != ISTIO_BOOKINFO_QUERY_RESULT:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out and failed to get bookinfo gateway ready")
        time.sleep(.5)
        result = run_command(curl_cmd)
    assert result.rstrip() == ISTIO_BOOKINFO_QUERY_RESULT

    return result


def create_bookinfo_virtual_service(app_client, namespace):
    http = [{
        "route": [{
                "destination": {
                    "host": "productpage",
                    "port": {"number": 9080}
                },
                "weight": 100,
                "portNumberOrName": "9080"
                }],
        "match": [
            {"uri": {"exact": "/productpage"}},
            {"uri": {"exact": "/login"}},
            {"uri": {"exact": "/logout"}},
            {"uri": {"prefix": "/api/v1/products"}}
        ]
      }]

    app_client.create_virtual_service(name="bookinfo",
                                      namespaceId=namespace.id,
                                      gateways=["bookinfo-gateway"],
                                      http=http,
                                      hosts=["*"])


def create_bookinfo_destination_rules(app_client, namespace):
    subsets = [
        {
            "name": "v1",
            "labels": {
                "version": "v1"
            }
        },
        {
            "name": "v2",
            "labels": {
                "version": "v2"
            }
        },
        {
            "name": "v3",
            "labels": {
                "version": "v3"
            }
        }
    ]
    app_client.create_destination_rule(namespaceId=namespace.id,
                                       name="reviews",
                                       host="reviews",
                                       subsets=subsets)


def create_and_test_bookinfo_routing(app_client, namespace,
                                     gateway_url, timeout=30):
    http = [{
        "route": [{
                "destination": {
                    "subset": "v3",
                    "host": "reviews",
                    "port": {"number": 9080}
                },
                "weight": 100,
                "portNumberOrName": "9080"
                }]
      }]

    create_bookinfo_destination_rules(app_client, namespace)
    app_client.create_virtual_service(name="reviews",
                                      namespaceId=namespace.id,
                                      http=http,
                                      hosts=["reviews"])

    curl_cmd = 'curl -s http://' + gateway_url \
               + '/productpage | grep -o "glyphicon-star"'

    result = run_command(curl_cmd)

    start = time.time()
    while result is None or "glyphicon-star" not in result:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out and failed to get correct reviews version")
        time.sleep(.5)
        result = run_command(curl_cmd)
    assert "glyphicon-star" in result

    return result


# if grep returns no output, subprocess.check_output raises CalledProcessError
def run_command(command):
    try:
        return run_command_common(command)
    except CalledProcessError:
        return None


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    projects = client.list_project(name='System', clusterId=cluster.id)
    if len(projects.data) == 0:
        raise AssertionError(
            "System project not found in the cluster " + cluster.Name)
    p = projects.data[0]
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)

    if cluster["enableClusterMonitoring"] is False:
        client.action(cluster, "enableMonitoring",
                      answers=C_MONITORING_ANSWERS)

    if cluster["istioEnabled"] is False:
        verify_admission_webhook()

        istio_versions = list(client.list_template(
            id=ISTIO_TEMPLATE_ID).data[0].versionLinks.keys())
        istio_version = istio_versions[len(istio_versions) - 1]

        if ISTIO_VERSION != "":
            istio_version = ISTIO_VERSION

        ns = create_ns(c_client, cluster, p, 'istio-system')
        create_and_verify_istio_app(p_client, ns, p, istio_version)
    else:
        ns = c_client.list_namespace(name='istio-system').data[0]

    istio_project, app_ns = create_project_and_ns(
        USER_TOKEN, cluster,
        random_test_name("istio-app"),
        random_test_name("istio-app-ns"))
    add_istio_label_to_ns(c_client, app_ns)

    app_client = get_project_client_for_token(istio_project, USER_TOKEN)

    istio_gateway_wl = p_client.by_id_workload('deployment:' +
                                               ns.name +
                                               ':istio-ingressgateway')
    assert istio_gateway_wl is not None
    endpoints = istio_gateway_wl['publicEndpoints'][0]
    gateway_url = endpoints['addresses'][0] + ':' + str(endpoints['port'])

    namespace["gateway_url"] = gateway_url
    namespace["app_ns"] = app_ns
    namespace["app_client"] = app_client

    def fin():
        client = get_user_client()
        client.delete(istio_project)
    request.addfinalizer(fin)
