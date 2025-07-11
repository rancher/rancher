from ..common import *  # NOQA
import inspect
import json
import os
import random
import subprocess
import ssl
import time
import requests
import ast
import paramiko
import rancher
import pytest
from urllib.parse import urlparse
from rancher import ApiError
from lib.aws import AmazonWebServices
from copy import deepcopy
from threading import Lock
from threading import Thread
import websocket
import base64

DEFAULT_CATALOG_TIMEOUT = 15
DEFAULT_MONITORING_TIMEOUT = 180
DEFAULT_CLUSTER_STATE_TIMEOUT = 320
DEFAULT_MULTI_CLUSTER_APP_TIMEOUT = 300
DEFAULT_APP_DELETION_TIMEOUT = 360
DEFAULT_APP_V2_TIMEOUT = 60

CATTLE_API_URL = CATTLE_TEST_URL + "/v3"
CATTLE_AUTH_URL = \
    CATTLE_TEST_URL + "/v3-public/localproviders/local?action=login"

DNS_REGEX = "(https*://)(.*[^/])"

USER_PASSWORD = os.environ.get('USER_PASSWORD', "None")
ADMIN_PASSWORD = os.environ.get('ADMIN_PASSWORD', "None")

kube_fname = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                          "k8s_kube_config")
MACHINE_TIMEOUT = float(os.environ.get('RANCHER_MACHINE_TIMEOUT', "1200"))

HARDENED_CLUSTER = ast.literal_eval(
    os.environ.get('RANCHER_HARDENED_CLUSTER', "False"))
TEST_OS = os.environ.get('RANCHER_TEST_OS', "linux")
TEST_IMAGE = os.environ.get(
    'RANCHER_TEST_IMAGE', "ranchertest/mytestcontainer")
TEST_IMAGE_PORT = os.environ.get('RANCHER_TEST_IMAGE_PORT', "80")
TEST_IMAGE_REDIS = os.environ.get('RANCHER_TEST_IMAGE_REDIS', "redis:latest")
TEST_IMAGE_OS_BASE = os.environ.get('RANCHER_TEST_IMAGE_OS_BASE', "ubuntu")
if TEST_OS == "windows":
    DEFAULT_TIMEOUT = 300
skip_test_windows_os = pytest.mark.skipif(
    TEST_OS == "windows",
    reason='Tests Skipped for including Windows nodes cluster')
skip_test_hardened = pytest.mark.skipif(
    HARDENED_CLUSTER,
    reason='Tests Skipped due to being a hardened cluster')

UPDATE_KDM = ast.literal_eval(os.environ.get('RANCHER_UPDATE_KDM', "False"))
KDM_URL = os.environ.get("RANCHER_KDM_URL", "")
CLUSTER_NAME = os.environ.get("RANCHER_CLUSTER_NAME", "")
RANCHER_CLEANUP_CLUSTER = \
    ast.literal_eval(os.environ.get('RANCHER_CLEANUP_CLUSTER', "True"))
env_file = os.path.join(
    os.path.dirname(os.path.realpath(__file__)),
    "rancher_env.config")

AWS_SSH_KEY_NAME = os.environ.get("AWS_SSH_KEY_NAME")
AWS_ACCESS_KEY_ID = os.environ.get("AWS_ACCESS_KEY_ID")
AWS_SECRET_ACCESS_KEY = os.environ.get("AWS_SECRET_ACCESS_KEY")
AWS_REGION = os.environ.get("AWS_REGION")
AWS_SUBNET = os.environ.get("AWS_SUBNET")
AWS_VPC = os.environ.get("AWS_VPC")
AWS_SG = os.environ.get("AWS_SG")
AWS_ZONE = os.environ.get("AWS_ZONE")
AWS_IAM_PROFILE = os.environ.get("AWS_IAM_PROFILE", "")
AWS_S3_BUCKET_NAME = os.environ.get("AWS_S3_BUCKET_NAME", "")
AWS_S3_BUCKET_FOLDER_NAME = os.environ.get("AWS_S3_BUCKET_FOLDER_NAME", "")
LINODE_ACCESSKEY = os.environ.get('RANCHER_LINODE_ACCESSKEY', "None")
NFS_SERVER_MOUNT_PATH = "/nfs"

TEST_RBAC = ast.literal_eval(os.environ.get('RANCHER_TEST_RBAC', "False"))
if_test_rbac = pytest.mark.skipif(TEST_RBAC is False,
                                  reason='rbac tests are skipped')

TEST_ALL_SNAPSHOT = ast.literal_eval(
    os.environ.get('RANCHER_TEST_ALL_SNAPSHOT', "False")
)
if_test_all_snapshot = \
    pytest.mark.skipif(TEST_ALL_SNAPSHOT is False,
                       reason='Snapshots check tests are skipped')
DATA_SUBDIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           'resource')

# here are all supported roles for RBAC testing
CLUSTER_MEMBER = "cluster-member"
CLUSTER_OWNER = "cluster-owner"
PROJECT_MEMBER = "project-member"
PROJECT_OWNER = "project-owner"
PROJECT_READ_ONLY = "read-only"

rbac_data = {
    "project": None,
    "namespace": None,
    "workload": None,
    "p_unshared": None,
    "ns_unshared": None,
    "wl_unshared": None,
    "users": {
        CLUSTER_OWNER: {},
        CLUSTER_MEMBER: {},
        PROJECT_OWNER: {},
        PROJECT_MEMBER: {},
        PROJECT_READ_ONLY: {},
    }
}

auth_rbac_data = {
    "project": None,
    "namespace": None,
    "users": {}
}

# here are the global role templates used for
# testing globalRoleBinding and groupRoleBinding
TEMPLATE_MANAGE_CATALOG = {
    "newUserDefault": "false",
    "rules": [
        {
            "type": "/v3/schemas/policyRule",
            "apiGroups": [
                "management.cattle.io"
            ],
            "verbs": [
                "*"
            ],
            "resources": [
                "catalogs",
                "templates",
                "templateversions"
            ]
        }
    ],
    "name": "gr-test-manage-catalog",
}

TEMPLATE_LIST_CLUSTER = {
    "newUserDefault": "false",
    "rules": [
        {
            "type": "/v3/schemas/policyRule",
            "apiGroups": [
                "management.cattle.io"
            ],
            "verbs": [
                "get",
                "list",
                "watch"
            ],
            "resources": [
                "clusters"
            ]
        }
    ],
    "name": "gr-test-list-cluster",
}

# this is used when testing users from a auth provider
AUTH_PROVIDER = os.environ.get('RANCHER_AUTH_PROVIDER', "")
if AUTH_PROVIDER not in ["activeDirectory", "freeIpa", "openLdap", ""]:
    pytest.fail("Invalid RANCHER_AUTH_PROVIDER. Please provide one of: "
                "activeDirectory, freeIpa, or openLdap (case sensitive).")
NESTED_GROUP_ENABLED = ast.literal_eval(
    os.environ.get('RANCHER_NESTED_GROUP_ENABLED', "False"))
# Admin Auth username and the shared password for all auth users
AUTH_USER_PASSWORD = os.environ.get('RANCHER_AUTH_USER_PASSWORD', "")

# the link to log in as an auth user
LOGIN_AS_AUTH_USER_URL = \
    CATTLE_TEST_URL + "/v3-public/" \
    + AUTH_PROVIDER + "Providers/" \
    + AUTH_PROVIDER.lower() + "?action=login"
CATTLE_AUTH_PRINCIPAL_URL = CATTLE_TEST_URL + "/v3/principals?action=search"

# This is used for nested group when a third part Auth is enabled
nested_group = {
    "auth_info": None,
    "users": None,
    "group_dic": None,
    "groups": None
}
auth_requirements = not AUTH_PROVIDER or not AUTH_USER_PASSWORD
if_test_group_rbac = pytest.mark.skipif(
    auth_requirements,
    reason='Group RBAC tests are skipped.'
           'Required AUTH env variables '
           'have not been set.'
)

# -----------------------------------------------------------------------------
# global variables from test_create_ha.py
test_run_id = "test" + str(random.randint(10000, 99999))
RANCHER_HOSTNAME_PREFIX = os.environ.get("RANCHER_HOSTNAME_PREFIX",
                                         test_run_id)
CERT_MANAGER_VERSION = os.environ.get("RANCHER_CERT_MANAGER_VERSION", "v1.0.1")
# -----------------------------------------------------------------------------

# this is used for testing rbac v2
test_rbac_v2 = os.environ.get("RANCHER_TEST_RBAC_V2", "False")
if_test_rbac_v2 = pytest.mark.skipif(test_rbac_v2 != "True",
                                     reason='test for rbac v2 is skipped')


def is_windows(os_type=TEST_OS):
    return os_type == "windows"


def get_cluster_client_for_token_v1(cluster_id, token):
    url = CATTLE_TEST_URL + "/k8s/clusters/" + cluster_id + "/v1/schemas"
    return rancher.Client(url=url, token=token, verify=False)


def get_admin_client():
    return rancher.Client(url=CATTLE_API_URL, token=ADMIN_TOKEN, verify=False)


def get_user_client():
    return rancher.Client(url=CATTLE_API_URL, token=USER_TOKEN, verify=False)


def get_client_for_token(token, url=CATTLE_API_URL):
    return rancher.Client(url=url, token=token, verify=False)


def get_project_client_for_token(project, token):
    p_url = project.links['self'] + '/schemas'
    p_client = rancher.Client(url=p_url, token=token, verify=False)
    return p_client


def get_cluster_client_for_token(cluster, token):
    c_url = cluster.links['self'] + '/schemas'
    c_client = rancher.Client(url=c_url, token=token, verify=False)
    return c_client


def up(cluster, token):
    c_url = cluster.links['self'] + '/schemas'
    c_client = rancher.Client(url=c_url, token=token, verify=False)
    return c_client


def wait_state(client, obj, state, timeout=DEFAULT_TIMEOUT):
    wait_for(lambda: client.reload(obj).state == state, timeout)
    return client.reload(obj)


def wait_for_condition(client, resource, check_function, fail_handler=None,
                       timeout=DEFAULT_TIMEOUT):
    start = time.time()
    resource = client.reload(resource)
    while not check_function(resource):
        if time.time() - start > timeout:
            exceptionMsg = 'Timeout waiting for ' + resource.baseType + \
                           ' to satisfy condition: ' + \
                           inspect.getsource(check_function)
            if fail_handler:
                exceptionMsg = exceptionMsg + fail_handler(resource)
            raise Exception(exceptionMsg)
        time.sleep(.5)
        resource = client.reload(resource)
    return resource


def get_setting_value_by_name(name):
    settings_url = CATTLE_API_URL + "/settings/" + name
    head = {'Authorization': 'Bearer ' + ADMIN_TOKEN}
    response = requests.get(settings_url, verify=False, headers=head)
    return response.json()["value"]


# Return value is negative if v1 < v2, zero if v1 == v2 and positive if v1 > v2
def compare_versions(v1, v2):
    if tuple(map(int, (v1.split(".")))) > tuple(map(int, (v2.split(".")))):
        return 1
    elif tuple(map(int, (v1.split(".")))) < tuple(map(int, (v2.split(".")))):
        return -1
    else:
        return 0


def create_project_and_ns(token, cluster, project_name=None, ns_name=None):
    server_url = cluster.links['self'].split("/clusters")[0]
    client = get_client_for_token(token, server_url)
    p = create_project(client, cluster, project_name)
    c_client = get_cluster_client_for_token(cluster, token)
    ns = create_ns(c_client, cluster, p, ns_name)
    return p, ns


def create_project(client, cluster, project_name=None):
    if project_name is None:
        project_name = random_name()
    p = client.create_project(name=project_name,
                              clusterId=cluster.id)
    time.sleep(5)
    p = wait_until_available(client, p)
    assert p.state == 'active'
    return p


def create_project_with_pspt(client, cluster, pspt):
    p = client.create_project(name=random_name(),
                              clusterId=cluster.id)
    p = wait_until_available(client, p)
    assert p.state == 'active'
    return set_pspt_for_project(p, client, pspt)


def set_pspt_for_project(project, client, pspt):
    project.setpodsecuritypolicytemplate(podSecurityPolicyTemplateId=pspt.id)
    project = wait_until_available(client, project)
    assert project.state == 'active'
    return project


def create_ns(client, cluster, project, ns_name=None):
    if ns_name is None:
        ns_name = random_name()
    ns = client.create_namespace(name=ns_name,
                                 clusterId=cluster.id,
                                 projectId=project.id)
    wait_for_ns_to_become_active(client, ns)
    ns = client.reload(ns)
    assert ns.state == 'active'
    return ns


