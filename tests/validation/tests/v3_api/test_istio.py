import copy
import os

import pytest
import time

from subprocess import CalledProcessError

from rancher import ApiError

from .common import check_condition
from .common import CLUSTER_MEMBER
from .common import CLUSTER_OWNER
from .common import create_kubeconfig
from .common import create_project_and_ns
from .common import create_ns
from .common import DEFAULT_TIMEOUT
from .common import execute_kubectl_cmd
from .common import get_cluster_client_for_token
from .common import get_project_client_for_token
from .common import get_user_client
from .common import get_user_client_and_cluster
from .common import if_test_rbac
from .common import PROJECT_MEMBER
from .common import PROJECT_OWNER
from .common import PROJECT_READ_ONLY
from .common import random_test_name
from .common import rbac_get_user_token_by_role
from .common import requests
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

DEFAULT_ANSWERS = {
    "enableCRDs": "true",
    "gateways.enabled": "true",
    "gateways.istio-ingressgateway.type": "NodePort",
    "gateways.istio-ingressgateway.ports[0].nodePort":
        ISTIO_INGRESSGATEWAY_NODEPORT,
    "gateways.istio-ingressgateway.ports[0].port": 80,
    "gateways.istio-ingressgateway.ports[0].targetPort": 80,
    "gateways.istio-ingressgateway.ports[0].name": "http2",
    "global.monitoring.type": "cluster-monitoring"}
namespace = {"app_client": None, "app_ns": None, "gateway_url": None,
             "system_ns": None, "system_project": None,
             "istio_version": None, "istio_app": None}


def test_istio_resources():
    app_client = namespace["app_client"]
    app_ns = namespace["app_ns"]
    gateway_url = namespace["gateway_url"]

    create_and_test_bookinfo_services(app_client, app_ns)
    create_bookinfo_virtual_service(app_client, app_ns)
    create_and_test_bookinfo_gateway(app_client, app_ns, gateway_url)
    create_and_test_bookinfo_routing(app_client, app_ns, gateway_url)


