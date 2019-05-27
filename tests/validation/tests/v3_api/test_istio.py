import os

import pytest
import time

from .common import *  # NOQA

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None}
ISTIO_PATH = os.path.join(
    os.path.dirname(os.path.realpath(__file__)), "resource/istio_files")
ISTIO_CATALOG = os.environ.get('RANCHER_ISTIO_CATALOG', "system-library")
ISTIO_TEMPLATE = os.environ.get('RANCHER_ISTIO_TEMPLATE', "rancher-istio")
ISTIO_VERSION = os.environ.get('RANCHER_ISTIO_VERSION', "1.1.5-rancher1")
ISTIO_INGRESSGATEWAY_NODEPORT = os.environ.get(
    'RANCHER_ISTIO_INGRESSGATEWAY_NODEPORT', 31380)
ISTIO_CRD_COUNT = "53"
ISTIO_BOOKINFO_QUERY_RESULT = "<title>Simple Bookstore App</title>"


def test_add_custom_istio_catalog():
    p_client = namespace["p_client"]
    c_client = namespace["c_client"]
    project = namespace["project"]
    ns = namespace["ns"]
    name = "istio"

    app_ns = namespace["app_ns"]
    istio_app_client = namespace["istio_app_client"]

    # run dynamic admission webhook prerequisites check
    verify_admission_webhook()

    # add label istio-injection=true to the istio app namespace
    add_istio_label_to_ns(c_client, app_ns)

    # deploy the istio catalog app and verify its status
    app = create_and_verify_istio_app(p_client, ns, project, name)

    # verify istio CRDs resources
    verify_istio_crds_count()

    # implement health checking of istio services
    create_health_check(p_client, ns)

    # test istio bookinfo sample app
    create_and_test_booinfo_services(istio_app_client, app_ns, 30)

    # test istio ingressgateway with bookinfo gateway
    create_and_test_booinfo_gateway(istio_app_client, ns, app_ns, 30)

    p_client.delete(app)


def create_and_test_booinfo_gateway(
        client, ns, app_ns, timeout=DEFAULT_TIMEOUT):
    # create bookinfo gateway
    book_info_file_path = ISTIO_PATH + '/bookinfo-gateway.yaml'
    execute_kubectl_cmd('apply -f ' + book_info_file_path
                        + ' -n ' + app_ns.name, False)
    gateways = execute_kubectl_cmd('get gateway -n' + app_ns.name, True)
    assert len(gateways['items']) == 1

    # test access bookinfo via istio ingress gateway
    istio_gateway_wl = client.by_id_workload('deployment:' + ns.name
                                             + ':istio-ingressgateway')
    assert istio_gateway_wl is not None
    endpoints = istio_gateway_wl['publicEndpoints'][0]
    gateway_url = endpoints['addresses'][0] + ':' + str(endpoints['port'])
    curl_cmd = 'curl -s http://' + gateway_url \
               + '/productpage | grep -o "<title>.*</title>"'
    result = run_command(curl_cmd)
    start = time.time()
    while result.rstrip() != ISTIO_BOOKINFO_QUERY_RESULT:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out and failed to get bookinfo gateway ready")
        time.sleep(.5)
        result = run_command(curl_cmd)
        assert result.rstrip() == ISTIO_BOOKINFO_QUERY_RESULT
    return result


def create_and_test_booinfo_services(p_client, ns, timeout=DEFAULT_TIMEOUT):
    book_info_file_path = ISTIO_PATH + '/bookinfo.yaml'
    execute_kubectl_cmd('apply -f ' + book_info_file_path + ' -n '
                        + ns.name, False)
    result = execute_kubectl_cmd('get deployment -n' + ns.name, True)

    for wl in result['items']:
        wl = wait_for_wl_by_id_to_active(p_client, wl, 60)
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


def wait_for_wl_by_id_to_active(client, wl, timeout=DEFAULT_TIMEOUT):
    start = time.time()
    workload = client.by_id_workload('deployment:'
                                     + wl['metadata']['namespace']
                                     + ':' + wl['metadata']['name'])
    while workload.state != "active":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        workload = client.by_id_workload('deployment:'
                                         + wl['metadata']['namespace']
                                         + ':' + wl['metadata']['name'])
    return workload