def assign_members_to_cluster(client, user, cluster, role_template_id):
    crtb = client.create_cluster_role_template_binding(
        clusterId=cluster.id,
        roleTemplateId=role_template_id,
        subjectKind="User",
        userId=user.id)
    return crtb


def assign_members_to_project(client, user, project, role_template_id):
    prtb = client.create_project_role_template_binding(
        projectId=project.id,
        roleTemplateId=role_template_id,
        subjectKind="User",
        userId=user.id)
    return prtb


def change_member_role_in_cluster(client, user, crtb, role_template_id):
    client.delete(crtb)
    crtb = client.create_cluster_role_template_binding(
        clusterId=crtb.clusterId,
        roleTemplateId=role_template_id,
        subjectKind="User",
        userId=user.id
    )
    return crtb


def change_member_role_in_project(client, user, prtb, role_template_id):
    client.delete(prtb)
    prtb = client.create_project_role_template_binding(
        projectId=prtb.projectId,
        roleTemplateId=role_template_id,
        subjectKind="User",
        userId=user.id
    )
    return prtb


def create_kubeconfig(cluster, file_name=kube_fname):
    generateKubeConfigOutput = cluster.generateKubeconfig()
    print(generateKubeConfigOutput.config)
    file = open(file_name, "w")
    file.write(generateKubeConfigOutput.config)
    file.close()


def validate_psp_error_worklaod(p_client, workload, error_message):
    workload = wait_for_wl_transitioning(p_client, workload)
    assert workload.state == "updating"
    assert workload.transitioning == "error"
    print(workload.transitioningMessage)
    assert error_message in workload.transitioningMessage


def validate_all_workload_image_from_rancher(project_client, ns, pod_count=1,
                                             ignore_pod_count=False,
                                             deployment_list=None,
                                             daemonset_list=None,
                                             cronjob_list=None, job_list=None):
    if cronjob_list is None:
        cronjob_list = []
    if daemonset_list is None:
        daemonset_list = []
    if deployment_list is None:
        deployment_list = []
    if job_list is None:
        job_list = []
    workload_list = deployment_list + daemonset_list + cronjob_list + job_list

    wls = [dep.name for dep in project_client.list_workload(
        namespaceId=ns.id).data]
    assert len(workload_list) == len(wls), \
        "Expected {} workload(s) to be present in {} namespace " \
        "but there were {}".format(len(workload_list), ns.name, len(wls))

    for workload_name in workload_list:
        workloads = project_client.list_workload(name=workload_name,
                                                 namespaceId=ns.id).data
        assert len(workloads) == workload_list.count(workload_name), \
            "Expected {} workload(s) to be present with name {} " \
            "but there were {}".format(workload_list.count(workload_name),
                                       workload_name, len(workloads))
        for workload in workloads:
            for container in workload.containers:
                assert str(container.image).startswith("rancher/")
            if workload_name in deployment_list:
                validate_workload(project_client, workload, "deployment",
                                  ns.name, pod_count=pod_count,
                                  ignore_pod_count=ignore_pod_count)
                deployment_list.remove(workload_name)
            if workload_name in daemonset_list:
                validate_workload(project_client, workload, "daemonSet",
                                  ns.name, pod_count=pod_count,
                                  ignore_pod_count=ignore_pod_count)
                daemonset_list.remove(workload_name)
            if workload_name in cronjob_list:
                validate_workload(project_client, workload, "cronJob",
                                  ns.name, pod_count=pod_count,
                                  ignore_pod_count=ignore_pod_count)
                cronjob_list.remove(workload_name)
            if workload_name in job_list:
                validate_workload(project_client, workload, "job",
                                  ns.name, pod_count=pod_count,
                                  ignore_pod_count=ignore_pod_count)
                job_list.remove(workload_name)
    # Final assertion to ensure all expected workloads have been validated
    assert not deployment_list + daemonset_list + cronjob_list


def validate_workload(p_client, workload, type, ns_name, pod_count=1,
                      wait_for_cron_pods=60, ignore_pod_count=False):
    workload = wait_for_wl_to_active(p_client, workload)
    assert workload.state == "active"
    # For cronjob, wait for the first pod to get created after
    # scheduled wait time
    if type == "cronJob":
        time.sleep(wait_for_cron_pods)
    if ignore_pod_count:
        pods = p_client.list_pod(workloadId=workload.id).data
    else:
        pods = wait_for_pods_in_workload(p_client, workload, pod_count)
        assert len(pods) == pod_count
        pods = p_client.list_pod(workloadId=workload.id).data
        assert len(pods) == pod_count
    for pod in pods:
        if type == "job":
            job_type = True
            expected_status = "Succeeded"
        else:
            job_type = False
            expected_status = "Running"
        p = wait_for_pod_to_running(p_client, pod, job_type=job_type)
        assert p["status"]["phase"] == expected_status

    wl_result = execute_kubectl_cmd(
        "get " + type + " " + workload.name + " -n " + ns_name)
    if type == "deployment" or type == "statefulSet":
        assert wl_result["status"]["readyReplicas"] == len(pods)
    if type == "daemonSet":
        assert wl_result["status"]["currentNumberScheduled"] == len(pods)
    if type == "cronJob":
        assert len(wl_result["status"]["active"]) >= len(pods)
    if type == "job":
        assert wl_result["status"]["succeeded"] == len(pods)


def validate_workload_with_sidekicks(p_client, workload, type, ns_name,
                                     pod_count=1):
    workload = wait_for_wl_to_active(p_client, workload)
    assert workload.state == "active"
    pods = wait_for_pods_in_workload(p_client, workload, pod_count)
    assert len(pods) == pod_count
    for pod in pods:
        wait_for_pod_to_running(p_client, pod)
    wl_result = execute_kubectl_cmd(
        "get " + type + " " + workload.name + " -n " + ns_name)
    assert wl_result["status"]["readyReplicas"] == pod_count
    for key, value in workload.workloadLabels.items():
        label = key + "=" + value
    get_pods = "get pods -l" + label + " -n " + ns_name
    execute_kubectl_cmd(get_pods)
    pods_result = execute_kubectl_cmd(get_pods)
    assert len(pods_result["items"]) == pod_count
    for pod in pods_result["items"]:
        assert pod["status"]["phase"] == "Running"
        assert len(pod["status"]["containerStatuses"]) == 2
        assert "running" in pod["status"]["containerStatuses"][0]["state"]
        assert "running" in pod["status"]["containerStatuses"][1]["state"]


def validate_workload_paused(p_client, workload, expectedstatus):
    workloadStatus = p_client.list_workload(uuid=workload.uuid).data[0].paused
    assert workloadStatus == expectedstatus


def validate_pod_images(expectedimage, workload, ns_name):
    for key, value in workload.workloadLabels.items():
        label = key + "=" + value
    get_pods = "get pods -l" + label + " -n " + ns_name
    pods = execute_kubectl_cmd(get_pods)

    for pod in pods["items"]:
        assert pod["spec"]["containers"][0]["image"] == expectedimage


def validate_pods_are_running_by_id(expectedpods, workload, ns_name):
    for key, value in workload.workloadLabels.items():
        label = key + "=" + value
    get_pods = "get pods -l" + label + " -n " + ns_name
    pods = execute_kubectl_cmd(get_pods)

    curpodnames = []
    for pod in pods["items"]:
        curpodnames.append(pod["metadata"]["name"])

    for expectedpod in expectedpods["items"]:
        assert expectedpod["metadata"]["name"] in curpodnames


def validate_workload_image(client, workload, expectedImage, ns):
    workload = client.list_workload(uuid=workload.uuid).data[0]
    assert workload.containers[0].image == expectedImage
    validate_pod_images(expectedImage, workload, ns.name)


def execute_kubectl_cmd(cmd, json_out=True, stderr=False,
                        kubeconfig=kube_fname):
    command = 'kubectl --kubeconfig {0} {1}'.format(
        kubeconfig, cmd)
    if json_out:
        command += ' -o json'
    print("run cmd: \t{0}".format(command))

    if stderr:
        result = run_command_with_stderr(command, False)
    else:
        result = run_command(command, False)
    print("returns: \t{0}".format(result))

    if json_out:
        result = json.loads(result)
    return result


def run_command(command, log_out=True):
    if log_out:
        print("run cmd: \t{0}".format(command))

    try:
        return subprocess.check_output(command, shell=True, text=True)
    except subprocess.CalledProcessError as e:
        return None


def run_command_with_stderr(command, log_out=True):
    if log_out:
        print("run cmd: \t{0}".format(command))

    try:
        output = subprocess.check_output(command, shell=True,
                                         stderr=subprocess.PIPE)
        returncode = 0
    except subprocess.CalledProcessError as e:
        output = e.stderr
        returncode = e.returncode

    if log_out:
        print("return code: \t{0}".format(returncode))
        if returncode != 0:
            print("output: \t{0}".format(output))

    return output


def wait_for_wl_to_active(client, workload, timeout=DEFAULT_TIMEOUT):
    start = time.time()
    timeout = start + timeout
    workloads = client.list_workload(uuid=workload.uuid).data
    assert len(workloads) == 1
    wl = workloads[0]
    while wl.state != "active":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        workloads = client.list_workload(uuid=workload.uuid).data
        assert len(workloads) == 1
        wl = workloads[0]
    return wl


def wait_for_ingress_to_active(client, ingress, timeout=DEFAULT_TIMEOUT):
    start = time.time()
    ingresses = client.list_ingress(uuid=ingress.uuid).data
    assert len(ingresses) == 1
    wl = ingresses[0]
    while wl.state != "active":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        ingresses = client.list_ingress(uuid=ingress.uuid).data
        assert len(ingresses) == 1
        wl = ingresses[0]
    return wl


def wait_for_wl_transitioning(client, workload, timeout=DEFAULT_TIMEOUT,
                              state="error"):
    start = time.time()
    workloads = client.list_workload(uuid=workload.uuid).data
    assert len(workloads) == 1
    wl = workloads[0]
    while wl.transitioning != state:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        workloads = client.list_workload(uuid=workload.uuid).data
        assert len(workloads) == 1
        wl = workloads[0]
    return wl


def wait_for_pod_to_running(client, pod, timeout=DEFAULT_TIMEOUT, job_type=False):
    start = time.time()
    pods = client.list_pod(uuid=pod.uuid).data
    assert len(pods) == 1
    p = pods[0]
    if job_type:
        expected_state = "succeeded"
    else:
        expected_state = "running"
    while p.state != expected_state:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        pods = client.list_pod(uuid=pod.uuid).data
        assert len(pods) == 1
        p = pods[0]
    return p


def get_schedulable_nodes(cluster, client=None, os_type=TEST_OS):
    if not client:
        client = get_user_client()
    nodes = client.list_node(clusterId=cluster.id).data
    schedulable_nodes = []
    for node in nodes:
        if not node.unschedulable:
            shouldSchedule = True
            # node.taints doesn't exist if the node has no taints.
            try:
                for tval in node.taints:
                    if str(tval).find("PreferNoSchedule") == -1:
                        if str(tval).find("NoExecute") > -1 or str(tval).find("NoSchedule") > -1:
                            shouldSchedule = False
                            break
            except AttributeError:
                pass
            if not shouldSchedule:
                continue
            for key, val in node.labels.items():
                # Either one of the labels should be present on the node
                if key == 'kubernetes.io/os' or key == 'beta.kubernetes.io/os':
                    if val == os_type:
                        schedulable_nodes.append(node)
                        break
    return schedulable_nodes


def get_etcd_nodes(cluster, client=None):
    if not client:
        client = get_user_client()
    nodes = client.list_node(clusterId=cluster.id).data
    etcd_nodes = []
    for node in nodes:
        if node.etcd:
            etcd_nodes.append(node)
    return etcd_nodes


def get_role_nodes(cluster, role, client=None):
    etcd_nodes = []
    control_nodes = []
    worker_nodes = []
    node_list = []
    if not client:
        client = get_user_client()
    nodes = client.list_node(clusterId=cluster.id).data
    for node in nodes:
        if node.etcd:
            etcd_nodes.append(node)
        if node.controlPlane:
            control_nodes.append(node)
        if node.worker:
            worker_nodes.append(node)
    if role == "etcd":
        node_list = etcd_nodes
    if role == "control":
        node_list = control_nodes
    if role == "worker":
        node_list = worker_nodes
    return node_list


