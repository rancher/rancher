import pytest
import requests
import time
from rancher import ApiError
from lib.aws import AmazonWebServices
from .common import CLUSTER_MEMBER, configure_cis_requirements
from .common import CLUSTER_OWNER
from .common import RESTRICTED_ADMIN
from .common import CIS_SCAN_PROFILE
from .common import cluster_cleanup
from .common import get_user_client
from .common import get_user_client_and_cluster
from .common import get_custom_host_registration_cmd
from .common import get_project_client_for_token
from .common import get_client_for_token
from .common import get_cluster_by_name
from .common import if_test_rbac
from .common import PROJECT_OWNER
from .common import PROJECT_MEMBER
from .common import PROJECT_READ_ONLY
from .common import random_test_name
from .common import rbac_get_user_token_by_role
from .common import USER_TOKEN
from .common import validate_cluster_state
from .common import wait_for_cluster_node_count
from .test_rke_cluster_provisioning import HOST_NAME, \
    POD_SECURITY_POLICY_TEMPLATE, get_cis_rke_config  # NOQA

scan_results = {
    "rke-cis-1.4": {
        "permissive": {"pass": 63, "skip": 15},
        "hardened": {"pass": 78, "skip": 0},
        "not_applicable": 19, "total": 97, "fail": 0
    },
    "rke-cis-1.5": {
        "permissive": {"pass": 58, "fail": 0, "skip": 14},
        "hardened": {"pass": 72, "fail": 0, "skip": 0},
        "not_applicable": 20, "total": 92, "fail": 0
    }
}
DEFAULT_TIMEOUT = 120
cluster_detail = {
    "cluster_14": None, "nodes_14": None, "name": None,
    "cluster_15": None, "nodes_15": None
}


def test_cis_scan_run_scan_hardened_14():
    cluster = cluster_detail["cluster_14"]
    scan_detail = run_scan(cluster, USER_TOKEN, "hardened")
    report_link = scan_detail["links"]["report"]
    test_total, tests_passed, tests_skipped, tests_failed, tests_na = \
        get_scan_results("rke-cis-1.4", "hardened")
    verify_cis_scan_report(
        report_link, token=USER_TOKEN,
        test_total=test_total,
        tests_passed=tests_passed,
        tests_skipped=tests_skipped,
        tests_failed=tests_failed,
        tests_na=tests_na
    )


def test_cis_scan_run_scan_hardened_15():
    """
    This will fail because of 2 tests which fail - 5.1.5 and 5.3.2
    :return:
    """
    cluster = cluster_detail["cluster_15"]
    scan_detail = run_scan(cluster, USER_TOKEN, "hardened",
                           scan_tool_version="rke-cis-1.5")
    report_link = scan_detail["links"]["report"]
    test_total, tests_passed, tests_skipped, tests_failed, tests_na = \
        get_scan_results("rke-cis-1.5", "hardened")
    verify_cis_scan_report(
        report_link, token=USER_TOKEN,
        test_total=test_total,
        tests_passed=tests_passed,
        tests_skipped=tests_skipped,
        tests_failed=tests_failed,
        tests_na=tests_na
    )


def test_cis_scan_run_scan_permissive_14():
    client, cluster = get_user_client_and_cluster()
    scan_detail = run_scan(cluster, USER_TOKEN, "permissive")
    report_link = scan_detail["links"]["report"]
    test_total, tests_passed, tests_skipped, tests_failed, tests_na = \
        get_scan_results("rke-cis-1.4", "permissive")
    verify_cis_scan_report(
        report_link, token=USER_TOKEN,
        test_total=test_total,
        tests_passed=tests_passed,
        tests_skipped=tests_skipped,
        tests_failed=tests_failed,
        tests_na=tests_na
    )


def test_cis_scan_run_scan_permissive_15():
    """
        This will fail because of 1 tests which fails - 5.1.5
        :return:
        """
    client, cluster = get_user_client_and_cluster()
    scan_detail = run_scan(cluster, USER_TOKEN, "permissive",
                           scan_tool_version="rke-cis-1.5")
    report_link = scan_detail["links"]["report"]
    test_total, tests_passed, tests_skipped, tests_failed, tests_na = \
        get_scan_results("rke-cis-1.5", "permissive")
    verify_cis_scan_report(
        report_link, token=USER_TOKEN,
        test_total=test_total,
        tests_passed=tests_passed,
        tests_skipped=tests_skipped,
        tests_failed=tests_failed,
        tests_na=tests_na
    )


