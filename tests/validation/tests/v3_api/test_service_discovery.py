import os

import pytest

from .common import *  # NOQA

RANCHER_CLEANUP_PROJECT = os.environ.get("RANCHER_CLEANUP_PROJECT", "True")
namespace = {"p_client": None, "ns": None, "cluster": None,
             "project": None, "testclient_pods": [], "workload": None}
DNS_RESOLUTION_DEFAULT_SECONDS = \
    os.environ.get("RANCHER_DNS_RESOLUTION_SECONDS", 30)
SKIP_PING_CHECK_TEST = \
    ast.literal_eval(os.environ.get('RANCHER_SKIP_PING_CHECK_TEST', "False"))
if_skip_ping_check_test = pytest.mark.skipif(
    SKIP_PING_CHECK_TEST,
    reason='This test is only for testing upgrading Rancher')


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


def validate_service_discovery(workload, scale,
                               p_client=None, ns=None, testclient_pods=None):
    if p_client is None:
        p_client = namespace["p_client"]
    if ns is None:
        ns = namespace["ns"]
    if testclient_pods is None:
        testclient_pods = namespace["testclient_pods"]

    expected_ips = []
    pods = p_client.list_pod(workloadId=workload["id"]).data
    assert len(pods) == scale
    for pod in pods:
        expected_ips.append(pod["status"]["podIp"])
    host = '{0}.{1}.svc.cluster.local'.format(workload.name, ns.id)
    for pod in testclient_pods:
        validate_dns_entry(pod, host, expected_ips)


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
    validate_service_discovery(workload, scale)

    # workload scales up to 3 pods
    scale = 3
    update_and_validate_workload(workload, con, scale)
    # test service discovery
    time.sleep(DNS_RESOLUTION_DEFAULT_SECONDS)
    validate_service_discovery(workload, scale)


def test_service_discovery_when_workload_scale_down():
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("test-sd-dw")
    type = "deployment"

    # deploy a workload
    scale = 3
    workload = create_and_validate_wl(name, con, scale, type)
    # test service discovery
    validate_service_discovery(workload, scale)

    # workload scale down to 2 pods
    scale = 2
    update_and_validate_workload(workload, con, scale)
    # test service discovery
    time.sleep(DNS_RESOLUTION_DEFAULT_SECONDS)
    validate_service_discovery(workload, scale)


def test_service_discovery_when_workload_upgrade():
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("test-sd-upgrade")
    type = "deployment"
    scale = 2

    # deploy a workload
    workload = create_and_validate_wl(name, con, scale, type)
    # test service discovery
    validate_service_discovery(workload, scale)

    # upgrade
    con = [{"name": "test1",
            "image": TEST_IMAGE_NGINX}]
    update_and_validate_workload(workload, con, scale)
    # test service discovery
    time.sleep(DNS_RESOLUTION_DEFAULT_SECONDS)
    validate_service_discovery(workload, scale)

    # upgrade again
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    update_and_validate_workload(workload, con, scale)
    # test service discovery
    time.sleep(DNS_RESOLUTION_DEFAULT_SECONDS)
    validate_service_discovery(workload, scale)


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


# Windows could not transpose the remote ICMP packages, since TCP/UDP packets can still be transposed,
# one can substitute ping <destination> with curl <destination> to be able to debug connectivity to outside
@skip_test_windows_os
@if_skip_ping_check_test
def test_dns_record_type_external_ip():
    ns = namespace["ns"]
    record = {"type": "dnsRecord", "ipAddresses": ["8.8.8.8"],
              "name": random_test_name("record"), "namespaceId": ns.id}
    expected = record["ipAddresses"]
    create_and_validate_dns_record(record, expected)


# Windows could not transpose the remote ICMP packages, since TCP/UDP packets can still be transposed,
# one can substitute ping <destination> with curl <destination> to be able to debug connectivity to outside
@skip_test_windows_os
@if_skip_ping_check_test
def test_dns_record_type_multiple_external_ips():
    ns = namespace["ns"]
    record = {"type": "dnsRecord", "ipAddresses": ["8.8.8.8", "8.8.4.4"],
              "name": random_test_name("record"), "namespaceId": ns.id}
    expected = record["ipAddresses"]
    create_and_validate_dns_record(record, expected)


# Windows could not transpose the remote ICMP packages, since TCP/UDP packets can still be transposed,
# one can substitute ping <destination> with curl <destination> to be able to debug connectivity to outside
@skip_test_windows_os
@if_skip_ping_check_test
def test_dns_record_type_hostname():
    ns = namespace["ns"]
    record = {"type": "dnsRecord", "hostname": "google.com",
              "name": random_test_name("record"), "namespaceId": ns.id}
    expected = [record["hostname"]]
    create_and_validate_dns_record(record, expected)


# Windows could not transpose the remote ICMP packages, since TCP/UDP packets can still be transposed,
# one can substitute ping <destination> with curl <destination> to be able to debug connectivity to outside
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