def validate_ingress(p_client, cluster, workloads, host, path,
                     insecure_redirect=False):
    time.sleep(10)
    curl_args = " "
    if (insecure_redirect):
        curl_args = " -L --insecure "
    if len(host) > 0:
        curl_args += " --header 'Host: " + host + "'"
    nodes = get_schedulable_nodes(cluster, os_type="linux")
    target_name_list = get_target_names(p_client, workloads)
    for node in nodes:
        host_ip = resolve_node_ip(node)
        url = "http://" + host_ip + path
        if not insecure_redirect:
            wait_until_ok(url, timeout=300, headers={
                "Host": host
            })
        cmd = curl_args + " " + url
        validate_http_response(cmd, target_name_list)


def validate_ingress_using_endpoint(p_client, ingress, workloads,
                                    timeout=300,
                                    certcheck=False, is_insecure=False):
    target_name_list = get_target_names(p_client, workloads)
    start = time.time()
    fqdn_available = False
    url = None
    while not fqdn_available:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for endpoint to be available")
        time.sleep(.5)
        ingress_list = p_client.list_ingress(uuid=ingress.uuid).data
        assert len(ingress_list) == 1
        ingress = ingress_list[0]
        if hasattr(ingress, 'publicEndpoints'):
            for public_endpoint in ingress.publicEndpoints:
                if public_endpoint["hostname"].startswith(ingress.name) \
                        or certcheck:
                    fqdn_available = True
                    url = \
                        public_endpoint["protocol"].lower() + "://" + \
                        public_endpoint["hostname"]
                    if "path" in public_endpoint.keys():
                        url += public_endpoint["path"]
    time.sleep(10)
    validate_http_response(url, target_name_list, insecure=is_insecure)


def get_target_names(p_client, workloads):
    pods = []
    for workload in workloads:
        pod_list = p_client.list_pod(workloadId=workload.id).data
        pods.extend(pod_list)
    target_name_list = []
    for pod in pods:
        target_name_list.append(pod.name)
    print("target name list:" + str(target_name_list))
    return target_name_list


def get_endpoint_url_for_workload(p_client, workload, timeout=600):
    fqdn_available = False
    url = ""
    start = time.time()
    while not fqdn_available:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for endpoint to be available")
        time.sleep(.5)
        workload_list = p_client.list_workload(uuid=workload.uuid).data
        assert len(workload_list) == 1
        workload = workload_list[0]
        if hasattr(workload, 'publicEndpoints'):
            assert len(workload.publicEndpoints) > 0
            url = "http://"
            url = url + workload.publicEndpoints[0]["addresses"][0] + ":"
            url = url + str(workload.publicEndpoints[0]["port"])
            fqdn_available = True
    return url


def wait_until_lb_is_active(url, timeout=300):
    start = time.time()
    while check_for_no_access(url):
        time.sleep(.5)
        print("No access yet")
        if time.time() - start > timeout:
            raise Exception('Timed out waiting for LB to become active')
    return


def check_for_no_access(url, verify=False):
    try:
        requests.get(url, verify=verify)
        return False
    except requests.ConnectionError:
        print("Connection Error - " + url)
        return True


def wait_until_active(url, timeout=120):
    start = time.time()
    while check_for_no_access(url):
        time.sleep(.5)
        print("No access yet")
        if time.time() - start > timeout:
            raise Exception('Timed out waiting for url '
                            'to become active')
    return


def wait_until_ok(url, timeout=120, headers={}):
    start = time.time()
    while not check_if_ok(url, headers=headers):
        time.sleep(.5)
        if time.time() - start > timeout:
            raise Exception(
                'Timed out waiting for {0} to become ok'.format(url)
            )
    return


def wait_for_status_code(url, expected_code=200, timeout=DEFAULT_TIMEOUT):
    start = time.time()
    r = requests.get(url, verify=False)
    while r.status_code != expected_code:
        time.sleep(1)
        r = requests.get(url, verify=False)
        if time.time() - start > timeout:
            raise Exception(
                'Timed out waiting for status code {0}'
                ', actual code {1}'.format(
                    expected_code, r.status_code
                )
            )
    return


def check_if_ok(url, verify=False, headers={}):
    try:
        res = requests.head(url, verify=verify, headers=headers)
        if res.status_code == 200:
            return True
        return False
    except requests.ConnectionError:
        print("Connection Error - " + url)
        return False

def retry_cmd_validate_expected(pod, cmd, expected, timeout=300):
    start = time.time()
    timeout = start + timeout
    cmd_output = kubectl_pod_exec(pod, cmd)
    decode_cmd = cmd_output.decode('utf-8')
    while time.time() < timeout:
        if any(x in str(cmd_output) for x in expected):
            return decode_cmd
        time.sleep(5)
        cmd_output = kubectl_pod_exec(pod, cmd)
        decode_cmd = cmd_output.decode('utf-8')
    raise AssertionError(
        "Timed out waiting to get expected output")

def validate_http_response(cmd, target_name_list, client_pod=None,
                           insecure=False):
    if client_pod is None and cmd.startswith("http://"):
        wait_until_active(cmd, 60)
    target_hit_list = target_name_list[:]
    while len(target_hit_list) != 0:
        if len(target_hit_list) == 0:
            break
        if client_pod is None:
            curl_cmd = "curl " + cmd
            if insecure:
                curl_cmd += "\t--insecure"
            result = run_command(curl_cmd)
        else:
            if is_windows():
                wget_cmd = 'powershell -NoLogo -NonInteractive -Command ' \
                           '"& {{ (Invoke-WebRequest -UseBasicParsing -Uri ' \
                           '{0}).Content }}"'.format(cmd)
            else:
                wget_cmd = "wget -qO- " + cmd
            time.sleep(6)
            result = retry_cmd_validate_expected(client_pod, wget_cmd, target_name_list)
        if result is not None:
            result = result.rstrip()
            assert result in target_name_list
            if result in target_hit_list:
                target_hit_list.remove(result)
    print("After removing all, the rest is: ", target_hit_list)
    assert len(target_hit_list) == 0


def validate_cluster(client, cluster, intermediate_state="provisioning",
                     check_intermediate_state=True, skipIngresscheck=True,
                     nodes_not_in_active_state=[], k8s_version="",
                     userToken=USER_TOKEN, timeout=MACHINE_TIMEOUT):
    # Allow sometime for the "cluster_owner" CRTB to take effect
    time.sleep(5)
    cluster = validate_cluster_state(
        client, cluster,
        check_intermediate_state=check_intermediate_state,
        intermediate_state=intermediate_state,
        nodes_not_in_active_state=nodes_not_in_active_state,
        timeout=timeout)
    create_kubeconfig(cluster)
    if k8s_version != "":
        check_cluster_version(cluster, k8s_version)
    if hasattr(cluster, 'rancherKubernetesEngineConfig'):
        check_cluster_state(len(get_role_nodes(cluster, "etcd", client)))
    # check all workloads under the system project are active
    # wait for workloads to be active
    # time.sleep(DEFAULT_TIMEOUT)
    print("checking if workloads under the system project are active")
    sys_project = client.list_project(name='System',
                                      clusterId=cluster.id).data[0]
    sys_p_client = get_project_client_for_token(sys_project, userToken)
    for wl in sys_p_client.list_workload().data:
        """to  help run KDM job faster (when there are many clusters), 
        timeout=300 is set"""
        wait_for_wl_to_active(sys_p_client, wl, timeout=300)
    # Create Daemon set workload and have an Ingress with Workload
    # rule pointing to this daemonSet
    project, ns = create_project_and_ns(userToken, cluster)
    p_client = get_project_client_for_token(project, userToken)
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    validate_workload(p_client, workload, "daemonSet", ns.name,
                      len(get_schedulable_nodes(cluster, client)))
    if not skipIngresscheck:
        pods = p_client.list_pod(workloadId=workload["id"]).data
        scale = len(pods)
        # test service discovery
        validate_service_discovery(workload, scale, p_client, ns, pods)
        host = "test" + str(random_int(10000, 99999)) + ".com"
        path = "/name.html"
        rule = {"host": host,
                "paths":
                    [{"workloadIds": [workload.id],
                      "targetPort": TEST_IMAGE_PORT}]}
        ingress = p_client.create_ingress(name=name,
                                          namespaceId=ns.id,
                                          rules=[rule])
        wait_for_ingress_to_active(p_client, ingress)
        validate_ingress(p_client, cluster, [workload], host, path)
    return cluster


def check_cluster_version(cluster, version):
    cluster_k8s_version = \
        cluster.appliedSpec["rancherKubernetesEngineConfig"][
            "kubernetesVersion"]
    assert cluster_k8s_version == version, \
        "cluster_k8s_version: " + cluster_k8s_version + \
        " Expected: " + version
    expected_k8s_version = version[:version.find("-rancher")]
    k8s_version = execute_kubectl_cmd("version")
    kubectl_k8s_version = k8s_version["serverVersion"]["gitVersion"]
    assert kubectl_k8s_version == expected_k8s_version, \
        "kubectl version: " + kubectl_k8s_version + \
        " Expected: " + expected_k8s_version


def check_cluster_state(etcd_count):
    css_resp = execute_kubectl_cmd("get cs")
    css = css_resp["items"]
    components = ["scheduler", "controller-manager"]
    for i in range(0, etcd_count):
        components.append("etcd-" + str(i))
    print("components to check - " + str(components))
    for cs in css:
        component_name = cs["metadata"]["name"]
        assert component_name in components
        components.remove(component_name)
        assert cs["conditions"][0]["status"] == "True"
        assert cs["conditions"][0]["type"] == "Healthy"
    assert len(components) == 0


def validate_dns_record(pod, record, expected, port=TEST_IMAGE_PORT):
    # requires pod with `dig` available - TEST_IMAGE
    host = '{0}.{1}.svc.cluster.local'.format(
        record["name"], record["namespaceId"])
    validate_dns_entry(pod, host, expected, port=port)

def retry_dig(host, pod, expected, timeout=300):
    start = 0
    while start < timeout:
        dig_cmd = 'dig {0} +short'.format(host)
        dig_output = kubectl_pod_exec(pod, dig_cmd)
        decode_dig = dig_output.decode('utf-8')
        split_dig = decode_dig.splitlines()
        dig_length = len(split_dig)
        expected_length = len(expected)
        if dig_length >= expected_length:
            return dig_output
        start += 5
        time.sleep(5)
    raise AssertionError(
        "Timed out waiting to get expected output")

def validate_dns_entry(pod, host, expected, port=TEST_IMAGE_PORT, retry_count=3):
    if is_windows():
        validate_dns_entry_windows(pod, host, expected)
        return

    # requires pod with `dig` available - TEST_IMAGE
    if HARDENED_CLUSTER:
        cmd = 'curl -vs {}:{} 2>&1'.format(host, port)
    else:
        cmd = 'ping -c 2 -W 2 {0}'.format(host)
    cmd_output = retry_cmd_validate_expected(pod, cmd, expected)

    connectivity_validation_pass = False
    for expected_value in expected:
        if expected_value in str(cmd_output):
            connectivity_validation_pass = True
            break

    assert connectivity_validation_pass is True
    if HARDENED_CLUSTER:
        assert " 200 OK" in str(cmd_output)
    else:
        assert " 0% packet loss" in str(cmd_output)

    dig_output = retry_dig(host, pod, expected)

    for expected_value in expected:
        assert expected_value in str(dig_output), \
            "Error the dig command returned: {0}".format(dig_output)


def validate_dns_entry_windows(pod, host, expected):
    def ping_check():
        ping_cmd = 'ping -w 1 -n 1 {0}'.format(host)
        ping_output = kubectl_pod_exec(pod, ping_cmd)
        ping_validation_pass = False
        for expected_value in expected:
            if expected_value in str(ping_output):
                ping_validation_pass = True
                break
        return ping_validation_pass and (" (0% loss)" in str(ping_output))

    wait_for(callback=ping_check,
             timeout_message="Failed to ping {0}".format(host))

    def dig_check():
        dig_cmd = 'powershell -NoLogo -NonInteractive -Command ' \
                  '"& {{ (Resolve-DnsName {0}).IPAddress }}"'.format(host)
        dig_output = kubectl_pod_exec(pod, dig_cmd)
        dig_validation_pass = True
        for expected_value in expected:
            if expected_value not in str(dig_output):
                dig_validation_pass = False
                break
        return dig_validation_pass

    wait_for(callback=dig_check,
             timeout_message="Failed to resolve {0}".format(host))


def validate_dns_record_deleted(client, dns_record, timeout=DEFAULT_TIMEOUT):
    """
    Checks whether dns_record got deleted successfully.
    Validates if dns_record is null in for current object client.
    @param client: Object client use to create dns_record
    @param dns_record: record object subjected to be deleted
    @param timeout: Max time to keep checking whether record is deleted or not
    """
    time.sleep(2)
    start = time.time()
    records = client.list_dns_record(name=dns_record.name, ).data
    while len(records) != 0:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for record {} to be deleted"
                "".format(dns_record.name))
        time.sleep(.5)
        records = client.list_dns_record(name=dns_record.name, ).data