def test_cis_scan_skip_test_ui():
    client = get_user_client()
    cluster = cluster_detail["cluster_14"]
    # run security scan
    scan_detail = run_scan(cluster, USER_TOKEN, "hardened")
    report_link = scan_detail["links"]["report"]
    test_total, tests_passed, tests_skipped, tests_failed, tests_na = \
        get_scan_results("rke-cis-1.4", "hardened")
    verify_cis_scan_report(
        report_link, token=USER_TOKEN,
        test_total=test_total,
        tests_passed=tests_passed,
        tests_skipped=tests_skipped,
        tests_failed=tests_failed,
        tests_na=tests_na
    )

    # get system project
    system_project = cluster.projects(name="System")["data"][0]
    system_project_id = system_project["id"]
    print(system_project)
    p_client = get_project_client_for_token(system_project, USER_TOKEN)

    # check config map is NOT generated for first scan
    try:
        p_client.list_configMap(projectId=system_project_id,
                                namespaceId="security-scan")
    except ApiError as e:
        assert e.error.status == 404, "Config Map is generated for first scan"

    # delete security-scan-cf config if present
    security_scan_config = \
        p_client.list_configMap(projectId=system_project_id,
                                namespaceId="security-scan",
                                id="security-scan:security-scan-cfg",
                                name="security-scan-cfg")
    print(security_scan_config)
    if len(security_scan_config["data"]) != 0:
        p_client.delete(security_scan_config["data"][0])
    # skip action as on UI
    cm_data = {"config.json": "{\"skip\":{\"rke-cis-1.4\":[\"1.1.2\"]}}"}
    p_client.create_configMap(projectId=system_project_id,
                              name="security-scan-cfg",
                              namespaceId="security-scan",
                              id="security-scan:security-scan-cfg",
                              data=cm_data)
    # run security scan
    scan_detail_2 = run_scan(cluster, USER_TOKEN, "hardened")
    client.reload(scan_detail_2)
    report_link = scan_detail_2["links"]["report"]
    report = verify_cis_scan_report(
        report_link, token=USER_TOKEN,
        test_total=test_total,
        tests_passed=tests_passed - 1,
        tests_skipped=tests_skipped + 1,
        tests_failed=tests_failed,
        tests_na=tests_na
    )
    print(report["results"][0]["checks"][0]["state"])
    assert report["results"][0]["checks"][0]["state"] == "skip", \
        "State of the test is not as expected"

    """As part of clean up
    delete security-scan-cf config
    """
    security_scan_config = \
        p_client.list_configMap(projectId=system_project_id,
                                namespaceId="security-scan",
                                id="security-scan:security-scan-cfg",
                                name="security-scan-cfg")
    print(security_scan_config)
    if len(security_scan_config["data"]) != 0:
        p_client.delete(security_scan_config["data"][0])


def test_cis_scan_skip_test_api():
    client = get_user_client()
    cluster = cluster_detail["cluster_14"]
    # run security scan
    scan_detail = run_scan(cluster, USER_TOKEN, "hardened")
    report_link = scan_detail["links"]["report"]
    test_total, tests_passed, tests_skipped, tests_failed, tests_na = \
        get_scan_results("rke-cis-1.4", "hardened")
    verify_cis_scan_report(
        report_link, token=USER_TOKEN,
        test_total=test_total,
        tests_passed=tests_passed,
        tests_skipped=tests_skipped,
        tests_failed=tests_failed,
        tests_na=tests_na
    )

    # skip test 1.1.3
    cluster.runSecurityScan(overrideSkip=["1.1.3"],
                            profile="hardened",
                            overrideBenchmarkVersion="rke-cis-1.4")
    cluster = client.reload(cluster)
    cluster_scan_report_id = cluster["currentCisRunName"]
    print(cluster_scan_report_id)
    scan_detail = wait_for_scan_active(cluster_scan_report_id, client)
    wait_for_cis_pod_remove(cluster, cluster_scan_report_id)
    report_link = scan_detail["links"]["report"]
    report = verify_cis_scan_report(
        report_link, token=USER_TOKEN,
        test_total=test_total,
        tests_passed=tests_passed - 1,
        tests_skipped=tests_skipped + 1,
        tests_failed=tests_failed,
        tests_na=tests_na
    )
    assert report["results"][0]["checks"][1]["state"] == "skip", \
        "State of the test is not as expected"


