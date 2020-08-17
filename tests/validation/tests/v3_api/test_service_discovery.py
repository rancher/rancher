"""
This file contains tests for service discovery.
This file also has rbac tests based on different roles
Test requirement:
Below Env variables need to set
CATTLE_TEST_URL - url to rancher server
ADMIN_TOKEN - Admin token from rancher
USER_TOKEN - User token from rancher
RANCHER_CLUSTER_NAME - Cluster name to run test on
RANCHER_TEST_RBAC - Boolean (Optional), To run role based tests.
"""


from .common import ApiError
from .common import ast
from .common import CLUSTER_MEMBER
from .common import CLUSTER_OWNER
from .common import create_kubeconfig
from .common import create_ns
from .common import create_project_and_ns
from .common import get_cluster_client_for_token
from .common import get_project_client_for_token
from .common import get_user_client
from .common import get_user_client_and_cluster
from .common import if_test_rbac
from .common import os
from .common import PROJECT_MEMBER
from .common import PROJECT_OWNER
from .common import PROJECT_READ_ONLY
from .common import pytest
from .common import random_test_name
from .common import rbac_get_namespace
from .common import rbac_get_project
from .common import rbac_get_user_token_by_role
from .common import rbac_get_workload
from .common import skip_test_windows_os
from .common import TEST_IMAGE
from .common import TEST_IMAGE_NGINX
from .common import time
from .common import USER_TOKEN
from .common import validate_dns_record
from .common import validate_dns_record_deleted
from .common import validate_service_discovery
from .common import validate_workload
from .common import validate_workload_image
from .common import wait_for_condition
from .common import wait_for_pod_images
from .common import wait_for_pods_in_workload
from .common import wait_for_pod_to_running
from .common import wait_for_wl_to_active


RANCHER_CLEANUP_PROJECT = os.environ.get("RANCHER_CLEANUP_PROJECT", "True")
namespace = {"p_client": None, "ns": None, "cluster": None,
             "project": None, "testclient_pods": [], "workload": None}
rbac_role_list = [
                  CLUSTER_OWNER,
                  CLUSTER_MEMBER,
                  PROJECT_OWNER,
                  PROJECT_MEMBER,
                  PROJECT_READ_ONLY
                 ]
DNS_RESOLUTION_DEFAULT_SECONDS = \
    os.environ.get("RANCHER_DNS_RESOLUTION_SECONDS", 30)
SKIP_PING_CHECK_TEST = \
    ast.literal_eval(os.environ.get('RANCHER_SKIP_PING_CHECK_TEST', "False"))
if_skip_ping_check_test = pytest.mark.skipif(
    SKIP_PING_CHECK_TEST,
    reason='For skipping tests in clusters that ' \
           'are deployed with security groups that will not allow ping')


def create_and_validate_wl(name, con, scale, type, p_client=None, ns=None):
    if p_client is None:
        p_client = namespace["p_client"]
    if ns is None:
        ns = namespace["ns"]

    workload = p_client.create_workload(name=name, containers=con,
                                        namespaceId=ns.id, scale=scale)
    wait_for_pods_in_workload(p_client, workload, scale)
    validate_workload(p_client, workload, type, ns.id, pod_count=scale)
    return workload


def update_and_validate_workload(workload, con, scale, p_client=None, ns=None):
    if p_client is None:
        p_client = namespace["p_client"]
    if ns is None:
        ns = namespace["ns"]

    p_client.update(workload, containers=con, scale=scale)
    workload = wait_for_wl_to_active(p_client, workload)
    wait_for_pod_images(p_client, workload, ns.name, con[0]["image"], scale)
    wait_for_pods_in_workload(p_client, workload, scale)
    validate_workload(p_client, workload, "deployment", ns.name, scale)
    validate_workload_image(p_client, workload, con[0]["image"], ns)


def validate_dns_record_for_workload(workload, scale, record,
                                     p_client=None, testclient_pods=None):
    if p_client is None:
        p_client = namespace["p_client"]
    if testclient_pods is None:
        testclient_pods = namespace["testclient_pods"]

    expected_ips = []
    pods = p_client.list_pod(workloadId=workload["id"]).data
    assert len(pods) == scale
    for pod in pods:
        expected_ips.append(pod["status"]["podIp"])
    for pod in testclient_pods:
        validate_dns_record(pod, record, expected_ips)


def test_service_discovery_when_workload_scale_up():
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("test-sd-up")
    type = "deployment"

    # deploy a workload
    scale = 2
    workload = create_and_validate_wl(name, con, scale, type)
    # test service discovery
    validate_service_discovery(workload, scale, namespace["p_client"],
                               namespace["ns"], namespace["testclient_pods"])

    # workload scales up to 3 pods
    scale = 3
    update_and_validate_workload(workload, con, scale)
    # test service discovery
    time.sleep(DNS_RESOLUTION_DEFAULT_SECONDS)
    validate_service_discovery(workload, scale, namespace["p_client"],
                               namespace["ns"], namespace["testclient_pods"])