def wait_for_nodes_to_become_active(client, cluster, exception_list=[],
                                    retry_count=0):
    nodes = client.list_node(clusterId=cluster.id).data
    node_auto_deleted = False
    for node in nodes:
        if node.requestedHostname not in exception_list:
            node = wait_for_node_status(client, node, "active")
            if node is None:
                print("Need to re-evalauate new node list")
                node_auto_deleted = True
                retry_count += 1
                print("Retry Count:" + str(retry_count))
    if node_auto_deleted and retry_count < 5:
        wait_for_nodes_to_become_active(client, cluster, exception_list,
                                        retry_count)


def wait_for_node_status(client, node, state):
    uuid = node.uuid
    start = time.time()
    nodes = client.list_node(uuid=uuid).data
    node_count = len(nodes)
    # Handle the case of nodes getting auto deleted when they are part of
    # nodepools
    if node_count == 1:
        node_status = nodes[0].state
    else:
        print("Node does not exist anymore -" + uuid)
        return None
    while node_status != state:
        if time.time() - start > MACHINE_TIMEOUT:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(5)
        nodes = client.list_node(uuid=uuid).data
        node_count = len(nodes)
        if node_count == 1:
            node_status = nodes[0].state
        else:
            print("Node does not exist anymore -" + uuid)
            return None
    return node


def wait_for_node_to_be_deleted(client, node, timeout=300):
    uuid = node.uuid
    start = time.time()
    nodes = client.list_node(uuid=uuid).data
    node_count = len(nodes)
    while node_count != 0:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for node delete")
        time.sleep(.5)
        nodes = client.list_node(uuid=uuid).data
        node_count = len(nodes)


def wait_for_cluster_node_count(client, cluster, expected_node_count,
                                timeout=300):
    start = time.time()
    nodes = client.list_node(clusterId=cluster.id).data
    node_count = len(nodes)
    while node_count != expected_node_count:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        nodes = client.list_node(clusterId=cluster.id).data
        node_count = len(nodes)


def get_custom_host_registration_cmd(client, cluster, roles, node):
    allowed_roles = ["etcd", "worker", "controlplane"]
    cluster_tokens = client.list_cluster_registration_token(
        clusterId=cluster.id).data
    if len(cluster_tokens) > 0:
        cluster_token = cluster_tokens[0]
    else:
        cluster_token = create_custom_host_registration_token(client, cluster)

    additional_options = " --address " + node.public_ip_address + \
                         " --internal-address " + node.private_ip_address

    if 'Administrator' == node.ssh_user:
        cmd = cluster_token.windowsNodeCommand
        cmd = cmd.replace('| iex', '--worker' + additional_options + ' | iex ')
    else:
        cmd = cluster_token.nodeCommand
        for role in roles:
            assert role in allowed_roles
            cmd += " --" + role

        cmd += additional_options
    return cmd


def create_custom_host_registration_token(client, cluster):
    # Allow sometime for the "cluster_owner" CRTB to take effect
    time.sleep(5)
    cluster_token = client.create_cluster_registration_token(
        clusterId=cluster.id)
    cluster_token = client.wait_success(cluster_token)
    assert cluster_token.state == 'active'
    return cluster_token


def get_cluster_by_name(client, name):
    clusters = client.list_cluster(name=name).data
    assert len(clusters) == 1, "Cluster " + name + " does not exist"
    return clusters[0]


def get_cluster_type(client, cluster):
    cluster_configs = [
        "amazonElasticContainerServiceConfig",
        "azureKubernetesServiceConfig",
        "googleKubernetesEngineConfig",
        "rancherKubernetesEngineConfig"
    ]
    if "rancherKubernetesEngineConfig" in cluster:
        nodes = client.list_node(clusterId=cluster.id).data
        if len(nodes) > 0:
            if nodes[0].nodeTemplateId is None:
                return "Custom"
    for cluster_config in cluster_configs:
        if cluster_config in cluster:
            return cluster_config
    return "Imported"


def delete_cluster(client, cluster):
    nodes = client.list_node(clusterId=cluster.id).data
    # Delete nodes(in cluster) from AWS for Imported and Custom Cluster
    if len(nodes) > 0:
        cluster_type = get_cluster_type(client, cluster)
        print(cluster_type)
        if get_cluster_type(client, cluster) in ["Imported", "Custom"]:
            filters = [
                {'Name': 'tag:Name',
                 'Values': ['testcustom*', 'teststress*', 'testsa*']}]
            ip_filter = {}
            ip_list = []
            ip_filter['Name'] = \
                'network-interface.addresses.association.public-ip'
            ip_filter['Values'] = ip_list
            filters.append(ip_filter)
            for node in nodes:
                host_ip = resolve_node_ip(node)
                ip_list.append(host_ip)
            assert len(ip_filter) > 0
            print(ip_filter)
            aws_nodes = AmazonWebServices().get_nodes(filters)
            if aws_nodes is None:
                # search instances by IPs in case names do not follow patterns
                aws_nodes = AmazonWebServices().get_nodes(filters=[ip_filter])
            if aws_nodes is None:
                print("no instance is found in AWS")
            else:
                for node in aws_nodes:
                    print(node.public_ip_address)
                AmazonWebServices().delete_nodes(aws_nodes)
    # Delete Cluster
    client.delete(cluster)


def check_connectivity_between_workloads(p_client1, workload1, p_client2,
                                         workload2, allow_connectivity=True):
    wl1_pods = p_client1.list_pod(workloadId=workload1.id).data
    wl2_pods = p_client2.list_pod(workloadId=workload2.id).data
    for pod in wl1_pods:
        for o_pod in wl2_pods:
            check_connectivity_between_pods(pod, o_pod, allow_connectivity)


def check_connectivity_between_workload_pods(p_client, workload):
    pods = p_client.list_pod(workloadId=workload.id).data
    for pod in pods:
        for o_pod in pods:
            check_connectivity_between_pods(pod, o_pod)


def check_connectivity_between_pods(pod1, pod2, allow_connectivity=True):
    pod_ip = pod2.status.podIp

    if is_windows():
        cmd = 'ping -w 1 -n 1 {0}'.format(pod_ip)
    elif HARDENED_CLUSTER:
        cmd = 'curl -I {}:{}'.format(pod_ip, TEST_IMAGE_PORT)
    else:
        cmd = "ping -c 1 -W 1 " + pod_ip

    response = retry_cmd_validate_expected(pod1, cmd, pod_ip)
    if not HARDENED_CLUSTER:
        assert pod_ip in str(response)
    if allow_connectivity:
        if is_windows():
            assert " (0% loss)" in str(response)
        elif HARDENED_CLUSTER:
            assert " 200 OK" in str(response)
        else:
            assert " 0% packet loss" in str(response)
    else:
        if is_windows():
            assert " (100% loss)" in str(response)
        elif HARDENED_CLUSTER:
            assert " 200 OK" not in str(response)
        else:
            assert " 100% packet loss" in str(response)


def kubectl_pod_exec(pod, cmd):
    command = "exec " + pod.name + " -n " + pod.namespaceId + " -- " + cmd
    return execute_kubectl_cmd(command, json_out=False, stderr=True)


def exec_shell_command(ip, port, cmd, password, user="root", sshKey=None):
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    if sshKey:
        ssh.connect(ip, username=user, key_filename=sshKey, port=port)
    else:
        ssh.connect(ip, username=user, password=password, port=port)

    stdin, stdout, stderr = ssh.exec_command(cmd)
    response = stdout.readlines()
    return response


def wait_for_ns_to_become_active(client, ns, timeout=DEFAULT_TIMEOUT):
    start = time.time()
    time.sleep(10)
    nss = client.list_namespace(uuid=ns.uuid).data
    assert len(nss) == 1
    ns = nss[0]
    while ns.state != "active":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        nss = client.list_namespace(uuid=ns.uuid).data
        assert len(nss) == 1
        ns = nss[0]
    return ns


def wait_for_pod_images(p_client, workload, ns_name, expectedimage, numofpods,
                        timeout=DEFAULT_TIMEOUT):
    start = time.time()

    for key, value in workload.workloadLabels.items():
        label = key + "=" + value
    get_pods = "get pods -l" + label + " -n " + ns_name
    pods = execute_kubectl_cmd(get_pods)

    for x in range(0, numofpods - 1):
        pod = pods["items"][x]
        podimage = pod["spec"]["containers"][0]["image"]
        while podimage != expectedimage:
            if time.time() - start > timeout:
                raise AssertionError(
                    "Timed out waiting for correct pod images")
            time.sleep(.5)
            pods = execute_kubectl_cmd(get_pods)
            pod = pods["items"][x]
            podimage = pod["spec"]["containers"][0]["image"]


def wait_for_pods_in_workload(p_client, workload, pod_count,
                              timeout=DEFAULT_TIMEOUT):
    start = time.time()
    pods = p_client.list_pod(workloadId=workload.id).data
    while len(pods) != pod_count:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for pods in workload {}. Expected {}. "
                "Got {}".format(workload.name, pod_count, len(pods)))
        time.sleep(.5)
        pods = p_client.list_pod(workloadId=workload.id).data
    return pods


def get_user_client_and_cluster(client=None):
    if not client:
        client = get_user_client()
    if CLUSTER_NAME == "":
        clusters = client.list_cluster().data
    else:
        clusters = client.list_cluster(name=CLUSTER_NAME).data
    assert len(clusters) > 0
    cluster = clusters[0]
    return client, cluster


def get_global_admin_client_and_cluster():
    client = get_admin_client()
    if CLUSTER_NAME == "":
        clusters = client.list_cluster().data
    else:
        clusters = client.list_cluster(name=CLUSTER_NAME).data
    assert len(clusters) > 0
    cluster = clusters[0]
    return client, cluster


def validate_cluster_state(client, cluster,
                           check_intermediate_state=True,
                           intermediate_state="provisioning",
                           nodes_not_in_active_state=[],
                           timeout=MACHINE_TIMEOUT):
    start_time = time.time()
    if check_intermediate_state:
        cluster = wait_for_condition(
            client, cluster,
            lambda x: x.state == intermediate_state,
            lambda x: 'State is: ' + x.state,
            timeout=timeout)
        if intermediate_state != "updating":
            assert cluster.state == intermediate_state
    cluster = wait_for_condition(
        client, cluster,
        lambda x: x.state == "active",
        lambda x: 'State is: ' + x.state,
        timeout=timeout)
    assert cluster.state == "active"
    wait_for_nodes_to_become_active(client, cluster,
                                    exception_list=nodes_not_in_active_state)
    timeout = 60
    start = time.time()
    while "version" not in cluster.keys():
        time.sleep(1)
        cluster = client.reload(cluster)
        delta = time.time() - start
        if delta > timeout:
            msg = "Timeout waiting for K8s version to be synced"
            raise Exception(msg)
    end_time = time.time()
    diff = time.strftime("%H:%M:%S", time.gmtime(end_time - start_time))
    print("The total time for provisioning/updating the cluster {} : {}".
          format(cluster.name, diff))
    return cluster


def wait_until_available(client, obj, timeout=DEFAULT_TIMEOUT):
    start = time.time()
    sleep = 0.01
    while True:
        time.sleep(sleep)
        sleep *= 2
        if sleep > 2:
            sleep = 2
        try:
            obj = client.reload(obj)
        except ApiError as e:
            if e.error.status != 403:
                raise e
        else:
            return obj
        delta = time.time() - start
        if delta > timeout:
            msg = 'Timeout waiting for [{}:{}] for condition after {}' \
                  ' seconds'.format(obj.type, obj.id, delta)
            raise Exception(msg)


def delete_node(aws_nodes):
    for node in aws_nodes:
        AmazonWebServices().delete_node(node)


def cluster_cleanup(client, cluster, aws_nodes=None):
    if RANCHER_CLEANUP_CLUSTER:
        start = time.time()
        client.delete(cluster)
        if cluster.rancherKubernetesEngineConfig.cloudProvider.name == "azure":
            time.sleep(20)
            print("-------sleep time after cluster deletion--------", time.time() - start)
        if aws_nodes is not None:
            delete_node(aws_nodes)
    else:
        env_details = "env.CATTLE_TEST_URL='" + CATTLE_TEST_URL + "'\n"
        env_details += "env.ADMIN_TOKEN='" + ADMIN_TOKEN + "'\n"
        env_details += "env.USER_TOKEN='" + USER_TOKEN + "'\n"
        env_details += "env.CLUSTER_NAME='" + cluster.name + "'\n"
        create_config_file(env_details)
        
        