@if_test_rbac
def test_rbac_cluster_owner_istio_metrics_allow_all(allow_all_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_owner = rbac_get_user_token_by_role(CLUSTER_OWNER)
    validate_access(kiali_url, cluster_owner)
    validate_access(tracing_url, cluster_owner)


@if_test_rbac
def test_rbac_cluster_owner_istio_monitoring_allow_all(allow_all_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_owner = rbac_get_user_token_by_role(CLUSTER_OWNER)
    validate_access(grafana_url, cluster_owner)
    validate_access(prometheus_url, cluster_owner)


@if_test_rbac
def test_rbac_cluster_member_istio_metrics_allow_all(allow_all_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    validate_access(kiali_url, cluster_member)
    validate_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_cluster_member_istio_monitoring_allow_all(allow_all_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_project_owner_istio_metrics_allow_all(allow_all_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_OWNER)
    validate_access(kiali_url, cluster_member)
    validate_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_project_owner_istio_monitoring_allow_all(allow_all_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_OWNER)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_project_member_istio_metrics_allow_all(allow_all_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_access(kiali_url, cluster_member)
    validate_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_project_member_istio_monitoring_allow_all(allow_all_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_project_read_istio_metrics_allow_all(allow_all_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    validate_access(kiali_url, cluster_member)
    validate_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_project_read_istio_monitoring_allow_all(allow_all_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_cluster_owner_istio_metrics_allow_none(default_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_owner = rbac_get_user_token_by_role(CLUSTER_OWNER)
    validate_access(kiali_url, cluster_owner)
    validate_access(tracing_url, cluster_owner)


@if_test_rbac
def test_rbac_cluster_owner_istio_monitoring_allow_none(default_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_owner = rbac_get_user_token_by_role(CLUSTER_OWNER)
    validate_access(grafana_url, cluster_owner)
    validate_access(prometheus_url, cluster_owner)


@if_test_rbac
def test_rbac_cluster_member_istio_metrics_allow_none(default_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    validate_no_access(kiali_url, cluster_member)
    validate_no_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_cluster_member_istio_monitoring_allow_none(default_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_project_owner_istio_metrics_allow_none(default_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_OWNER)
    validate_no_access(kiali_url, cluster_member)
    validate_no_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_project_owner_istio_monitoring_allow_none(default_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_OWNER)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_project_member_istio_metrics_allow_none(default_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_no_access(kiali_url, cluster_member)
    validate_no_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_project_member_istio_monitoring_allow_none(default_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_project_read_istio_metrics_allow_none(default_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    validate_no_access(kiali_url, cluster_member)
    validate_no_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_project_read_istio_monitoring_allow_none(default_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_cluster_member_istio_update():
    user = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    with pytest.raises(ApiError) as e:
        update_istio_app({"FOO": "BAR"}, user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_cluster_member_istio_disable():
    user = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    with pytest.raises(ApiError) as e:
        delete_istio_app(user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_project_owner_istio_disable():
    user = rbac_get_user_token_by_role(PROJECT_OWNER)
    with pytest.raises(ApiError) as e:
        delete_istio_app(user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_project_owner_istio_update():
    user = rbac_get_user_token_by_role(PROJECT_OWNER)
    with pytest.raises(ApiError) as e:
        update_istio_app({"FOO": "BAR"}, user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_project_member_istio_update():
    user = rbac_get_user_token_by_role(PROJECT_MEMBER)
    with pytest.raises(ApiError) as e:
        update_istio_app({"FOO": "BAR"}, user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_project_member_istio_disable():
    user = rbac_get_user_token_by_role(PROJECT_MEMBER)
    with pytest.raises(ApiError) as e:
        delete_istio_app(user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_project_read_istio_update():
    user = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    with pytest.raises(ApiError) as e:
        update_istio_app({"FOO": "BAR"}, user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_project_read_istio_disable():
    user = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    with pytest.raises(ApiError) as e:
        delete_istio_app(user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


def validate_access(url, user):
    headers = {'Authorization': 'Bearer ' + user}
    response = requests.get(headers=headers, url=url, verify=False)

    assert response.ok
    return response


def validate_no_access(url, user):
    headers = {'Authorization': 'Bearer ' + user}
    response = requests.get(headers=headers, url=url, verify=False)

    assert not response.ok
    return response


def update_istio_app(answers, user):
    p_client = get_system_client(user)
    updated_answers = copy.deepcopy(DEFAULT_ANSWERS)
    updated_answers.update(answers)
    external_id = "catalog://?catalog=system-library" + \
                  "&template=rancher-istio" + \
                  "&version=" + namespace["istio_version"]
    namespace["istio_app"] = p_client.update(
        obj=namespace["istio_app"],
        externalId=external_id,
        targetNamespace=namespace["system_ns"].name,
        projectId=namespace["system_project"].id,
        answers=updated_answers)
    verify_istio_app_ready(p_client, namespace["istio_app"], 120, 120)


def create_and_verify_istio_app(p_client, ns, project, version):
    external_id = "catalog://?catalog=system-library" + \
                  "&template=rancher-istio" + \
                  "&version=" + version
    print("creating istio catalog app")
    app = p_client.create_app(
        name="cluster-istio",
        externalId=external_id,
        targetNamespace=ns.name,
        projectId=project.id,
        answers=DEFAULT_ANSWERS
    )
    verify_istio_app_ready(p_client, app, 120, 600)
    return app


def delete_istio_app(user):
    p_client = get_system_client(user)
    p_client.delete(namespace["istio_app"])


def verify_istio_app_ready(p_client, app, install_timeout, deploy_timeout):
    print("Verify istio app installed condition")
    wait_for_condition(
        p_client, app, check_condition('Installed', 'True'), install_timeout)

    print("Verify istio app deployment condition")
    wait_for_condition(
        p_client, app, check_condition('Deployed', 'True'), deploy_timeout)


def get_urls():
    _, cluster = get_user_client_and_cluster()
    if ISTIO_VERSION == "0.1.0" or ISTIO_VERSION == "0.1.1":
        kiali_url = os.environ.get('CATTLE_TEST_URL', "") + \
            "/k8s/clusters/" + cluster.id + \
            "/api/v1/namespaces/istio-system/services/" \
            "http:kiali-http:80/proxy/"
    else:
        kiali_url = os.environ.get('CATTLE_TEST_URL', "") + \
            "/k8s/clusters/" + cluster.id + \
            "/api/v1/namespaces/istio-system/services/" \
            "http:kiali:20001/proxy/"
    tracing_url = os.environ.get('CATTLE_TEST_URL', "") + \
        "/k8s/clusters/" + cluster.id + \
        "/api/v1/namespaces/istio-system/services/" \
        "http:tracing:80/proxy/jaeger/search"
    grafana_url = os.environ.get('CATTLE_TEST_URL', "") + \
        "/k8s/clusters/" + cluster.id + \
        "/api/v1/namespaces/cattle-prometheus/services/" \
        "http:access-grafana:80/proxy/dashboards/"
    prometheus_url = os.environ.get('CATTLE_TEST_URL', "") + \
        "/k8s/clusters/" + cluster.id + \
        "/api/v1/namespaces/cattle-prometheus/services/" \
        "http:access-prometheus:80/proxy/"
    return kiali_url, tracing_url, grafana_url, prometheus_url


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
    try:
        result = execute_kubectl_cmd(
            'exec -it -n ' + ns.name + ' ' + rating_pod_name
            + ' -c ratings -- curl productpage:9080/productpage'
            + ' | grep -o "<title>.*</title>"', False)
    except CalledProcessError:
        result = None

    start = time.time()
    while result is None or result.rstrip() != ISTIO_BOOKINFO_QUERY_RESULT:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out and failed to get bookinfo service ready")
        time.sleep(.5)
        try:
            result = execute_kubectl_cmd(
                'exec -it -n ' + ns.name + ' ' + rating_pod_name
                + ' -c ratings -- curl productpage:9080/productpage'
                + ' | grep -o "<title>.*</title>"', False)
        except CalledProcessError:
            result = None
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


def get_system_client(user):
    # Gets client and cluster using USER_TOKEN, who is a CLUSTER_OWNER
    client, cluster = get_user_client_and_cluster()
    projects = client.list_project(name='System', clusterId=cluster.id)
    if len(projects.data) == 0:
        raise AssertionError(
            "System project not found in the cluster " + cluster.Name)
    p = projects.data[0]
    return get_project_client_for_token(p, user)


@pytest.fixture(scope='function')
def default_access(request):
    answers = {
        "kiali.enabled": "true",
        "tracing.enabled": "true",
    }
    update_istio_app(answers, USER_TOKEN)


@pytest.fixture(scope='function')
def allow_all_access(request):
    answers = {
        "global.members[0].kind": "Group",
        "global.members[0].name": "system:authenticated",
        "kiali.enabled": "true",
        "tracing.enabled": "true",
    }
    update_istio_app(answers, USER_TOKEN)


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

    istio_versions = list(client.list_template(
        id=ISTIO_TEMPLATE_ID).data[0].versionLinks.keys())
    istio_version = istio_versions[len(istio_versions) - 1]

    if ISTIO_VERSION != "":
        istio_version = ISTIO_VERSION
    answers = {"global.rancher.clusterId": p.clusterId}
    DEFAULT_ANSWERS.update(answers)

    if cluster["enableClusterMonitoring"] is False:
        client.action(cluster, "enableMonitoring",
                      answers=C_MONITORING_ANSWERS)

    if cluster["istioEnabled"] is False:
        verify_admission_webhook()

        ns = create_ns(c_client, cluster, p, 'istio-system')
        app = create_and_verify_istio_app(p_client, ns, p, istio_version)
    else:
        app = p_client.list_app(name='cluster-istio').data[0]
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
    namespace["system_ns"] = ns
    namespace["system_project"] = p
    namespace["istio_version"] = istio_version
    namespace["istio_app"] = app

    def fin():
        client = get_user_client()
        client.delete(istio_project)
    request.addfinalizer(fin)