def test_service_discovery_when_workload_scale_down():
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("test-sd-dw")
    type = "deployment"

    # deploy a workload
    scale = 3
    workload = create_and_validate_wl(name, con, scale, type)
    # test service discovery
    validate_service_discovery(workload, scale, namespace["p_client"],
                               namespace["ns"], namespace["testclient_pods"])

    # workload scale down to 2 pods
    scale = 2
    update_and_validate_workload(workload, con, scale)
    # test service discovery
    time.sleep(DNS_RESOLUTION_DEFAULT_SECONDS)
    validate_service_discovery(workload, scale, namespace["p_client"],
                               namespace["ns"], namespace["testclient_pods"])


def test_service_discovery_when_workload_upgrade():
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("test-sd-upgrade")
    type = "deployment"
    scale = 2

    # deploy a workload
    workload = create_and_validate_wl(name, con, scale, type)
    # test service discovery
    validate_service_discovery(workload, scale, namespace["p_client"],
                               namespace["ns"], namespace["testclient_pods"])

    # upgrade
    con = [{"name": "test1",
            "image": TEST_IMAGE_NGINX}]
    update_and_validate_workload(workload, con, scale)
    # test service discovery
    time.sleep(DNS_RESOLUTION_DEFAULT_SECONDS)
    validate_service_discovery(workload, scale, namespace["p_client"],
                               namespace["ns"], namespace["testclient_pods"])

    # upgrade again
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    update_and_validate_workload(workload, con, scale)
    # test service discovery
    time.sleep(DNS_RESOLUTION_DEFAULT_SECONDS)
    validate_service_discovery(workload, scale, namespace["p_client"],
                               namespace["ns"], namespace["testclient_pods"])


def test_dns_record_type_workload_when_workload_scale_up():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("test-dns-up")
    type = "deployment"

    # deploy a workload
    scale = 2
    workload = create_and_validate_wl(name, con, scale, type)
    record = {"type": "dnsRecord", "targetWorkloadIds": [workload["id"]],
              "name": random_test_name("record"), "namespaceId": ns.id}
    create_dns_record(record, p_client)
    # test dns record for the workload
    validate_dns_record_for_workload(workload, scale, record)

    # workload scale up to 3 pods
    scale = 3
    update_and_validate_workload(workload, con, scale)

    # test service discovery
    time.sleep(DNS_RESOLUTION_DEFAULT_SECONDS)
    validate_dns_record_for_workload(workload, scale, record)


def test_dns_record_type_workload_when_workload_scale_down():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("test-dns-dw")
    type = "deployment"

    # deploy a workload
    scale = 3
    workload = create_and_validate_wl(name, con, scale, type)
    record = {"type": "dnsRecord",
              "targetWorkloadIds": [workload["id"]],
              "name": random_test_name("record"),
              "namespaceId": ns.id}
    create_dns_record(record, p_client)
    # test service discovery
    validate_dns_record_for_workload(workload, scale, record)

    # workload scale down to 2 pods
    scale = 2
    update_and_validate_workload(workload, con, scale)

    # test service discovery
    time.sleep(DNS_RESOLUTION_DEFAULT_SECONDS)
    validate_dns_record_for_workload(workload, scale, record)


def test_dns_record_type_workload_when_workload_upgrade():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("test-dns-upgrade")
    scale = 2
    type = "deployment"

    # deploy a workload
    workload = create_and_validate_wl(name, con, scale, type)
    record = {"type": "dnsRecord", "targetWorkloadIds": [workload["id"]],
              "name": random_test_name("record"), "namespaceId": ns.id}
    create_dns_record(record, p_client)
    # test service discovery
    validate_dns_record_for_workload(workload, scale, record)

    # upgrade the workload
    con = [{"name": "test1",
            "image": TEST_IMAGE_NGINX}]
    update_and_validate_workload(workload, con, scale)
    # test service discovery
    time.sleep(DNS_RESOLUTION_DEFAULT_SECONDS)
    validate_dns_record_for_workload(workload, scale, record)

    # upgrade the workload again
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    update_and_validate_workload(workload, con, scale)
    # test service discovery
    time.sleep(DNS_RESOLUTION_DEFAULT_SECONDS)
    validate_dns_record_for_workload(workload, scale, record)