def hosted_cluster_cleanup(client, cluster, cluster_name):
    if RANCHER_CLEANUP_CLUSTER:
        client.delete(cluster)
    else:
        env_details = "env.CATTLE_TEST_URL='" + CATTLE_TEST_URL + "'\n"
        env_details += "env.ADMIN_TOKEN='" + ADMIN_TOKEN + "'\n"
        env_details += "env.USER_TOKEN='" + USER_TOKEN + "'\n"
        env_details += "env.CLUSTER_NAME='" + cluster_name + "'\n"
        create_config_file(env_details)


def create_config_file(env_details):
    file = open(env_file, "w")
    file.write(env_details)
    file.close()


def validate_hostPort(p_client, workload, source_port, cluster):
    get_endpoint_url_for_workload(p_client, workload)
    wl = p_client.list_workload(uuid=workload.uuid).data[0]
    source_port_wk = wl.publicEndpoints[0]["port"]
    assert source_port == source_port_wk, "Source ports do not match"
    pods = p_client.list_pod(workloadId=workload.id).data
    nodes = get_schedulable_nodes(cluster)
    for node in nodes:
        target_name_list = []
        for pod in pods:
            print(pod.nodeId + " check " + node.id)
            if pod.nodeId == node.id:
                target_name_list.append(pod.name)
                break
        if len(target_name_list) > 0:
            host_ip = resolve_node_ip(node)
            curl_cmd = " http://" + host_ip + ":" + \
                       str(source_port) + "/name.html"
            validate_http_response(curl_cmd, target_name_list)


def validate_lb(p_client, workload, source_port):
    url = get_endpoint_url_for_workload(p_client, workload)
    wl = p_client.list_workload(uuid=workload.uuid).data[0]
    source_port_wk = wl.publicEndpoints[0]["port"]
    assert source_port == source_port_wk, "Source ports do not match"
    target_name_list = get_target_names(p_client, [workload])
    wait_until_lb_is_active(url)
    validate_http_response(url + "/name.html", target_name_list)


def validate_nodePort(p_client, workload, cluster, source_port):
    get_endpoint_url_for_workload(p_client, workload, 600)
    wl = p_client.list_workload(uuid=workload.uuid).data[0]
    source_port_wk = wl.publicEndpoints[0]["port"]
    assert source_port == source_port_wk, "Source ports do not match"
    nodes = get_schedulable_nodes(cluster)
    pods = p_client.list_pod(workloadId=wl.id).data
    target_name_list = []
    for pod in pods:
        target_name_list.append(pod.name)
    print("target name list:" + str(target_name_list))
    for node in nodes:
        host_ip = resolve_node_ip(node)
        curl_cmd = " http://" + host_ip + ":" + \
                   str(source_port_wk) + "/name.html"
        validate_http_response(curl_cmd, target_name_list)


def validate_clusterIp(p_client, workload, cluster_ip, test_pods, source_port):
    pods = p_client.list_pod(workloadId=workload.id).data
    target_name_list = []
    for pod in pods:
        target_name_list.append(pod["name"])
    curl_cmd = "http://" + cluster_ip + ":" + \
               str(source_port) + "/name.html"
    for pod in test_pods:
        validate_http_response(curl_cmd, target_name_list, pod)


def wait_for_pv_to_be_available(c_client, pv_object, timeout=DEFAULT_TIMEOUT):
    start = time.time()
    time.sleep(2)
    list = c_client.list_persistent_volume(uuid=pv_object.uuid).data
    assert len(list) == 1
    pv = list[0]
    while pv.state != "available":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to available")
        time.sleep(.5)
        list = c_client.list_persistent_volume(uuid=pv_object.uuid).data
        assert len(list) == 1
        pv = list[0]
    return pv


def wait_for_pvc_to_be_bound(p_client, pvc_object, timeout=DEFAULT_TIMEOUT):
    start = time.time()
    time.sleep(2)
    list = p_client.list_persistent_volume_claim(uuid=pvc_object.uuid).data
    assert len(list) == 1
    pvc = list[0]
    while pvc.state != "bound":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to bound")
        time.sleep(.5)
        list = p_client.list_persistent_volume_claim(uuid=pvc_object.uuid).data
        assert len(list) == 1
        pvc = list[0]
    return pvc


def create_wl_with_nfs(p_client, ns_id, pvc_name, wl_name,
                       mount_path, sub_path, is_daemonSet=False):
    volumes = [{"type": "volume",
                "name": "vol1",
                "persistentVolumeClaim": {
                    "readOnly": "false",
                    "type": "persistentVolumeClaimVolumeSource",
                    "persistentVolumeClaimId": pvc_name
                }}]
    volumeMounts = [{"readOnly": "False",
                     "type": "volumeMount",
                     "mountPath": mount_path,
                     "subPath": sub_path,
                     "name": "vol1"
                     }]
    con = [{"name": "test1",
            "image": TEST_IMAGE,
            "volumeMounts": volumeMounts
            }]
    if is_daemonSet:
        workload = p_client.create_workload(name=wl_name,
                                            containers=con,
                                            namespaceId=ns_id,
                                            volumes=volumes,
                                            daemonSetConfig={})
    else:
        workload = p_client.create_workload(name=wl_name,
                                            containers=con,
                                            namespaceId=ns_id,
                                            volumes=volumes)
    return workload


def write_content_to_file(pod, content, filename):
    cmd_write = "/bin/bash -c 'echo {1} > {0}'".format(filename, content)
    if is_windows():
        cmd_write = \
            'powershell -NoLogo -NonInteractive -Command ' \
            '"& { echo {1} > {0} }"'.format(filename, content)
    output = kubectl_pod_exec(pod, cmd_write)
    assert output.strip().decode('utf-8') == ""


def validate_file_content(pod, content, filename):
    cmd_get_content = "/bin/bash -c 'cat {0}' ".format(filename)
    if is_windows():
        cmd_get_content = 'powershell -NoLogo -NonInteractive -Command ' \
                          '"& { cat {0} }"'.format(filename)
    output = kubectl_pod_exec(pod, cmd_get_content)
    assert output.strip().decode('utf-8') == content


def wait_for_mcapp_to_active(client, multiClusterApp,
                             timeout=DEFAULT_MULTI_CLUSTER_APP_TIMEOUT):
    time.sleep(5)
    # When the app is deployed it goes into Active state for a short
    # period of time and then into installing/deploying.
    mcapps = client.list_multiClusterApp(uuid=multiClusterApp.uuid,
                                         name=multiClusterApp.name).data
    start = time.time()
    assert len(mcapps) == 1, "Cannot find multi cluster app"
    mapp = mcapps[0]
    while mapp.state != "active":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        multiclusterapps = client.list_multiClusterApp(
            uuid=multiClusterApp.uuid, name=multiClusterApp.name).data
        assert len(multiclusterapps) == 1
        mapp = multiclusterapps[0]
    return mapp


def wait_for_app_to_active(client, app_id,
                           timeout=DEFAULT_MULTI_CLUSTER_APP_TIMEOUT):
    """
    First wait for app to come in deployment state, then wait for it get
    in active state. This is to avoid wrongly conclude that app is active
    as app goes to state installing > active > deploying > active
    @param client: Project client
    @param app_id: App id of deployed app.
    @param timeout: Max time allowed to wait for app to become active.
    @return: app object
    """
    start = time.time()
    app_data = client.list_app(id=app_id).data
    while len(app_data) == 0:
        if time.time() - start > timeout / 10:
            raise AssertionError(
                "Timed out waiting for listing the app from API")
        time.sleep(.2)
        app_data = client.list_app(id=app_id).data

    application = app_data[0]
    while application.state != "deploying":
        if time.time() - start > timeout / 3:
            break
        time.sleep(.2)
        app_data = client.list_app(id=app_id).data
        application = app_data[0]
    while application.state != "active":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for {0} to get to active,"
                " the actual state: {1}".format(application.name,
                                                application.state))
        time.sleep(.5)
        app = client.list_app(id=app_id).data
        assert len(app) >= 1
        application = app[0]
    return application


def wait_for_app_to_remove(client, app_id,
                           timeout=DEFAULT_MULTI_CLUSTER_APP_TIMEOUT):
    start = time.time()
    app_data = client.list_app(id=app_id).data
    if len(app_data) == 0:
        return
    application = app_data[0]
    while application.state == "removing" or application.state == "active":
        if time.time() - start > timeout / 10:
            raise AssertionError(
                "Timed out waiting for app to not be installed")
        time.sleep(.2)
        app_data = client.list_app(id=app_id).data
        if len(app_data) == 0:
            break
        application = app_data[0]


def validate_response_app_endpoint(p_client, appId,
                                   timeout=DEFAULT_MULTI_CLUSTER_APP_TIMEOUT):
    ingress_list = p_client.list_ingress(namespaceId=appId).data
    assert len(ingress_list) == 1
    ingress = ingress_list[0]
    if hasattr(ingress, 'publicEndpoints'):
        for public_endpoint in ingress.publicEndpoints:
            url = \
                public_endpoint["protocol"].lower() + "://" + \
                public_endpoint["hostname"]
            print(url)
            start = time.time()
            try:
                while True:
                    r = requests.head(url)
                    print(r.status_code)
                    if r.status_code == 200:
                        return
                    if time.time() - start > timeout:
                        raise AssertionError(
                            "Timed out waiting response to be 200.")
                    time.sleep(.5)
            except requests.ConnectionError:
                print("failed to connect")
                assert False, "failed to connect to the app"


def resolve_node_ip(node):
    if hasattr(node, 'externalIpAddress'):
        node_ip = node.externalIpAddress
    else:
        node_ip = node.ipAddress
    return node_ip


def provision_nfs_server():
    node = AmazonWebServices().create_node(random_test_name("nfs-server"))
    node.wait_for_ssh_ready()
    c_path = os.getcwd()
    cmd_path = c_path + "/tests/v3_api/scripts/nfs-setup.sh"
    command = open(cmd_path, 'r').read()
    node.execute_command(command)
    return node


def get_defaut_question_answers(client, externalId):
    def get_answer(quest):
        if "default" in quest.keys():
            answer = quest["default"]
        else:
            answer = ""
            # If required and no default value is available, set fake value
            # only for type string . For other types error out
            if "required" in quest.keys():
                if quest["required"]:
                    if quest["type"] == "enum" and "options" in quest.keys():
                        answer = quest["options"][0]
                    elif quest["type"] == "password":
                        answer = "R@ncher135"
                    elif quest["type"] == "string":
                        answer = "fake"
                    else:
                        assert False, \
                            "Cannot set default for types {}" \
                            "".format(quest["type"])
        return answer

    def check_if_question_needed(questions_and_answers, ques):
        add_question = False
        match_string = ques["showIf"]
        match_q_as = match_string.split("&&")
        for q_a in match_q_as:
            items = q_a.split("=")
            if len(items) == 1:
                items.append("")
            if items[0] in questions_and_answers.keys():
                if questions_and_answers[items[0]] == items[1]:
                    add_question = True
                else:
                    add_question = False
                    break
        return add_question

    questions_and_answers = {}
    print("external id = {}".format(externalId))
    template_revs = client.list_template_version(externalId=externalId).data
    assert len(template_revs) == 1
    template_rev = template_revs[0]
    questions = template_rev.questions
    for ques in questions:
        add_question = True
        if "showIf" in ques.keys():
            add_question = \
                check_if_question_needed(questions_and_answers, ques)
        if add_question:
            question = ques["variable"]
            answer = get_answer(ques)
            questions_and_answers[question] = get_answer(ques)
            if "showSubquestionIf" in ques.keys():
                if ques["showSubquestionIf"] == answer:
                    sub_questions = ques["subquestions"]
                    for sub_question in sub_questions:
                        question = sub_question["variable"]
                        questions_and_answers[question] = \
                            get_answer(sub_question)
    print("questions_and_answers = {}".format(questions_and_answers))
    return questions_and_answers


def validate_app_deletion(client, app_id,
                          timeout=DEFAULT_APP_DELETION_TIMEOUT):
    app_data = client.list_app(id=app_id).data
    start = time.time()
    if len(app_data) == 0:
        return
    application = app_data[0]
    while application.state == "removing":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for app to delete")
        time.sleep(.5)
        app_data = client.list_app(id=app_id).data
        if len(app_data) == 0:
            break
        application = app_data[0]


