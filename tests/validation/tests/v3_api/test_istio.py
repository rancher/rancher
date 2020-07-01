import copy
import os
import re

import pytest
import time

from subprocess import CalledProcessError

from rancher import ApiError

from .test_auth import enable_ad, load_setup_data
from .common import add_role_to_user
from .common import auth_get_user_token
from .common import auth_resource_cleanup
from .common import AUTH_PROVIDER
from .common import AUTH_USER_PASSWORD
from .common import apply_crd
from .common import check_condition
from .common import compare_versions
from .common import CLUSTER_MEMBER
from .common import CLUSTER_OWNER
from .common import create_kubeconfig
from .common import create_project_and_ns
from .common import create_ns
from .common import DEFAULT_TIMEOUT
from .common import delete_crd
from .common import execute_kubectl_cmd
from .common import get_a_group_and_a_user_not_in_it
from .common import get_admin_client
from .common import get_client_for_token
from .common import get_cluster_client_for_token
from .common import get_crd
from .common import get_group_principal_id
from .common import get_project_client_for_token
from .common import get_user_by_group
from .common import get_user_client
from .common import get_user_client_and_cluster
from .common import if_test_group_rbac
from .common import if_test_rbac
from .common import login_as_auth_user
from .common import NESTED_GROUP_ENABLED
from .common import PROJECT_MEMBER
from .common import PROJECT_OWNER
from .common import PROJECT_READ_ONLY
from .common import random_test_name
from .common import rbac_get_kubeconfig_by_role
from .common import rbac_get_namespace
from .common import rbac_get_user_token_by_role
from .common import requests
from .common import run_command as run_command_common
from .common import ADMIN_TOKEN
from .common import USER_TOKEN
from .common import validate_all_workload_image_from_rancher
from .common import validate_app_deletion
from .common import wait_for_condition
from .common import wait_for_pod_to_running
from .common import wait_for_pods_in_workload
from .common import wait_for_wl_to_active

from .test_monitoring import C_MONITORING_ANSWERS

ISTIO_PATH = os.path.join(
    os.path.dirname(os.path.realpath(__file__)), "resource/istio")
ISTIO_CRD_PATH = os.path.join(ISTIO_PATH, "crds")
ISTIO_TEMPLATE_ID = "cattle-global-data:system-library-rancher-istio"
ISTIO_VERSION = os.environ.get('RANCHER_ISTIO_VERSION', "")
ISTIO_INGRESSGATEWAY_NODEPORT = os.environ.get(
    'RANCHER_ISTIO_INGRESSGATEWAY_NODEPORT', 31380)
ISTIO_BOOKINFO_QUERY_RESULT = "<title>Simple Bookstore App</title>"
ISTIO_EXTERNAL_ID = "catalog://?catalog=system-library" \
                    "&template=rancher-istio&version="

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


crd_test_data = [
    ("policy.authentication.istio.io", "authenticationpolicy.yaml"),
    # ("adapter.config.istio.io", "adapter.yaml"),
    # ABOVE FAILS in current state: Rancher v2.3.5
    # ("attributemanifest.config.istio.io", "attributemanifest.yaml"),
    # ABOVE FAILS in current state: Rancher v2.3.5
    ("handler.config.istio.io", "handler.yaml"),
    # ("httpapispecbinding.config.istio.io", "httpapispecbinding.yaml"),
    # ABOVE FAILS in current state: Rancher v2.3.5
    # ("httpapispec.config.istio.io", "httpapispec.yaml"),
    # ABOVE FAILS in current state: Rancher v2.3.5
    # ("instance.config.istio.io", "instance.yaml"),
    # ABOVE FAILS in current state: Rancher v2.3.5
    ("quotaspecbinding.config.istio.io", "quotaspecbinding.yaml"),
    ("quotaspec.config.istio.io", "quotaspec.yaml"),
    ("rule.config.istio.io", "rule.yaml"),
    # ("template.config.istio.io", "template.yaml"),
    # ABOVE FAILS in current state: Rancher v2.3.5
    ("destinationrule.networking.istio.io", "destinationrule.yaml"),
    ("envoyfilter.networking.istio.io", "envoyfilter.yaml"),
    ("gateway.networking.istio.io", "gateway.yaml"),
    ("serviceentry.networking.istio.io", "serviceentry.yaml"),
    ("sidecar.networking.istio.io", "sidecar.yaml"),
    ("virtualservice.networking.istio.io", "virtualservice.yaml"),
    ("rbacconfig.rbac.istio.io", "rbacconfig.yaml"),
    ("servicerolebinding.rbac.istio.io", "servicerolebinding.yaml"),
    ("servicerole.rbac.istio.io", "servicerole.yaml"),
    ("authorizationpolicy.security.istio.io", "authorizationpolicy.yaml"),
    # ("certificate.certmanager.k8s.io", "certificate.yaml"),
    # ABOVE FAILS in current state: Rancher v2.3.5
    # ("challenge.certmanager.k8s.io", "challenge.yaml"),
    # ABOVE FAILS in current state: Rancher v2.3.5
    # ("clusterissuer.certmanager.k8s.io", "clusterissuer.yaml"),
    # ABOVE FAILS in current state: Rancher v2.3.5
    # ("issuer.certmanager.k8s.io", "issuer.yaml"),
    # ABOVE FAILS in current state: Rancher v2.3.5
    # ("order.certmanager.k8s.io", "order.yaml"),
    # ABOVE FAILS in current state: Rancher v2.3.5
]