# Windows could not transpose the remote ICMP packages,
# since TCP/UDP packets can still be transposed,
# one can substitute ping <destination> with curl <destination> to be able
# to debug connectivity to outside
@skip_test_windows_os
@if_skip_ping_check_test
def test_dns_record_type_external_ip():
    ns = namespace["ns"]
    record = {"type": "dnsRecord", "ipAddresses": ["8.8.8.8"],
              "name": random_test_name("record"), "namespaceId": ns.id}
    expected = record["ipAddresses"]
    create_and_validate_dns_record(record, expected)


# Windows could not transpose the remote ICMP packages,
# since TCP/UDP packets can still be transposed,
# one can substitute ping <destination> with curl <destination> to be able
# to debug connectivity to outside
@skip_test_windows_os
@if_skip_ping_check_test
def test_dns_record_type_multiple_external_ips():
    ns = namespace["ns"]
    record = {"type": "dnsRecord", "ipAddresses": ["8.8.8.8", "8.8.4.4"],
              "name": random_test_name("record"), "namespaceId": ns.id}
    expected = record["ipAddresses"]
    create_and_validate_dns_record(record, expected)


# Windows could not transpose the remote ICMP packages,
# since TCP/UDP packets can still be transposed,
# one can substitute ping <destination> with curl <destination> to be able
# to debug connectivity to outside
@skip_test_windows_os
@if_skip_ping_check_test
def test_dns_record_type_hostname():
    ns = namespace["ns"]
    record = {"type": "dnsRecord", "hostname": "google.com",
              "name": random_test_name("record"), "namespaceId": ns.id}
    expected = [record["hostname"]]
    create_and_validate_dns_record(record, expected)


# Windows could not transpose the remote ICMP packages,
# since TCP/UDP packets can still be transposed,
# one can substitute ping <destination> with curl <destination> to be able
# to debug connectivity to outside
@skip_test_windows_os
@if_skip_ping_check_test
def test_dns_record_type_alias():
    ns = namespace["ns"]

    first_record = {"type": "dnsRecord", "hostname": "google.com",
                    "name": random_test_name("record"), "namespaceId": ns.id}
    target_record = create_dns_record(first_record)

    record = {"type": "dnsRecord", "targetDnsRecordIds": [target_record["id"]],
              "name": random_test_name("record"), "namespaceId": ns.id}

    expected = [first_record["hostname"]]
    create_and_validate_dns_record(record, expected)


def test_dns_record_type_workload():
    ns = namespace["ns"]
    workload = namespace["workload"]
    p_client = namespace["p_client"]

    record = {"type": "dnsRecord", "targetWorkloadIds": [workload["id"]],
              "name": random_test_name("record"), "namespaceId": ns.id}

    expected_ips = []
    pods = p_client.list_pod(workloadId=workload["id"]).data
    for pod in pods:
        expected_ips.append(pod["status"]["podIp"])

    create_and_validate_dns_record(record, expected_ips)


def test_dns_record_type_multiple_workloads():
    ns = namespace["ns"]
    workload = namespace["workload"]
    p_client = namespace["p_client"]

    wlname = random_test_name("default")

    con = [{"name": "test1",
            "image": TEST_IMAGE}]

    additional_workload = p_client.create_workload(name=wlname,
                                                   containers=con,
                                                   namespaceId=ns.id,
                                                   scale=1)
    wait_for_wl_to_active(p_client, additional_workload)
    awl_pods = wait_for_pods_in_workload(p_client, additional_workload, 1)
    wait_for_pod_to_running(p_client, awl_pods[0])

    record = {"type": "dnsRecord",
              "targetWorkloadIds": [workload["id"], additional_workload["id"]],
              "name": random_test_name("record"),
              "namespaceId": ns.id}

    workloads = [workload, additional_workload]
    expected_ips = []

    for wl in workloads:
        pods = p_client.list_pod(workloadId=wl["id"]).data
        for pod in pods:
            expected_ips.append(pod["status"]["podIp"])

    create_and_validate_dns_record(record, expected_ips)