def validate_catalog_app(proj_client, app, external_id, answer=None):
    """
    This method validates all the workloads deployed are in active state,
    have correct version and validates the answers.
    @param proj_client: Project client object of a existing project.
    @param app: Deployed app object.
    @param external_id: URl of app API.
    @param answer: answer, app seek while deploying, body of the post call.
    @return: Deployed app object.
    """
    if answer is None:
        answers = get_defaut_question_answers(get_user_client(), external_id)
    else:
        answers = answer
    # validate app is active
    app = wait_for_app_to_active(proj_client, app.id)
    assert app.externalId == external_id, \
        "the version of the app is not correct"
    # check if associated workloads are active
    ns = app.targetNamespace
    parameters = external_id.split('&')
    assert len(parameters) > 1, \
        "Incorrect list of parameters from catalog external ID"
    chart_prefix = parameters[len(parameters) - 2].split("=")[1]
    chart_suffix = parameters[len(parameters) - 1].split("=")[1]
    chart = chart_prefix + "-" + chart_suffix
    app_name = parameters[len(parameters) - 2].split("=")[1]
    workloads = proj_client.list_workload(namespaceId=ns).data

    # For longhorn app, only active state of workloads is verified as longhorn
    # workloads do not have the field workloadLabels
    # For all other apps active state of workloads & chart version are verified
    if "longhorn" in app.externalId:
        print("validating the Longhorn app, it may take longer than others")
        for wl in workloads:
            wait_for_wl_to_active(proj_client, wl)
    else:
        for wl in workloads:
            print("Workload {} , state - {}".format(wl.id, wl.state))
            assert wl.state == "active"
            chart_deployed = get_chart_info(wl.workloadLabels)
            print("Chart detail of app - {}".format(chart_deployed))
            # '-' check is to make sure chart has both app name and version.
            if app_name in chart_deployed and '-' in chart_deployed:
                assert chart_deployed == chart, "the chart version is wrong"
    # Validate_app_answers
    assert len(answers.items() - app["answers"].items()) == 0, \
        "Answers are not same as the original catalog answers"
    return app


def get_chart_info(workloadlabels):
    """
    This method finds either 'chart' tag or
    'helm.sh/chart' tag from workload API
    @param workloadlabels: workloadslabel object
    @return: chart value of workload e.g. 'app_name-version'
    """
    if "chart" in workloadlabels.keys():
        return workloadlabels.chart
    elif "helm.sh/chart" in workloadlabels.keys():
        return workloadlabels["helm.sh/chart"]
    else:
        return ''


def create_user(client, cattle_auth_url=CATTLE_AUTH_URL):
    user_name = random_name()
    user = client.create_user(username=user_name,
                              password=USER_PASSWORD)
    client.create_global_role_binding(globalRoleId="user",
                                      subjectKind="User",
                                      userId=user.id)
    user_token = get_user_token(user.username, USER_PASSWORD, cattle_auth_url)
    return user, user_token


def get_user_token(username, password, cattle_auth_url=CATTLE_AUTH_URL):
    r = requests.post(cattle_auth_url, json={
        'username': username,
        'password': password,
        'responseType': 'json',
    }, verify=False)
    print(r.json())
    return r.json()["token"]


def rbac_get_user_by_role(role):
    if role in rbac_data["users"].keys():
        return rbac_data["users"][role]["user"]
    return None


def rbac_get_user_token_by_role(role):
    if role in rbac_data["users"].keys():
        return rbac_data["users"][role]["token"]
    return None


def rbac_get_kubeconfig_by_role(role):
    if role in rbac_data["users"].keys():
        return rbac_data["users"][role]["kubeconfig"]
    return None


def rbac_get_project():
    return rbac_data["project"]


def rbac_get_namespace():
    return rbac_data["namespace"]


def rbac_get_workload():
    return rbac_data["workload"]


def rbac_get_unshared_project():
    return rbac_data["p_unshared"]


def rbac_get_unshared_ns():
    return rbac_data["ns_unshared"]


def rbac_get_unshared_workload():
    return rbac_data["wl_unshared"]


def rbac_prepare():
    """this function creates one project, one namespace,
    and four users with different roles"""
    admin_client, cluster = get_global_admin_client_and_cluster()
    create_kubeconfig(cluster)
    # create a new project in the cluster
    project, ns = create_project_and_ns(ADMIN_TOKEN,
                                        cluster,
                                        random_test_name("p-test-rbac"))
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    p_client = get_project_client_for_token(project, ADMIN_TOKEN)
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id)
    validate_workload(p_client, workload, "deployment", ns.name)
    rbac_data["workload"] = workload
    rbac_data["project"] = project
    rbac_data["namespace"] = ns
    # create new users
    for key in rbac_data["users"]:
        user1, token1 = create_user(admin_client)
        rbac_data["users"][key]["user"] = user1
        rbac_data["users"][key]["token"] = token1

    # assign different role to each user
    assign_members_to_cluster(admin_client,
                              rbac_data["users"][CLUSTER_OWNER]["user"],
                              cluster,
                              CLUSTER_OWNER)
    assign_members_to_cluster(admin_client,
                              rbac_data["users"][CLUSTER_MEMBER]["user"],
                              cluster,
                              CLUSTER_MEMBER)
    assign_members_to_project(admin_client,
                              rbac_data["users"][PROJECT_MEMBER]["user"],
                              project,
                              PROJECT_MEMBER)
    assign_members_to_project(admin_client,
                              rbac_data["users"][PROJECT_OWNER]["user"],
                              project,
                              PROJECT_OWNER)
    assign_members_to_project(admin_client,
                              rbac_data["users"][PROJECT_READ_ONLY]["user"],
                              project,
                              PROJECT_READ_ONLY)
    # create kubeconfig files for each user
    for key in rbac_data["users"]:
        user_client = get_client_for_token(rbac_data["users"][key]["token"])
        _, user_cluster = get_user_client_and_cluster(user_client)
        rbac_data["users"][key]["kubeconfig"] = os.path.join(
            os.path.dirname(os.path.realpath(__file__)),
            key + "_kubeconfig")
        create_kubeconfig(user_cluster, rbac_data["users"][key]["kubeconfig"])

    # create another project that none of the above users are assigned to
    p2, ns2 = create_project_and_ns(ADMIN_TOKEN,
                                    cluster,
                                    random_test_name("p-unshared"))
    name = random_test_name("default")
    p_client = get_project_client_for_token(p2, ADMIN_TOKEN)
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns2.id)
    validate_workload(p_client, workload, "deployment", ns2.name)
    rbac_data["p_unshared"] = p2
    rbac_data["ns_unshared"] = ns2
    rbac_data["wl_unshared"] = workload


def rbac_cleanup():
    """ remove the project, namespace and users created for the RBAC tests"""
    try:
        client = get_admin_client()
    except Exception:
        print("Not able to get admin client. Not performing RBAC cleanup")
        return
    for _, value in rbac_data["users"].items():
        try:
            client.delete(value["user"])
        except Exception:
            pass
    client.delete(rbac_data["project"])
    client.delete(rbac_data["wl_unshared"])
    client.delete(rbac_data["p_unshared"])


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


def create_catalog_external_id(catalog_name, template, version,
                               project_cluster_id=None, catalog_type=None):
    if catalog_type is None:
        return "catalog://?catalog=" + catalog_name + \
               "&template=" + template + "&version=" + version
    elif catalog_type == "project" or catalog_type == "cluster":
        return "catalog://?catalog=" + project_cluster_id + "/" \
               + catalog_name + "&type=" + catalog_type \
               + "Catalog&template=" + template + "&version=" + version


def wait_for_catalog_active(client, catalog, timeout=DEFAULT_CATALOG_TIMEOUT):
    time.sleep(2)
    catalog_data = client.list_catalog(name=catalog.name)
    print(catalog_data)
    start = time.time()
    assert len(catalog_data["data"]) >= 1, "Cannot find catalog"
    catalog = catalog_data["data"][0]
    while catalog.state != "active":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        catalog_data = client.list_catalog(name=catalog.name)
        assert len(catalog_data["data"]) >= 1
        catalog = catalog_data["data"][0]
    return catalog


def readDataFile(data_dir, name):
    fname = os.path.join(data_dir, name)
    print("File: " + fname)
    is_file = os.path.isfile(fname)
    assert is_file
    with open(fname) as f:
        return f.read()


def set_url_password_token(rancher_url, server_url=None, version=""):
    """Returns a ManagementContext for the default global admin user."""
    auth_url = \
        rancher_url + "/v3-public/localproviders/local?action=login"
    rpassword = 'admin'
    print(auth_url)
    if "master" in version or \
            "2.6" in version or \
            "2.7" in version or \
            "2.8" in version or \
            "2.9" in version:
        rpassword = ADMIN_PASSWORD
        print("on 2.6 or later")
    retries = 5
    for attempt in range(1, retries):
        try:
            r = requests.post(auth_url, json={
                'username': 'admin',
                'password': rpassword,
                'responseType': 'json',
            }, verify=False)
        except requests.exceptions.RequestException:
            print("password request failed. Retry attempt: ",
                  "{} of {}".format(attempt, retries))
            time.sleep(2)
        else:
            break
    print(r.json())
    token = r.json()['token']
    print(token)
    # Change admin password
    client = rancher.Client(url=rancher_url + "/v3",
                            token=token, verify=False)
    admin_user = client.list_user(username="admin").data
    admin_user[0].setpassword(newPassword=ADMIN_PASSWORD)

    # Set server-url settings
    serverurl = client.list_setting(name="server-url").data
    if server_url:
        client.update(serverurl[0], value=server_url)
    else:
        client.update(serverurl[0], value=rancher_url)
    return token


def validate_create_catalog(token, catalog_name, branch, url, permission=True):
    """
    This function validates if the user has the permission to create a
    global catalog.

    :param token: user's token
    :param catalog_name: the name of the catalog
    :param branch: the branch of the git repo
    :param url:  the url of the git repo
    :param permission: boolean value, True if the user can create catalog
    :return: the catalog object or None
    """
    client = get_client_for_token(token)
    if not permission:
        with pytest.raises(ApiError) as e:
            client.create_catalog(name=catalog_name,
                                  branch=branch,
                                  url=url)
        error_msg = "user with no permission should receive 403: Forbidden"
        error_code = e.value.error.code
        error_status = e.value.error.status
        assert error_status == 403 and error_code == 'Forbidden', error_msg
        return None
    else:
        try:
            client.create_catalog(name=catalog_name,
                                  branch=branch,
                                  url=url)
        except ApiError as e:
            assert False, "user with permission should receive no exception:" \
                          + str(e.error.status) + " " + e.error.code

    catalog_list = client.list_catalog(name=catalog_name).data
    assert len(catalog_list) == 1
    return catalog_list[0]


def generate_template_global_role(name, new_user_default=False, template=None):
    """ generate a template that is used for creating a global role"""
    if template is None:
        template = TEMPLATE_MANAGE_CATALOG
    template = deepcopy(template)
    if new_user_default:
        template["newUserDefault"] = "true"
    else:
        template["newUserDefault"] = "false"
    if name is None:
        name = random_name()
    template["name"] = name
    return template


def wait_for_backup_to_active(cluster, backupname,
                              timeout=DEFAULT_TIMEOUT):
    start = time.time()
    timeout = start + timeout
    etcdbackups = cluster.etcdBackups(name=backupname)
    assert len(etcdbackups) == 1
    etcdbackupdata = etcdbackups['data']
    etcdbackupstate = etcdbackupdata[0]['state']
    while etcdbackupstate != "active":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        etcdbackups = cluster.etcdBackups(name=backupname)
        assert len(etcdbackups) == 1
        etcdbackupdata = etcdbackups['data']
        etcdbackupstate = etcdbackupdata[0]['state']
    print("BACKUP STATE")
    print(etcdbackupstate)
    return etcdbackupstate


def wait_for_backup_to_delete(cluster, backupname,
                              timeout=DEFAULT_TIMEOUT):
    start = time.time()
    etcdbackups = cluster.etcdBackups(name=backupname)
    while len(etcdbackups) == 1:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for backup to be deleted")
        time.sleep(.5)
        etcdbackups = cluster.etcdBackups(name=backupname)