def test_istio_resources():
    app_client = namespace["app_client"]
    app_ns = namespace["app_ns"]
    gateway_url = namespace["gateway_url"]

    create_and_test_bookinfo_services(app_client, app_ns)
    create_bookinfo_virtual_service(app_client, app_ns)
    create_and_test_bookinfo_gateway(app_client, app_ns, gateway_url)
    create_and_test_bookinfo_routing(app_client, app_ns, gateway_url)


def test_istio_deployment_options():
    file_path = ISTIO_PATH + '/nginx-custom-sidecar.yaml'
    expected_image = "rancher/istio-proxyv2:1.4.3"
    p_client = namespace["app_client"]
    ns = namespace["app_ns"]

    execute_kubectl_cmd('apply -f ' + file_path + ' -n ' + ns.name, False)
    result = execute_kubectl_cmd('get deployment -n ' + ns.name, True)

    for deployment in result['items']:
        wl = p_client.list_workload(id='deployment:'
                                       + deployment['metadata']['namespace']
                                       + ':'
                                       + deployment['metadata']['name']).data[
            0]
        wl = wait_for_wl_to_active(p_client, wl, 60)
        wl_pods = wait_for_pods_in_workload(p_client, wl, 1)
        wait_for_pod_to_running(p_client, wl_pods[0])
    workload = p_client.list_workload(name="nginx-v1",
                                      namespaceId=ns.id).data[0]
    pod = p_client.list_pod(workloadId=workload.id).data[0]
    try:
        assert any(container.image == expected_image
                   for container in pod.containers)
    except AssertionError as e:
        retrieved_images = ""
        for container in pod.containers:
            retrieved_images += container.image + " "
        retrieved_images = retrieved_images.strip().split(" ")
        raise AssertionError("None of {} matches '{}'".format(
            retrieved_images, expected_image))


# Enables all possible istio custom answers with the exception of certmanager
def test_istio_custom_answers(skipif_unsupported_istio_version,
                              enable_all_options_except_certmanager):
    expected_deployments = [
        "grafana", "istio-citadel", "istio-egressgateway", "istio-galley",
        "istio-ilbgateway", "istio-ingressgateway", "istio-pilot",
        "istio-policy", "istio-sidecar-injector", "istio-telemetry",
        "istio-tracing", "istiocoredns", "kiali", "prometheus"
    ]
    expected_daemonsets = ["istio-nodeagent"]
    validate_all_workload_image_from_rancher(
        get_system_client(USER_TOKEN), namespace["system_ns"],
        ignore_pod_count=True, deployment_list=expected_deployments,
        daemonset_list=expected_daemonsets)