def test_cis_scan_edit_cluster():
    aws_nodes = cluster_detail["nodes_14"]
    client = get_user_client()
    cluster = cluster_detail["cluster_14"]
    # Add 2 etcd nodes to the cluster
    for i in range(0, 2):
        aws_node = aws_nodes[3 + i]
        aws_node.execute_command("sudo sysctl -w vm.overcommit_memory=1")
        aws_node.execute_command("sudo sysctl -w kernel.panic=10")
        aws_node.execute_command("sudo sysctl -w kernel.panic_on_oops=1")
        docker_run_cmd = get_custom_host_registration_cmd(client,
                                                          cluster,
                                                          ["etcd"],
                                                          aws_node)
        aws_node.execute_command(docker_run_cmd)
    wait_for_cluster_node_count(client, cluster, 5)
    validate_cluster_state(client, cluster, intermediate_state="updating")
    cluster = client.reload(cluster)

    # run CIS Scan
    scan_detail = run_scan(cluster, USER_TOKEN, "hardened")
    report_link = scan_detail["links"]["report"]
    test_total, tests_passed, tests_skipped, tests_failed, tests_na = \
        get_scan_results("rke-cis-1.4", "hardened")
    report = verify_cis_scan_report(
        report_link, token=USER_TOKEN,
        test_total=test_total,
        tests_passed=tests_passed - 2,
        tests_skipped=tests_skipped,
        tests_failed=tests_failed + 2,
        tests_na=tests_na
    )
    print(report["results"][3]["checks"][18])
    assert report["results"][3]["checks"][18]["state"] == "mixed"

    # edit nodes and run command
    for i in range(0, 2):
        aws_node = aws_nodes[3 + i]
        aws_node.execute_command("sudo useradd etcd")

    # run CIS Scan
    scan_detail = run_scan(cluster, USER_TOKEN, "hardened")
    report_link = scan_detail["links"]["report"]
    report = verify_cis_scan_report(
        report_link, token=USER_TOKEN,
        test_total=test_total,
        tests_passed=tests_passed,
        tests_skipped=tests_skipped,
        tests_failed=tests_failed,
        tests_na=tests_na
    )
    print(report["results"][3]["checks"][18]["state"])
    assert report["results"][3]["checks"][18]["state"] == "pass"


@if_test_rbac
@pytest.mark.parametrize("role", [CLUSTER_OWNER, RESTRICTED_ADMIN])
def test_rbac_run_scan_1(role):
    client, cluster = get_user_client_and_cluster()
    user_token = rbac_get_user_token_by_role(role)
    # run a permissive scan run
    scan_detail = run_scan(cluster, user_token)
    report_link = scan_detail["links"]["report"]
    test_total, tests_passed, tests_skipped, tests_failed, tests_na = \
        get_scan_results("rke-cis-1.4", "permissive")
    verify_cis_scan_report(
        report_link, token=USER_TOKEN,
        test_total=test_total,
        tests_passed=tests_passed,
        tests_skipped=tests_skipped,
        tests_failed=tests_failed,
        tests_na=tests_na
    )


@if_test_rbac
@pytest.mark.parametrize("role", [CLUSTER_MEMBER, PROJECT_MEMBER,
                                  PROJECT_OWNER, PROJECT_READ_ONLY])
def test_rbac_run_scan_2(role):
    client, cluster = get_user_client_and_cluster()
    user_token = rbac_get_user_token_by_role(role)
    run_scan(cluster, user_token, can_run_scan=False)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client = get_user_client()
    # create cluster for running rke-cis-1.4
    cluster_14, aws_nodes_14 = create_cluster_cis()
    cluster_detail["cluster_14"] = cluster_14
    cluster_detail["nodes_14"] = aws_nodes_14

    # create cluster for running rke-cis-1.5
    cluster_15, aws_nodes_15 = create_cluster_cis("rke-cis-1.5")
    cluster_detail["cluster_15"] = cluster_15
    cluster_detail["nodes_15"] = aws_nodes_15

    def fin():
        cluster_cleanup(client, cluster_14, aws_nodes_14)
        cluster_cleanup(client, cluster_15, aws_nodes_15)

    request.addfinalizer(fin)