def validate_backup_create(namespace, backup_info, backup_mode=None):
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    cluster = namespace["cluster"]
    name = random_test_name("default")

    if not hasattr(cluster, 'rancherKubernetesEngineConfig'):
        assert False, "Cluster is not of type RKE"

    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    backup_info["workload"] = p_client.create_workload(name=name,
                                                       containers=con,
                                                       namespaceId=ns.id,
                                                       daemonSetConfig={})
    validate_workload(p_client, backup_info["workload"], "daemonSet", ns.name,
                      len(get_schedulable_nodes(cluster)))
    host = "test" + str(random_int(10000, 99999)) + ".com"
    namespace["host"] = host
    path = "/name.html"
    rule = {"host": host,
            "paths": [{"workloadIds": [backup_info["workload"].id],
                       "targetPort": TEST_IMAGE_PORT}]}
    p_client.create_ingress(name=name,
                            namespaceId=ns.id,
                            rules=[rule])
    validate_ingress(p_client, cluster, [backup_info["workload"]], host, path)

    # Perform Backup
    user_client = get_user_client()
    cluster = user_client.reload(cluster)
    backup = cluster.backupEtcd()
    backup_info["backupname"] = backup['metadata']['name']
    wait_for_backup_to_active(cluster, backup_info["backupname"])

    # Get all the backup info
    etcdbackups = cluster.etcdBackups(name=backup_info["backupname"])
    backup_info["etcdbackupdata"] = etcdbackups['data']
    backup_info["backup_id"] = backup_info["etcdbackupdata"][0]['id']

    if backup_mode == "s3":
        backupfileurl = backup_info["etcdbackupdata"][0]['filename']
        # Check the backup filename exists in S3
        parseurl = urlparse(backupfileurl)
        backup_info["backupfilename"] = os.path.basename(parseurl.path)
        backup_found = AmazonWebServices().s3_backup_check(
            backup_info["backupfilename"])
        assert backup_found, "the backup was not found in the S3 bucket"
    elif backup_mode == 'filesystem':
        for node in namespace['nodes']:
            if 'etcd' not in node.roles:
                continue
            get_filesystem_snapshots = 'ls /opt/rke/etcd-snapshots'
            response = node.execute_command(get_filesystem_snapshots)[0]
            assert backup_info["etcdbackupdata"][0]['filename'] in response, \
                "The filename doesn't match any of the files locally"
    return namespace, backup_info


def validate_backup_restore(namespace, backup_info):
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    client = get_user_client()
    cluster = namespace["cluster"]
    name = random_test_name("default")

    host = namespace["host"]
    path = "/name.html"
    con = [{"name": "test1",
            "image": TEST_IMAGE}]

    # Create workload after backup
    testworkload = p_client.create_workload(name=name,
                                            containers=con,
                                            namespaceId=ns.id)

    validate_workload(p_client, testworkload, "deployment", ns.name)

    # Perform Restore
    cluster.restoreFromEtcdBackup(etcdBackupId=backup_info["backup_id"])
    # After restore, validate cluster
    validate_cluster(client, cluster, intermediate_state="updating",
                     check_intermediate_state=True,
                     skipIngresscheck=False)

    # Verify the ingress created before taking the snapshot
    validate_ingress(p_client, cluster, [backup_info["workload"]], host, path)

    # Verify the workload created after getting a snapshot does not exist
    # after restore
    workload_list = p_client.list_workload(uuid=testworkload.uuid).data
    print(len(workload_list))
    assert len(workload_list) == 0, "workload shouldn't exist after restore"
    return namespace, backup_info


def validate_backup_delete(namespace, backup_info, backup_mode=None):
    client = get_user_client()
    cluster = namespace["cluster"]
    client.delete(
        cluster.etcdBackups(name=backup_info["backupname"])['data'][0]
    )
    wait_for_backup_to_delete(cluster, backup_info["backupname"])
    assert len(cluster.etcdBackups(name=backup_info["backupname"])) == 0, \
        "backup shouldn't be listed in the Cluster backups"
    if backup_mode == "s3":
        # Check the backup reference is deleted in Rancher and S3
        backup_found = AmazonWebServices().s3_backup_check(
            backup_info["backupfilename"])
        assert_message = "The backup should't exist in the S3 bucket"
        assert backup_found is False, assert_message
    elif backup_mode == 'filesystem':
        for node in namespace['nodes']:
            if 'etcd' not in node.roles:
                continue
            get_filesystem_snapshots = 'ls /opt/rke/etcd-snapshots'
            response = node.execute_command(get_filesystem_snapshots)[0]
            filename = backup_info["etcdbackupdata"][0]['filename']
            assert filename not in response, \
                "The file still exist in the filesystem"


def apply_crd(ns, file, kubectl_context):
    return execute_kubectl_cmd('apply -f ' + file + ' -n ' + ns.name,
                               json_out=False, stderr=True,
                               kubeconfig=kubectl_context).decode("ascii")


def get_crd(ns, crd_name, kubectl_context):
    return execute_kubectl_cmd('get ' + crd_name + ' -n ' + ns.name,
                               json_out=False, stderr=True,
                               kubeconfig=kubectl_context).decode("ascii")


def delete_crd(ns, file, kubectl_context):
    return execute_kubectl_cmd('delete -f ' + file + ' -n ' + ns.name,
                               json_out=False, stderr=True,
                               kubeconfig=kubectl_context).decode("ascii")


def prepare_auth_data():
    name = \
        os.path.join(os.path.dirname(os.path.realpath(__file__)) + "/resource",
                     AUTH_PROVIDER.lower() + ".json")
    with open(name) as reader:
        auth_data = reader.read()
    raw = json.loads(auth_data).get("nested_group_info")

    nested_group["auth_info"] = raw.copy()
    nested_group["users"] = raw.get("users")
    raw.pop("users")
    nested_group["group_dic"] = raw
    nested_group["groups"] = raw.keys()


def is_nested():
    """ check if the provided groups are nested groups,
    return True if at least one of the groups contains other groups
    """
    count = 0
    for user, group in nested_group["group_dic"].items():
        if len(group) == 0:
            count += 1
    if count < len(nested_group["group_dic"]):
        return True
    return False


def get_group(nested=False):
    """ return a group or a nested group"""
    if nested:
        # return the name of a group that contains at least one other group
        for item in nested_group["groups"]:
            if len(nested_group["group_dic"].get(item).get("users")) == 0:
                pass
            sub_groups = nested_group["group_dic"].get(item).get("groups")
            if len(sub_groups) == 0:
                pass
            for g in sub_groups:
                if len(nested_group["group_dic"].get(g).get("users")) > 0:
                    return item
        assert False, "cannot find any valid nested group"

    else:
        # return the name of a group that has at least one direct user
        for group in nested_group["groups"]:
            if len(nested_group["group_dic"].get(group).get("users")) > 0:
                return group
        assert False, "cannot find any valid non-nested group"


def get_user_by_group(group, nested=False):
    """ return the list of uses in the group or nested group

    if nested is False, return the direct users in the group;
    otherwise, return all users including those from nested groups
    """

    def get_user_in_nested_group(group, source):
        if group == "":
            return []
        users = source["group_dic"].get(group).get("users")
        for sub_group in source["group_dic"].get(group).get("groups"):
            temp = get_user_in_nested_group(sub_group, source)
            for user in temp:
                if user not in users:
                    users.append(user)
        return users

    if nested:
        users = get_user_in_nested_group(group, nested_group)
        assert len(users) > 0, "no user in the group"
    else:
        users = nested_group["group_dic"].get(group).get("users")
        assert users is not None, "no user in the group"
    print("group: {}, users: {}".format(group, users))
    return users


def get_a_group_and_a_user_not_in_it(nested=False):
    """ return a group or a nested group and a user that is not in the group"""
    all_users = nested_group["users"]
    for group in nested_group["groups"]:
        group_users = get_user_by_group(group, nested)
        for user in all_users:
            if user not in group_users:
                print("group: {}, user not in it: {}".format(group, user))
                return group, user
    assert False, "cannot find a group and a user not in it"


def get_group_principal_id(group_name, token=ADMIN_TOKEN, expected_status=200):
    """ get the group's principal id from the auth provider"""
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.post(CATTLE_AUTH_PRINCIPAL_URL,
                      json={'name': group_name,
                            'principalType': 'group',
                            'responseType': 'json'},
                      verify=False, headers=headers)
    assert r.status_code == expected_status
    return r.json()['data'][0]["id"]


def login_as_auth_user(username, password, login_url=LOGIN_AS_AUTH_USER_URL):
    """ login with the user account from the auth provider,
    and return the user token"""
    r = requests.post(login_url, json={
        'username': username,
        'password': password,
        'responseType': 'json',
    }, verify=False)
    assert r.status_code in [200, 201]
    return r.json()


def validate_service_discovery(workload, scale,
                               p_client=None, ns=None, testclient_pods=None):
    expected_ips = []
    pods = p_client.list_pod(workloadId=workload["id"]).data
    assert len(pods) == scale
    for pod in pods:
        expected_ips.append(pod["status"]["podIp"])
    host = '{0}.{1}.svc.cluster.local'.format(workload.name, ns.id)
    for pod in testclient_pods:
        validate_dns_entry(pod, host, expected_ips)


def auth_get_project():
    return auth_rbac_data["project"]


def auth_get_namespace():
    return auth_rbac_data["namespace"]


def auth_get_user_token(username):
    if username in auth_rbac_data["users"].keys():
        return auth_rbac_data["users"][username].token
    return None


def add_role_to_user(user, role):
    """this function adds a user from the auth provider to given cluster"""
    admin_client, cluster = get_global_admin_client_and_cluster()
    project = auth_get_project()
    ns = auth_get_namespace()
    if not (project and ns):
        project, ns = create_project_and_ns(ADMIN_TOKEN, cluster,
                                            random_test_name("p-test-auth"))
        auth_rbac_data["project"] = project
        auth_rbac_data["namespace"] = ns
    if role in [PROJECT_OWNER, PROJECT_MEMBER, PROJECT_READ_ONLY]:
        assign_members_to_project(admin_client, user, project, role)
    else:
        assign_members_to_cluster(admin_client, user, cluster, role)
    auth_rbac_data["users"][user.username] = user


def auth_resource_cleanup():
    """ remove the project and namespace created for the AUTH tests"""
    client, cluster = get_global_admin_client_and_cluster()
    client.delete(auth_rbac_data["project"])
    auth_rbac_data["project"] = None
    auth_rbac_data["ns"] = None
    for username, user in auth_rbac_data["users"].items():
        user_crtbs = client.list_cluster_role_template_binding(userId=user.id)
        for crtb in user_crtbs:
            client.delete(crtb)


class WebsocketLogParse:
    """
    the class is used for receiving and parsing the message
    received from the websocket
    """

    def __init__(self):
        self.lock = Lock()
        self._last_message = ''

    def receiver(self, socket, skip, b64=True):
        """
        run a thread to receive and save the message from the web socket
        :param socket: the socket connection
        :param skip: if True skip the first char of the received message
        """
        while True and socket.connected:
            try:
                data = socket.recv()
                # the message from the kubectl contains an extra char
                if skip:
                    data = data[1:]
                if len(data) < 5:
                    pass
                if b64:
                    data = base64.b64decode(data).decode()
                self.lock.acquire()
                self._last_message += data
                self.lock.release()
            except websocket.WebSocketConnectionClosedException:
                print("Connection closed")
                break
            except websocket.WebSocketProtocolException as wpe:
                print("Error: {}".format(wpe))
                break

    @staticmethod
    def start_thread(target, args):
        thread = Thread(target=target, args=args)
        thread.daemon = True
        thread.start()
        time.sleep(1)

    @property
    def last_message(self):
        return self._last_message

    @last_message.setter
    def last_message(self, value):
        self.lock.acquire()
        self._last_message = value
        self.lock.release()


def wait_for_cluster_delete(client, cluster_name, timeout=DEFAULT_TIMEOUT):
    start = time.time()
    cluster = client.list_cluster(name=cluster_name).data
    cluster_count = len(cluster)
    while cluster_count != 0:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for cluster to get deleted")
        time.sleep(.5)
        cluster = client.list_cluster(name=cluster_name).data
        cluster_count = len(cluster)


def create_connection(url, subprotocols):
    """
    create a webscoket connection and check if it is connected
    :param url: the url to connect to
    :param subprotocols: the list of subprotocols
    :return:
    """
    ws = websocket.create_connection(
        url=url,
        sslopt={"cert_reqs": ssl.CERT_NONE},
        subprotocols=subprotocols,
        timeout=20,
        cookie="R_SESS=" + USER_TOKEN
    )
    assert ws.connected, "failed to build the websocket"
    return ws


def wait_for_hpa_to_active(client, hpa, timeout=DEFAULT_TIMEOUT):
    start = time.time()
    hpalist = client.list_horizontalPodAutoscaler(uuid=hpa.uuid).data
    assert len(hpalist) == 1
    hpa = hpalist[0]
    while hpa.state != "active":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        hpas = client.list_horizontalPodAutoscaler(uuid=hpa.uuid).data
        assert len(hpas) == 1
        hpa = hpas[0]
    return hpa