# This is split out separately from test_istio_custom_answers because
# certmanager creates its own crds outside of istio
def test_istio_certmanager_enables(skipif_unsupported_istio_version,
                                   enable_certmanager):
    expected_deployments = [
        "certmanager", "istio-citadel", "istio-galley", "istio-ingressgateway",
        "istio-pilot", "istio-policy", "istio-sidecar-injector",
        "istio-telemetry", "istio-tracing", "kiali"
    ]
    validate_all_workload_image_from_rancher(
        get_system_client(USER_TOKEN), namespace["system_ns"],
        ignore_pod_count=True, deployment_list=expected_deployments)


@if_test_rbac
def test_rbac_istio_metrics_allow_all_cluster_owner(allow_all_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_owner = rbac_get_user_token_by_role(CLUSTER_OWNER)
    validate_access(kiali_url, cluster_owner)
    validate_access(tracing_url, cluster_owner)


@if_test_rbac
def test_rbac_istio_monitoring_allow_all_cluster_owner(allow_all_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_owner = rbac_get_user_token_by_role(CLUSTER_OWNER)
    validate_access(grafana_url, cluster_owner)
    validate_access(prometheus_url, cluster_owner)


@if_test_rbac
def test_rbac_istio_metrics_allow_all_cluster_member(allow_all_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    validate_access(kiali_url, cluster_member)
    validate_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_istio_monitoring_allow_all_cluster_member(allow_all_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_istio_metrics_allow_all_project_owner(allow_all_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_OWNER)
    validate_access(kiali_url, cluster_member)
    validate_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_istio_monitoring_allow_all_project_owner(allow_all_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_OWNER)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_istio_metrics_allow_all_project_member(allow_all_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_access(kiali_url, cluster_member)
    validate_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_istio_monitoring_allow_all_project_member(allow_all_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_istio_metrics_allow_all_project_read(allow_all_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    validate_access(kiali_url, cluster_member)
    validate_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_istio_monitoring_allow_all_project_read(allow_all_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_istio_metrics_allow_none_cluster_owner(default_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_owner = rbac_get_user_token_by_role(CLUSTER_OWNER)
    validate_access(kiali_url, cluster_owner)
    validate_access(tracing_url, cluster_owner)


@if_test_rbac
def test_rbac_istio_monitoring_allow_none_cluster_owner(default_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_owner = rbac_get_user_token_by_role(CLUSTER_OWNER)
    validate_access(grafana_url, cluster_owner)
    validate_access(prometheus_url, cluster_owner)


@if_test_rbac
def test_rbac_istio_metrics_allow_none_cluster_member(default_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    validate_no_access(kiali_url, cluster_member)
    validate_no_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_istio_monitoring_allow_none_cluster_member(default_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_istio_metrics_allow_none_project_owner(default_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_OWNER)
    validate_no_access(kiali_url, cluster_member)
    validate_no_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_istio_monitoring_allow_none_project_owner(default_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_OWNER)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_istio_metrics_allow_none_project_member(default_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_no_access(kiali_url, cluster_member)
    validate_no_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_istio_monitoring_allow_none_project_member(default_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_istio_metrics_allow_none_project_read(default_access):
    kiali_url, tracing_url, _, _ = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    validate_no_access(kiali_url, cluster_member)
    validate_no_access(tracing_url, cluster_member)


@if_test_rbac
def test_rbac_istio_monitoring_allow_none_project_read(default_access):
    _, _, grafana_url, prometheus_url = get_urls()
    cluster_member = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    validate_no_access(grafana_url, cluster_member)
    validate_no_access(prometheus_url, cluster_member)


@if_test_rbac
def test_rbac_istio_update_cluster_member():
    user = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    with pytest.raises(ApiError) as e:
        update_istio_app({"FOO": "BAR"}, user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_istio_disable_cluster_member():
    user = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    with pytest.raises(ApiError) as e:
        delete_istio_app(user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_istio_update_project_owner():
    user = rbac_get_user_token_by_role(PROJECT_OWNER)
    with pytest.raises(ApiError) as e:
        update_istio_app({"FOO": "BAR"}, user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_istio_disable_project_owner():
    user = rbac_get_user_token_by_role(PROJECT_OWNER)
    with pytest.raises(ApiError) as e:
        delete_istio_app(user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_istio_update_project_member():
    user = rbac_get_user_token_by_role(PROJECT_MEMBER)
    with pytest.raises(ApiError) as e:
        update_istio_app({"FOO": "BAR"}, user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_istio_disable_project_member():
    user = rbac_get_user_token_by_role(PROJECT_MEMBER)
    with pytest.raises(ApiError) as e:
        delete_istio_app(user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_istio_update_project_read():
    user = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    with pytest.raises(ApiError) as e:
        update_istio_app({"FOO": "BAR"}, user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_istio_disable_project_read():
    user = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    with pytest.raises(ApiError) as e:
        delete_istio_app(user)

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
@pytest.mark.parametrize("crd,manifest", crd_test_data)
def test_rbac_istio_crds_project_owner(skipif_unsupported_istio_version,
                                       update_answers, crd, manifest):
    if "certmanager" in crd:
        update_answers("enable_certmanager")
    else :
        update_answers("default_access")
    kubectl_context = rbac_get_kubeconfig_by_role(PROJECT_OWNER)
    file = ISTIO_CRD_PATH + '/' + manifest
    ns = rbac_get_namespace()
    assert re.match("{}.* created".format(crd),
                    apply_crd(ns, file, kubectl_context))
    assert "Forbidden" not in get_crd(ns, crd, kubectl_context)
    assert re.match("{}.* deleted".format(crd),
                    delete_crd(ns, file, kubectl_context))


@if_test_rbac
@pytest.mark.parametrize("crd,manifest", crd_test_data)
def test_rbac_istio_crds_project_member(skipif_unsupported_istio_version,
                                        update_answers, crd, manifest):
    if "certmanager" in crd:
        update_answers("enable_certmanager")
    else :
        update_answers("default_access")
    kubectl_context = rbac_get_kubeconfig_by_role(PROJECT_MEMBER)
    file = ISTIO_CRD_PATH + '/' + manifest
    ns = rbac_get_namespace()
    assert re.match("{}.* created".format(crd),
                    apply_crd(ns, file, kubectl_context))
    assert "Forbidden" not in get_crd(ns, crd, kubectl_context)
    assert re.match("{}.* deleted".format(crd),
                    delete_crd(ns, file, kubectl_context))


@if_test_rbac
@pytest.mark.parametrize("crd,manifest", crd_test_data)
def test_rbac_istio_crds_project_read(skipif_unsupported_istio_version,
                                      update_answers, crd, manifest):
    if "certmanager" in crd:
        update_answers("enable_certmanager")
    else :
        update_answers("default_access")
    kubectl_context = rbac_get_kubeconfig_by_role(PROJECT_READ_ONLY)
    file = ISTIO_CRD_PATH + '/' + manifest
    ns = rbac_get_namespace()
    assert str(apply_crd(ns, file, kubectl_context)).startswith(
        "Error from server (Forbidden)")
    assert "Forbidden" not in get_crd(ns, crd, kubectl_context)
    assert str(delete_crd(ns, file, kubectl_context)).startswith(
        "Error from server (Forbidden)")


@if_test_group_rbac
def test_rbac_istio_group_access(auth_cluster_access, update_answers):
    group, users, noauth_user = auth_cluster_access
    update_answers("allow_group_access", group=group)
    kiali_url, tracing_url, grafana_url, prometheus_url = get_urls()
    for user in users:
        user_token = auth_get_user_token(user)
        print("Validating {} has access.".format(user))
        validate_access(kiali_url, user_token)
        validate_access(tracing_url, user_token)
        validate_no_access(grafana_url, user_token)
        validate_no_access(prometheus_url, user_token)

    print("Validating {} does not have access.".format(noauth_user))
    noauth_token = auth_get_user_token(noauth_user)
    validate_no_access(kiali_url, noauth_token)
    validate_no_access(tracing_url, noauth_token)
    validate_no_access(grafana_url, noauth_token)
    validate_no_access(prometheus_url, noauth_token)


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


def update_istio_app(answers, user, app=None, ns=None, project=None):
    if app is None:
        app = namespace["istio_app"]
    if ns is None:
        ns = namespace["system_ns"]
    if project is None:
        project = namespace["system_project"]
    p_client = get_system_client(user)
    updated_answers = copy.deepcopy(DEFAULT_ANSWERS)
    updated_answers.update(answers)
    namespace["istio_app"] = p_client.update(
        obj=app,
        externalId=ISTIO_EXTERNAL_ID,
        targetNamespace=ns.name,
        projectId=project.id,
        answers=updated_answers)
    verify_istio_app_ready(p_client, namespace["istio_app"], 120, 120)


def create_and_verify_istio_app(p_client, ns, project):
    print("creating istio catalog app")
    app = p_client.create_app(
        name="cluster-istio",
        externalId=ISTIO_EXTERNAL_ID,
        targetNamespace=ns.name,
        projectId=project.id,
        answers=DEFAULT_ANSWERS
    )
    verify_istio_app_ready(p_client, app, 120, 600)
    return app


def delete_istio_app(user):
    p_client = get_system_client(user)
    p_client.delete(namespace["istio_app"])


def verify_istio_app_ready(p_client, app, install_timeout, deploy_timeout,
                           initial_run=True):
    if initial_run:
        print("Verify Istio App has installed and deployed properly")
    if install_timeout <= 0 or deploy_timeout <= 0:
        raise TimeoutError("Timeout waiting for istio to be properly "
                           "installed and deployed.") from None
    elif 'conditions' in app and not initial_run:
        for cond in app['conditions']:
            if "False" in cond['status'] and 'message' in cond \
                    and "failed" in cond['message']:
                raise AssertionError(
                    "Failed to properly install/deploy app. Reason: {}".format(
                        cond['message'])) from None
    try:
        wait_for_condition(p_client, app, check_condition('Installed', 'True'),
                           timeout=2)
    except (Exception, TypeError):
        verify_istio_app_ready(p_client, p_client.list_app(
            name='cluster-istio').data[0], install_timeout-2, deploy_timeout,
                               initial_run=False)
    try:
        wait_for_condition(p_client, app, check_condition('Deployed', 'True'),
                           timeout=2)
    except (Exception, TypeError):
        verify_istio_app_ready(p_client, p_client.list_app(
            name='cluster-istio').data[0], 2, deploy_timeout-2,
                               initial_run=False)


def get_urls():
    _, cluster = get_user_client_and_cluster()
    if namespace["istio_version"] == "0.1.0" \
            or namespace["istio_version"] == "0.1.1":
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


def add_user_to_cluster(username):
    class User(object):
        def __init__(self, u_name, user_id, token):
            self.username = u_name
            self.id = user_id
            self.token = token
    user_data = login_as_auth_user(username, AUTH_USER_PASSWORD)
    u_id = user_data['userId']
    u_token = user_data['token']
    user_obj = User(username, u_id, u_token)
    add_role_to_user(user_obj, CLUSTER_MEMBER)
    # Enable one of these two below options to get around Issue #25365
    get_client_for_token(u_token)
    # headers = {'Authorization': 'Bearer ' + u_token}
    # url = os.environ.get('CATTLE_TEST_URL', "") + "/v3/users?me=true"
    # response = requests.get(headers=headers, url=url, verify=False)


@pytest.fixture()
def update_answers():
    def _update_answers(answer_type, group=None):
        answers = {
            "kiali.enabled": "true",
            "tracing.enabled": "true",
        }
        if answer_type == "allow_all_access":
            additional_answers = {
                "global.members[0].kind": "Group",
                "global.members[0].name": "system:authenticated",
            }
            answers.update(additional_answers)
        elif answer_type == "allow_group_access":
            auth_admin = login_as_auth_user(load_setup_data()["admin_user"],
                                            AUTH_USER_PASSWORD)
            group_id = get_group_principal_id(group, token=auth_admin['token'])
            additional_answers = {
                "global.members[0].kind": "Group",
                "global.members[0].name": group_id,
            }
            answers.update(additional_answers)
        elif answer_type == "enable_certmanager":
            additional_answers = {"certmanager.enabled": "true"}
            answers.update(additional_answers)
        elif answer_type == "enable_all_options_except_certmanager":
            additional_answers = {
                "gateways.istio-egressgateway.enabled": "true",
                "gateways.istio-ilbgateway.enabled": "true",
                "gateways.istio-ingressgateway.sds.enabled": "true",
                "global.proxy.accessLogFile": "/dev/stdout",
                "grafana.enabled": "true",
                "istiocoredns.enabled": "true",
                "kiali.dashboard.grafanaURL": "",
                "kiali.prometheusAddr": "http://prometheus:9090",
                "nodeagent.enabled": "true",
                "nodeagent.env.CA_ADDR": "istio-citadel:8060",
                "nodeagent.env.CA_PROVIDER": "Citadel",
                "prometheus.enabled": "true",
            }
            answers.update(additional_answers)
        update_istio_app(answers, USER_TOKEN)
    return _update_answers


@pytest.fixture()
def default_access(update_answers):
    update_answers("default_access")


@pytest.fixture()
def allow_all_access(update_answers):
    update_answers("allow_all_access")


@pytest.fixture()
def enable_certmanager(update_answers):
    update_answers("enable_certmanager")


@pytest.fixture()
def enable_all_options_except_certmanager(update_answers):
    update_answers("enable_all_options_except_certmanager")


@pytest.fixture(scope='function')
def skipif_unsupported_istio_version(request):
    if ISTIO_VERSION != "":
        istio_version = ISTIO_VERSION
    else:
        client, _ = get_user_client_and_cluster()
        istio_versions = list(client.list_template(
            id=ISTIO_TEMPLATE_ID).data[0].versionLinks.keys())
        istio_version = istio_versions[len(istio_versions) - 1]
    if compare_versions(istio_version, "1.4.3") < 0:
        pytest.skip("This test is not supported for older Istio versions")


@pytest.fixture(scope='function')
def auth_cluster_access(request):
    group, noauth_user = get_a_group_and_a_user_not_in_it(
        NESTED_GROUP_ENABLED)
    users = get_user_by_group(group, NESTED_GROUP_ENABLED)
    for user in users:
        add_user_to_cluster(user)
    add_user_to_cluster(noauth_user)

    def fin():
        auth_resource_cleanup()
    request.addfinalizer(fin)
    return group, users, noauth_user


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    global DEFAULT_ANSWERS
    global ISTIO_EXTERNAL_ID
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)

    admin_client = get_admin_client()
    ad_enabled = admin_client.by_id_auth_config("activedirectory").enabled
    if AUTH_PROVIDER == "activeDirectory" and not ad_enabled:
        enable_ad(load_setup_data()["admin_user"], ADMIN_TOKEN, 
                  password=AUTH_USER_PASSWORD, nested=NESTED_GROUP_ENABLED)

    projects = client.list_project(name='System', clusterId=cluster.id)
    if len(projects.data) == 0:
        raise AssertionError(
            "System project not found in the cluster " + cluster.name)
    p = projects.data[0]
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)

    istio_versions = list(client.list_template(
        id=ISTIO_TEMPLATE_ID).data[0].versionLinks.keys())
    istio_version = istio_versions[len(istio_versions) - 1]

    if ISTIO_VERSION != "":
        istio_version = ISTIO_VERSION
    ISTIO_EXTERNAL_ID += istio_version
    answers = {"global.rancher.clusterId": p.clusterId}
    DEFAULT_ANSWERS.update(answers)

    monitoring_answers = copy.deepcopy(C_MONITORING_ANSWERS)
    monitoring_answers["prometheus.persistence.enabled"] = "false"
    monitoring_answers["grafana.persistence.enabled"] = "false"

    if cluster["enableClusterMonitoring"] is False:
        client.action(cluster, "enableMonitoring",
                      answers=monitoring_answers)

    if cluster["istioEnabled"] is False:
        verify_admission_webhook()

        ns = create_ns(c_client, cluster, p, 'istio-system')
        app = create_and_verify_istio_app(p_client, ns, p)
    else:
        app = p_client.list_app(name='cluster-istio').data[0]
        ns = c_client.list_namespace(name='istio-system').data[0]
        update_istio_app(DEFAULT_ANSWERS, USER_TOKEN,
                         app=app, ns=ns, project=p)

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
        # delete the istio app
        app = p_client.delete(namespace["istio_app"])
        validate_app_deletion(p_client, app.id)
        # delete the istio ns
        p_client.delete(namespace["system_ns"])
        # disable the cluster monitoring
        c = client.reload(cluster)
        if c["enableClusterMonitoring"] is True:
            client.action(c, "disableMonitoring")
        # delete the istio testing project
        client.delete(istio_project)
    request.addfinalizer(fin)