def test_dns_record_type_selector():
    ns = namespace["ns"]
    workload = namespace["workload"]
    p_client = namespace["p_client"]

    selector = \
        workload["labels"]["workload.user.cattle.io/workloadselector"]

    record = {"type": "dnsRecord",
              "selector":
                  {"workload.user.cattle.io/workloadselector": selector},
              "name": random_test_name("record"), "namespaceId": ns.id}

    expected_ips = []
    pods = p_client.list_pod(workloadId=workload["id"]).data
    for pod in pods:
        expected_ips.append(pod["status"]["podIp"])

    create_and_validate_dns_record(record, expected_ips)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_service_discovery_create(role):
    """
    Creates dns record and validates it for different roles passed in parameter
    @param role: User role in rancher eg. project owner, project member etc
    """
    token = rbac_get_user_token_by_role(role)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    workload = rbac_get_workload()
    p_client = get_project_client_for_token(project, token)

    record = {"type": "dnsRecord", "targetWorkloadIds": [workload["id"]],
              "name": random_test_name("record"), "namespaceId": ns.id}
    if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
        with pytest.raises(ApiError) as e:
            dns_record = create_dns_record(record, p_client)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
    else:
        dns_record = create_dns_record(record, p_client)
        # test dns record for the workload
        validate_dns_record_for_workload(workload, 1,
                                         record, p_client=p_client)
        p_client.delete(dns_record)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_service_discovery_edit(role):
    """
    Creates dns record with cluster owner role and edit it with different roles
    @param role: User role in rancher eg. project owner, project member etc
    """
    c_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    token = rbac_get_user_token_by_role(role)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    workload_1 = rbac_get_workload()
    p_client = get_project_client_for_token(project, token)
    p_client_for_c_owner = get_project_client_for_token(project, c_owner_token)
    wlname = random_test_name("default")

    con = [{"name": "test1",
            "image": TEST_IMAGE}]

    workload_2 = p_client_for_c_owner.create_workload(name=wlname,
                                                      containers=con,
                                                      namespaceId=ns.id,
                                                      scale=1)
    wait_for_wl_to_active(p_client_for_c_owner, workload_2)

    record_1 = {"type": "dnsRecord", "targetWorkloadIds": [workload_1["id"]],
                "name": random_test_name("record"), "namespaceId": ns.id}
    dns_record = create_dns_record(record_1, p_client_for_c_owner)
    validate_dns_record_for_workload(workload_1, 1,
                                     record_1, p_client=p_client_for_c_owner)
    if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
        with pytest.raises(ApiError) as e:
            p_client.update(dns_record, targetWorkloadIds=workload_2["id"])
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
    else:
        p_client.update(dns_record, type="dnsRecord",
                        targetWorkloadIds=[workload_2["id"]])
        p_client.reload(dns_record)
    p_client_for_c_owner.delete(dns_record)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_service_discovery_delete(role):
    """
    Creates dns record with cluster owner and delete with different roles.
    @param role: User role in rancher eg. project owner, project member etc
    """
    c_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    token = rbac_get_user_token_by_role(role)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    workload = rbac_get_workload()
    p_client = get_project_client_for_token(project, token)
    p_client_for_c_owner = get_project_client_for_token(project, c_owner_token)

    record = {"type": "dnsRecord", "targetWorkloadIds": [workload["id"]],
              "name": random_test_name("record"), "namespaceId": ns.id}
    dns_record = create_dns_record(record, p_client_for_c_owner)
    validate_dns_record_for_workload(workload, 1,
                                     record, p_client=p_client_for_c_owner)
    if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
        with pytest.raises(ApiError) as e:
            p_client.delete(dns_record)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
    else:
        p_client.delete(dns_record)
        validate_dns_record_deleted(p_client, dns_record)


def create_and_validate_dns_record(record, expected, p_client=None,
                                   testclient_pods=None):
    if testclient_pods is None:
        testclient_pods = namespace["testclient_pods"]
    create_dns_record(record, p_client)
    assert len(testclient_pods) > 0
    for pod in testclient_pods:
        validate_dns_record(pod, record, expected)


def create_dns_record(record, p_client=None):
    if p_client is None:
        p_client = namespace["p_client"]
    created_record = p_client.create_dns_record(record)

    wait_for_condition(
        p_client, created_record,
        lambda x: x.state == "active",
        lambda x: 'State is: ' + x.state)

    return created_record


@pytest.fixture(scope='module', autouse="True")
def setup(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)

    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testsd")
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)

    new_ns = create_ns(c_client, cluster, p)

    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p

    wlname = random_test_name("default")

    con = [{"name": "test1",
            "image": TEST_IMAGE}]

    workload = p_client.create_workload(name=wlname,
                                        containers=con,
                                        namespaceId=ns.id,
                                        scale=2)
    wait_for_wl_to_active(p_client, workload)
    namespace["workload"] = workload

    pods = wait_for_pods_in_workload(p_client, workload, 2)
    pod = wait_for_pod_to_running(p_client, pods[0])
    namespace["testclient_pods"].append(pod)

    workload = p_client.create_workload(name=wlname,
                                        containers=con,
                                        namespaceId=new_ns.id,
                                        scale=1)
    wait_for_wl_to_active(p_client, workload)
    pods = wait_for_pods_in_workload(p_client, workload, 1)
    pod = wait_for_pod_to_running(p_client, pods[0])
    namespace["testclient_pods"].append(pod)

    assert len(namespace["testclient_pods"]) == 2

    def fin():
        client = get_user_client()
        client.delete(namespace["project"])

    if RANCHER_CLEANUP_PROJECT == "True":
        request.addfinalizer(fin)