def create_pv_pvc(client, ns, nfs_ip, cluster_client):
    pv_object = create_pv(cluster_client, nfs_ip)

    pvc_name = random_test_name("pvc")
    pvc_config = {"accessModes": ["ReadWriteOnce"],
                  "name": pvc_name,
                  "volumeId": pv_object.id,
                  "namespaceId": ns.id,
                  "storageClassId": "",
                  "resources": {"requests": {"storage": "10Gi"}}
                  }
    pvc_object = client.create_persistent_volume_claim(pvc_config)
    pvc_object = wait_for_pvc_to_be_bound(client, pvc_object, timeout=300)

    return pv_object, pvc_object


def create_pv(client, nfs_ip):
    pv_name = random_test_name("pv")
    pv_config = {"type": "persistentVolume",
                 "accessModes": ["ReadWriteOnce"],
                 "name": pv_name,
                 "nfs": {"readOnly": "false",
                         "type": "nfsvolumesource",
                         "path": NFS_SERVER_MOUNT_PATH,
                         "server": nfs_ip
                         },
                 "capacity": {"storage": "50Gi"}
                 }
    pv_object = client.create_persistent_volume(pv_config)
    capacitydict = pv_object['capacity']
    assert capacitydict['storage'] == '50Gi'
    assert pv_object['type'] == 'persistentVolume'
    return pv_object


def delete_resource_in_AWS_by_prefix(resource_prefix):
    """
    :param resource_prefix: the prefix of resource name
    :return: None
    """
    # delete nodes of both local and custom clusters
    node_filter = [{
        'Name': 'tag:Name',
        'Values': [resource_prefix + "-*"]
    }]
    nodes = AmazonWebServices().get_nodes(filters=node_filter)
    if nodes is None:
        print("deleting the following instances: None")
    else:
        print("deleting the following instances: {}"
              .format([node.public_ip_address for node in nodes]))
        AmazonWebServices().delete_nodes(nodes)

    # delete load balancer and target groups
    tg_list = []
    lb_list = []
    lb_names = [resource_prefix + '-nlb',
                resource_prefix + '-k3s-nlb',
                resource_prefix + '-internal-nlb']
    for name in lb_names:
        lb_arn = AmazonWebServices().get_lb(name)
        if lb_arn is not None:
            lb_list.append(lb_arn)
            res = AmazonWebServices().get_target_groups(lb_arn)
            tg_list.extend(res)

    print("deleting the following load balancers: {}".format(lb_list))
    print("deleting the following target groups: {}".format(tg_list))
    for lb in lb_list:
        AmazonWebServices().delete_lb(lb)
    for tg in tg_list:
        AmazonWebServices().delete_target_group(tg)

    # delete rds
    db_name = resource_prefix + "-db"
    print("deleting the database (if it exists): {}".format(db_name))
    AmazonWebServices().delete_db(db_name)

    # delete the route 53 record
    route53_names = [resource_prefix + ".qa.rancher.space.",
                     resource_prefix + "-internal.qa.rancher.space."]
    for name in route53_names:
        print("deleting the route53 record (if it exists): {}".format(name))
        AmazonWebServices().delete_route_53_record(name)

    print("deletion is done")
    return None


def configure_cis_requirements(aws_nodes, profile, node_roles, client,
                               cluster):
    prepare_hardened_nodes(
        aws_nodes, profile, node_roles, client, cluster, True)
    cluster = validate_cluster_state(client, cluster)

    # the workloads under System project to get active
    time.sleep(20)
    create_kubeconfig(cluster)
    prepare_hardened_cluster('rke-cis-1.5', kube_fname)
    return cluster


def get_node_details(cluster, client):
    """
    lists the nodes from the cluster. This cluster has only 1 node.
    :return: client and node object
    """
    create_kubeconfig(cluster)
    nodes = client.list_node(clusterId=cluster.id).data
    assert len(nodes) > 0
    for node in nodes:
        if node.worker:
            break
    return client, node


def create_service_account_configfile():
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    name = random_name()
    # create a service account
    execute_kubectl_cmd(cmd="create sa {}".format(name), json_out=False)
    # get the ca and token
    res = execute_kubectl_cmd(cmd="get secret -o name", json_out=False)
    secret_name = ""
    for item in res.split("\n"):
        if name in item:
            secret_name = item.split("/")[1]
            break
    res = execute_kubectl_cmd(cmd="get secret {}".format(secret_name))
    ca = res["data"]["ca.crt"]
    token = res["data"]["token"]
    token = base64.b64decode(token).decode()

    server = None
    nodes = client.list_node(clusterId=cluster.id).data
    for node in nodes:
        if node.controlPlane:
            server = "https://" + node.externalIpAddress + ":6443"
            break
    assert server is not None, 'failed to get the public ip of control plane'

    config = """
    apiVersion: v1
    kind: Config
    clusters:
    - name: test-cluster
      cluster:
        server: {server}
        certificate-authority-data: {ca}
    contexts:
    - name: default-context
      context:
        cluster: test-cluster
        namespace: default
        user: test-user
    current-context: default-context
    users:
    - name: test-user
      user:
        token: {token}
    """
    config = config.format(server=server, ca=ca, token=token)
    config_file = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                               name + ".yaml")
    with open(config_file, "w") as file:
        file.write(config)

    return name


def rbac_test_file_reader(file_path=None):
    """
    This method generates test cases from an input file and return the result
    that can be used to parametrize pytest cases
    :param file_path: the path to the JSON file for test cases
    :return: a list of tuples of
            (cluster_role, command, authorization, service account name)
    """
    if test_rbac_v2 == "False":
        return []

    if file_path is None:
        pytest.fail("no file is provided")
    with open(file_path) as reader:
        test_cases = json.loads(reader.read().replace("{resource_root}",
                                                      DATA_SUBDIR))
    output = []
    for cluster_role, checks in test_cases.items():
        # create a service account for each role
        name = create_service_account_configfile()
        # create the cluster role binding
        cmd = "create clusterrolebinding {} " \
              "--clusterrole {} " \
              "--serviceaccount {}".format(name, cluster_role,
                                           "default:" + name)
        execute_kubectl_cmd(cmd, json_out=False)
        for command in checks["should_pass"]:
            output.append((cluster_role, command, True, name))
        for command in checks["should_fail"]:
            output.append((cluster_role, command, False, name))

    return output


def validate_cluster_role_rbac(cluster_role, command, authorization, name):
    """
     This methods creates a new service account to validate the permissions
     both before and after creating the cluster role binding between the
     service account and the cluster role
    :param cluster_role:  the cluster role
    :param command: the kubectl command to run
    :param authorization: if the service account has the permission: True/False
    :param name: the name of the service account, cluster role binding, and the
    kubeconfig file
    """

    config_file = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                               name + ".yaml")
    result = execute_kubectl_cmd(command,
                                 json_out=False,
                                 kubeconfig=config_file,
                                 stderr=True).decode('utf_8')
    if authorization:
        assert "Error from server (Forbidden)" not in result, \
            "{} should have the authorization to run {}".format(cluster_role,
                                                                command)
    else:
        assert "Error from server (Forbidden)" in result, \
            "{} should NOT have the authorization to run {}".format(
                cluster_role, command)


def wait_until_app_v2_deployed(client, app_name, timeout=DEFAULT_APP_V2_TIMEOUT):
    """
    List all installed apps and check for the state of "app_name" to see
    if it == "deployed"
    :param client: cluster client for the user
    :param app_name: app which is being installed
    :param timeout: time for the app to come to Deployed state
    :return:
    """
    start = time.time()
    app = client.list_catalog_cattle_io_app()
    while True:
        app_list = []
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to Deployed")
        time.sleep(.5)
        for app in app["data"]:
            app_list.append(app["metadata"]["name"])
            if app["metadata"]["name"] == app_name:
                if app["status"]["summary"]["state"] == "deployed":
                    return app_list
        app = client.list_catalog_cattle_io_app()
    return


def wait_until_app_v2_uninstall(client, app_name, timeout=DEFAULT_APP_V2_TIMEOUT):
    """
    list all installed apps. search for "app_name" in the list
    if app_name is NOT in list, indicates the app has been uninstalled successfully
    :param client: cluster client for the user
    :param app_name: app which is being unstalled
    :param timeout: time for app to be uninstalled
    """
    start = time.time()
    app = client.list_catalog_cattle_io_app()
    while True:
        app_list = []
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to Uninstalled")
        time.sleep(.5)
        for app in app["data"]:
            app_list.append(app["metadata"]["name"])
        if app_name not in app_list:
            return app_list
        app = client.list_catalog_cattle_io_app()
    return


def check_v2_app_and_uninstall(client, chart_name):
    app = client.list_catalog_cattle_io_app()
    for app in app["data"]:
        if app["metadata"]["name"] == chart_name:
            response = client.action(obj=app, action_name="uninstall")
            app_list = wait_until_app_v2_uninstall(client, chart_name)
            assert chart_name not in app_list, \
                "App has not uninstalled"


def update_and_validate_kdm(kdm_url, admin_token=ADMIN_TOKEN,
                            rancher_api_url=CATTLE_API_URL):
    print("Updating KDM to use {}".format(kdm_url))
    header = {'Authorization': 'Bearer ' + admin_token}
    api_url = rancher_api_url + "/settings/rke-metadata-config"
    kdm_json = {
        "name": "rke-metadata-config",
        "value": json.dumps({
            "refresh-interval-minutes": "1440",
            "url": kdm_url
        })
    }
    r = requests.put(api_url, verify=False, headers=header, json=kdm_json)
    r_content = json.loads(r.content)
    assert r.ok
    assert r_content['name'] == kdm_json['name']
    assert r_content['value'] == kdm_json['value']
    time.sleep(2)

    # Refresh Kubernetes Metadata
    kdm_refresh_url = rancher_api_url + "/kontainerdrivers?action=refresh"
    response = requests.post(kdm_refresh_url, verify=False, headers=header)
    assert response.ok


def prepare_hardened_nodes(aws_nodes, profile, node_roles,
                           client=None, cluster=None, custom_cluster=False):
    i = 0
    conf_file = DATA_SUBDIR + "/sysctl-config"
    if profile == 'rke-cis-1.4':
        for aws_node in aws_nodes:
            file1 = open(conf_file, 'r')
            while True:
                line = file1.readline()
                if not line:
                    break
                aws_node.execute_command(line.strip())
            if "etcd" in node_roles[i]:
                aws_node.execute_command("sudo useradd etcd")
            if custom_cluster:
                docker_run_cmd = \
                    get_custom_host_registration_cmd(client,
                                                     cluster,
                                                     node_roles[i],
                                                     aws_node)
                aws_node.execute_command(docker_run_cmd)
            i += 1
    elif profile == 'rke-cis-1.5':
        for aws_node in aws_nodes:
            file1 = open(conf_file, 'r')
            while True:
                line = file1.readline()
                if not line:
                    break
                aws_node.execute_command(line.strip())
            if "etcd" in node_roles[i]:
                aws_node.execute_command("sudo groupadd -g 52034 etcd")
                aws_node.execute_command("sudo useradd -u 52034 -g 52034 etcd")
            if custom_cluster:
                docker_run_cmd = \
                    get_custom_host_registration_cmd(client,
                                                     cluster,
                                                     node_roles[i],
                                                     aws_node)
                aws_node.execute_command(docker_run_cmd)
            i += 1
    time.sleep(5)
    file1.close()
    return aws_nodes


def prepare_hardened_cluster(profile, kubeconfig_path):
    if profile == 'rke-cis-1.5':
        network_policy_file = DATA_SUBDIR + "/default-allow-all.yaml"
        account_update_file = DATA_SUBDIR + "/account_update.yaml"
        items = execute_kubectl_cmd("get namespaces -A",
                                    kubeconfig=kubeconfig_path)["items"]
        all_ns = [item["metadata"]["name"] for item in items]
        for ns in all_ns:
            execute_kubectl_cmd("apply -f {0} -n {1}".
                                format(network_policy_file, ns),
                                kubeconfig=kubeconfig_path)
            execute_kubectl_cmd('patch serviceaccount default'
                                ' -n {0} -p "$(cat {1})"'.
                                format(ns, account_update_file),
                                kubeconfig=kubeconfig_path)


def print_kubeconfig(kpath):
    kubeconfig_file = open(kpath, "r")
    kubeconfig_contents = kubeconfig_file.read()
    kubeconfig_file.close()
    kubeconfig_contents_encoded = base64.b64encode(
        kubeconfig_contents.encode("utf-8")).decode("utf-8")
    print("\n\n" + kubeconfig_contents + "\n\n")
    print("\nBase64 encoded: \n\n" + kubeconfig_contents_encoded + "\n\n")