def verify_cis_scan_report(
        report_link, token, test_total,
        tests_passed, tests_skipped,
        tests_failed, tests_na):
    head = {'Authorization': 'Bearer ' + token}
    response = requests.get(report_link, verify=False, headers=head)
    report = response.json()
    assert report["total"] == test_total, \
        "Incorrect number of tests run"
    assert report["pass"] == tests_passed, \
        "Incorrect number of tests passed"
    assert report["fail"] == tests_failed, \
        "Incorrect number of failed tests"
    assert report["skip"] == tests_skipped, \
        "Incorrect number of tests skipped"
    assert report["notApplicable"] == tests_na, \
        "Incorrect number of tests marked Not Applicable"
    return report


def run_scan(cluster, user_token, profile="permissive",
             can_run_scan=True, scan_tool_version=CIS_SCAN_PROFILE):
    client = get_client_for_token(user_token)
    cluster = get_cluster_by_name(client, cluster.name)
    if can_run_scan:
        cluster.runSecurityScan(profile=profile,
                                overrideBenchmarkVersion=scan_tool_version)
        cluster = client.reload(cluster)
        cluster_scan_report_id = cluster["currentCisRunName"]
        print(cluster_scan_report_id)
        scan_detail = wait_for_scan_active(cluster_scan_report_id, client)
        wait_for_cis_pod_remove(cluster, cluster_scan_report_id)
        return scan_detail
    else:
        assert "runSecurityScan" not in list(cluster.actions.keys()), \
            "User has Run CIS Scan permission"


def wait_for_scan_active(cluster_scan_report_id,
                         client,
                         timeout=DEFAULT_TIMEOUT):
    scan_detail_data = client.list_clusterScan(name=cluster_scan_report_id)
    scan_detail = scan_detail_data.data[0]
    # wait until scan is active
    start = time.time()
    state_scan = scan_detail["state"]
    while state_scan != "pass" and state_scan != "fail":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state of scan report to get to active")
        time.sleep(.5)
        scan_detail_data = client.list_clusterScan(name=cluster_scan_report_id)
        scan_detail = scan_detail_data.data[0]
        state_scan = scan_detail["state"]
        print(state_scan)
    scan_detail_data = client.list_clusterScan(name=cluster_scan_report_id)
    scan_detail = scan_detail_data.data[0]
    return scan_detail


def wait_for_cis_pod_remove(cluster,
                            cluster_scan_report_id,
                            timeout=DEFAULT_TIMEOUT):
    system_project = cluster.projects(name="System")["data"][0]
    p_client = get_project_client_for_token(system_project, USER_TOKEN)
    pod = p_client.list_pod(namespaceId="security-scan",
                            name="security-scan-runner-" +
                                 cluster_scan_report_id)
    start = time.time()
    while len(pod["data"]) != 0:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for removal of security scan pod")
        time.sleep(.5)
        pod = p_client.list_pod(namespaceId="security-scan",
                                name="security-scan-runner-" +
                                     cluster_scan_report_id)
        time.sleep(.5)


def create_cluster_cis(scan_tool_version="rke-cis-1.4"):
    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            5, random_test_name(HOST_NAME))
    node_roles = [
        ["controlplane"], ["etcd"], ["worker"]
    ]
    rke_config_temp = get_cis_rke_config(profile=scan_tool_version)
    client = get_user_client()
    cluster = client.create_cluster(
        name=random_test_name(),
        driver="rancherKubernetesEngine",
        rancherKubernetesEngineConfig=rke_config_temp,
        defaultPodSecurityPolicyTemplateId=POD_SECURITY_POLICY_TEMPLATE
    )
    assert cluster.state == "provisioning"
    # In the original design creates 5 nodes but only 3 are used
    # the other 2 nodes are for test_cis_scan_edit_cluster
    cluster = configure_cis_requirements(aws_nodes[:3],
                                         scan_tool_version,
                                         node_roles,
                                         client,
                                         cluster
                                         )
    return cluster, aws_nodes


def get_scan_results(scan_tool_version, profile):
    return scan_results[scan_tool_version]["total"], \
           scan_results[scan_tool_version][profile]["pass"], \
           scan_results[scan_tool_version][profile]["skip"], \
           scan_results[scan_tool_version]["fail"], \
           scan_results[scan_tool_version]["not_applicable"]