def create_and_verify_istio_app(p_client, ns, project, name):
    answers = {
        "enableCRDs": "true",
        "gateways.istio-ingressgateway.type": "NodePort",
        "gateways.istio-ingressgateway.ports[0].nodePort":
            ISTIO_INGRESSGATEWAY_NODEPORT,
        "gateways.istio-ingressgateway.ports[0].port": 80,
        "gateways.istio-ingressgateway.ports[0].targetPort": 80,
        "gateways.istio-ingressgateway.ports[0].name": "http2",
    }
    external_id = "catalog://?catalog=" + ISTIO_CATALOG + \
                  "&template=" + ISTIO_TEMPLATE + \
                  "&version=" + ISTIO_VERSION + \
                  "&namespace=cattle-global-data"
    print("creating istio catalog app")
    app = p_client.create_app(
        name=name,
        externalId=external_id,
        targetNamespace=ns.name,
        projectId=project.id,
        answers=answers
    )
    print("verify istio app installed condition")
    wait_for_condition(p_client, app, check_condition('Installed', 'True'), 120)
    print("verify istio app deployment condition")
    wait_for_condition(p_client, app, check_condition('Deployed', 'True'), 600)
    return app


def verify_admission_webhook():
    has_admission_webhook = execute_kubectl_cmd(
        'api-versions | grep admissionregistration', False)
    if len(has_admission_webhook) == 0:
        raise AssertionError(
            "MutatingAdmissionWebhook and ValidatingAdmissionWebhook plugins "
            "are not listed in the kube-apiserver --enable-admission-plugins")


def create_health_check(p_client, ns):
    workload = create_workload(p_client, ns)
    workload = p_client.reload(workload)

    # wait 15 seconds to run livenessProbe check
    time.sleep(15)
    results = validate_workload(p_client, workload, "deployment", ns.name)
    assert results[0]['status']['containerStatuses'][0]['restartCount'] == 0
    p_client.delete(workload)


def add_istio_label_to_ns(c_client, ns):
    labels = {
        "istio-injection": "enabled"
    }
    ns = c_client.update_by_id_namespace(ns.id, labels=labels)
    return ns


def verify_istio_crds_count():
    get_crd_count = execute_kubectl_cmd(
        'get crds | grep "istio.io" | wc -l', False)
    if len(get_crd_count) == 0:
        raise AssertionError(
            "get istio CRD count return 0, expected to be " + ISTIO_CRD_COUNT)
    assert get_crd_count.rstrip() == ISTIO_CRD_COUNT


def create_workload(p_client, ns):
    workload_name = random_test_name("liveness")
    con = [{
        "command": [
            "/bin/sh",
            "-c",
            "touch /tmp/healthy; sleep 3600",
        ],
        "image":"k8s.gcr.io/busybox",
        "imagePullPolicy":"Always",
        "livenessProbe":{
            "command": [
                "cat",
                "/tmp/healthy",
            ],
        },
        "name":"liveness",
    }]
    workload = p_client.create_workload(name=workload_name,
                                        containers=con,
                                        namespaceId=ns.id)
    wait_for_wl_to_active(p_client, workload, timeout=45)
    return workload


def check_condition(condition_type, status):
    def _find_condition(resource):
        if not hasattr(resource, "conditions"):
            return False

        if resource.conditions is None:
            return False

        for condition in resource.conditions:
            if condition.type == condition_type and condition.status == status:
                return True
        return False

    return _find_condition


def wait_for_template_to_be_deleted(client, name, timeout=45):
    found = False
    start = time.time()
    interval = 0.5
    while not found:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for templates")
        templates = client.list_template(catalogId=name)
        if len(templates) == 0:
            found = True
        time.sleep(interval)
        interval *= 2


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_admin_client_and_cluster()
    create_kubeconfig(cluster)
    projects = client.list_project(name='System', clusterId=cluster.id)
    if len(projects.data) == 0:
        raise AssertionError(
            "System project not found in the cluster " + cluster.Name)
    p = projects.data[0]
    p_client = get_project_client_for_token(p, ADMIN_TOKEN)
    c_client = get_cluster_client_for_token(cluster, ADMIN_TOKEN)

    # create istio-system namespace
    ns = create_ns(c_client, cluster, p, 'istio-system')

    # create istio app ns and client
    app_ns = create_ns(c_client, cluster, p, random_test_name('istio-app'))
    istio_app_client = get_project_client_for_token(p, ADMIN_TOKEN)

    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["c_client"] = c_client
    namespace["app_ns"] = app_ns
    namespace["istio_app_client"] = istio_app_client

    def fin():
        client = get_admin_client()
        client.delete(namespace["ns"])
        client.delete(namespace["app_ns"])
    request.addfinalizer(fin)
