import inspect
import json
import os
import random
import subprocess
import time
import requests
import ast
import paramiko
import rancher
from rancher import ApiError
from lib.aws import AmazonWebServices

DEFAULT_TIMEOUT = 120
DEFAULT_MULTI_CLUSTER_APP_TIMEOUT = 300
DEFAULT_APP_DELETION_TIMEOUT = 360

CATTLE_TEST_URL = os.environ.get('CATTLE_TEST_URL', "http://localhost:80")
CATTLE_API_URL = CATTLE_TEST_URL + "/v3"

ADMIN_TOKEN = os.environ.get('ADMIN_TOKEN', "None")
kube_fname = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                          "k8s_kube_config")
MACHINE_TIMEOUT = float(os.environ.get('RANCHER_MACHINE_TIMEOUT', "1200"))

TEST_IMAGE = os.environ.get('RANCHER_TEST_IMAGE', "sangeetha/mytestcontainer")
CLUSTER_NAME = os.environ.get("RANCHER_CLUSTER_NAME", "")
CLUSTER_NAME_2 = os.environ.get("RANCHER_CLUSTER_NAME_2", "")
RANCHER_CLEANUP_CLUSTER = \
    ast.literal_eval(os.environ.get('RANCHER_CLEANUP_CLUSTER', "True"))
env_file = os.path.join(
    os.path.dirname(os.path.realpath(__file__)),
    "rancher_env.config")


def random_str():
    return 'random-{0}-{1}'.format(random_num(), int(time.time()))


def random_num():
    return random.randint(0, 1000000)


def random_int(start, end):
    return random.randint(start, end)


def random_test_name(name="test"):
    return name + "-" + str(random_int(10000, 99999))


def get_admin_client():
    return rancher.Client(url=CATTLE_API_URL, token=ADMIN_TOKEN, verify=False)


def get_client_for_token(token):
    return rancher.Client(url=CATTLE_API_URL, token=token, verify=False)


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


def wait_for(callback, timeout=DEFAULT_TIMEOUT, timeout_message=None):
    start = time.time()
    ret = callback()
    while ret is None or ret is False:
        time.sleep(.5)
        if time.time() - start > timeout:
            if timeout_message:
                raise Exception(timeout_message)
            else:
                raise Exception('Timeout waiting for condition')
        ret = callback()
    return ret


def random_name():
    return "test" + "-" + str(random_int(10000, 99999))


def create_project_and_ns(token, cluster, project_name=None, ns_name=None):
    client = get_client_for_token(token)
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
    crtb = client.update(
        crtb,
        roleTemplateId=role_template_id,
        userId=user.id)
    return crtb


def change_member_role_in_project(client, user, prtb, role_template_id):
    prtb = client.update(
        prtb,
        roleTemplateId=role_template_id,
        userId=user.id)
    return prtb


def create_kubeconfig(cluster):
    generateKubeConfigOutput = cluster.generateKubeconfig()
    print(generateKubeConfigOutput.config)
    file = open(kube_fname, "w")
    file.write(generateKubeConfigOutput.config)
    file.close()


def validate_psp_error_worklaod(p_client, workload, error_message):
    workload = wait_for_wl_transitioning(p_client, workload)
    assert workload.state == "updating"
    assert workload.transitioning == "error"
    print(workload.transitioningMessage)
    assert error_message in workload.transitioningMessage


def validate_workload(p_client, workload, type, ns_name, pod_count=1,
                      wait_for_cron_pods=60):
    workload = wait_for_wl_to_active(p_client, workload)
    assert workload.state == "active"
    # For cronjob, wait for the first pod to get created after
    # scheduled wait time
    if type == "cronJob":
        time.sleep(wait_for_cron_pods)
    pods = wait_for_pods_in_workload(p_client, workload, pod_count)
    assert len(pods) == pod_count
    pods = p_client.list_pod(workloadId=workload.id).data
    assert len(pods) == pod_count

    for pod in pods:
        wait_for_pod_to_running(p_client, pod)
    wl_result = execute_kubectl_cmd(
        "get " + type + " " + workload.name + " -n " + ns_name)
    if type == "deployment" or type == "statefulSet":
        assert wl_result["status"]["readyReplicas"] == pod_count
    if type == "daemonSet":
        assert wl_result["status"]["currentNumberScheduled"] == pod_count
    if type == "cronJob":
        assert len(wl_result["status"]["active"]) >= pod_count
        return
    for key, value in workload.workloadLabels.items():
        label = key + "=" + value
    get_pods = "get pods -l" + label + " -n " + ns_name
    pods_result = execute_kubectl_cmd(get_pods)
    assert len(pods_result["items"]) == pod_count
    for pod in pods_result["items"]:
        assert pod["status"]["phase"] == "Running"
    return pods_result["items"]


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


def execute_kubectl_cmd(cmd, json_out=True, stderr=False):
    command = 'kubectl --kubeconfig {0} {1}'.format(
        kube_fname, cmd)
    if json_out:
        command += ' -o json'
    if stderr:
        result = run_command_with_stderr(command)
    else:
        result = run_command(command)
    if json_out:
        result = json.loads(result)
    print(result)
    return result


def run_command(command):
    return subprocess.check_output(command, shell=True, text=True)


def run_command_with_stderr(command):
    try:
        output = subprocess.check_output(command, shell=True,
                                         stderr=subprocess.PIPE)
        returncode = 0
    except subprocess.CalledProcessError as e:
        output = e.output
        returncode = e.returncode
    print(returncode)
    return output


def wait_for_wl_to_active(client, workload, timeout=DEFAULT_TIMEOUT):
    start = time.time()
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


def wait_for_pod_to_running(client, pod, timeout=DEFAULT_TIMEOUT):
    start = time.time()
    pods = client.list_pod(uuid=pod.uuid).data
    assert len(pods) == 1
    p = pods[0]
    while p.state != "running":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        pods = client.list_pod(uuid=pod.uuid).data
        assert len(pods) == 1
        p = pods[0]
    return p


def get_schedulable_nodes(cluster):
    client = get_admin_client()
    nodes = client.list_node(clusterId=cluster.id).data
    schedulable_nodes = []
    for node in nodes:
        if node.worker:
            schedulable_nodes.append(node)
    return schedulable_nodes


def get_role_nodes(cluster, role):
    etcd_nodes = []
    control_nodes = []
    worker_nodes = []
    node_list = []
    client = get_admin_client()
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
    curl_args = " "
    if (insecure_redirect):
        curl_args = " -L --insecure "
    if len(host) > 0:
        curl_args += " --header 'Host: " + host + "'"
    nodes = get_schedulable_nodes(cluster)
    target_name_list = get_target_names(p_client, workloads)
    for node in nodes:
        host_ip = resolve_node_ip(node)
        url = "http://" + host_ip + path
        wait_until_ok(url, timeout=300, headers={
            "Host": host
        })
        cmd = curl_args + " " + url
        validate_http_response(cmd, target_name_list)


def validate_ingress_using_endpoint(p_client, ingress, workloads,
                                    timeout=300):
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
                if public_endpoint["hostname"].startswith(ingress.name):
                    fqdn_available = True
                    url = \
                        public_endpoint["protocol"].lower() + "://" + \
                        public_endpoint["hostname"]
                    if "path" in public_endpoint.keys():
                        url += public_endpoint["path"]
    time.sleep(10)
    validate_http_response(url, target_name_list)


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


def check_if_ok(url, verify=False, headers={}):
    try:
        res = requests.head(url, verify=verify, headers=headers)
        if res.status_code == 200:
            return True
        return False
    except requests.ConnectionError:
        print("Connection Error - " + url)
        return False


def validate_http_response(cmd, target_name_list, client_pod=None):
    if client_pod is None and cmd.startswith("http://"):
        wait_until_active(cmd, 60)
    target_hit_list = target_name_list[:]
    count = 5 * len(target_name_list)
    for i in range(1, count):
        if len(target_hit_list) == 0:
            break
        if client_pod is None:
            curl_cmd = "curl " + cmd
            result = run_command(curl_cmd)
        else:
            wget_cmd = "wget -qO- " + cmd
            result = kubectl_pod_exec(client_pod, wget_cmd)
            result = result.decode()
        result = result.rstrip()
        print("cmd: \t" + cmd)
        print("result: \t" + result)
        assert result in target_name_list
        if result in target_hit_list:
            target_hit_list.remove(result)
    print("After removing all, the rest is: ", target_hit_list)
    assert len(target_hit_list) == 0


def validate_cluster(client, cluster, intermediate_state="provisioning",
                     check_intermediate_state=True, skipIngresscheck=True,
                     nodes_not_in_active_state=[], k8s_version=""):
    cluster = validate_cluster_state(
        client, cluster,
        check_intermediate_state=check_intermediate_state,
        intermediate_state=intermediate_state,
        nodes_not_in_active_state=nodes_not_in_active_state)
    # Create Daemon set workload and have an Ingress with Workload
    # rule pointing to this daemonset
    create_kubeconfig(cluster)
    if k8s_version != "":
        check_cluster_version(cluster, k8s_version)
    if hasattr(cluster, 'rancherKubernetesEngineConfig'):
        check_cluster_state(len(get_role_nodes(cluster, "etcd")))
    project, ns = create_project_and_ns(ADMIN_TOKEN, cluster)
    p_client = get_project_client_for_token(project, ADMIN_TOKEN)
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    validate_workload(p_client, workload, "daemonSet", ns.name,
                      len(get_schedulable_nodes(cluster)))
    if not skipIngresscheck:
        host = "test" + str(random_int(10000, 99999)) + ".com"
        path = "/name.html"
        rule = {"host": host,
                "paths":
                    [{"workloadIds": [workload.id], "targetPort": "80"}]}
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
    expected_k8s_version = version[:version.find("-")]
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


def validate_dns_record(pod, record, expected):
    # requires pod with `dig` available - TEST_IMAGE
    host = '{0}.{1}.svc.cluster.local'.format(
        record["name"], record["namespaceId"])
    validate_dns_entry(pod, host, expected)


def validate_dns_entry(pod, host, expected):
    # requires pod with `dig` available - TEST_IMAGE
    cmd = 'ping -c 1 -W 1 {0}'.format(host)
    ping_output = kubectl_pod_exec(pod, cmd)

    ping_validation_pass = False
    for expected_value in expected:
        if expected_value in str(ping_output):
            ping_validation_pass = True
            break

    assert ping_validation_pass is True
    assert " 0% packet loss" in str(ping_output)

    dig_cmd = 'dig {0} +short'.format(host)
    dig_output = kubectl_pod_exec(pod, dig_cmd)

    for expected_value in expected:
        assert expected_value in str(dig_output)


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
                "Timed out waiting for state to get to active")
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
    cmd = cluster_token.nodeCommand
    for role in roles:
        assert role in allowed_roles
        cmd += " --" + role
    additional_options = " --address " + node.public_ip_address + \
                         " --internal-address " + node.private_ip_address
    cmd += additional_options
    return cmd


def create_custom_host_registration_token(client, cluster):
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
    # Delete Cluster
    client.delete(cluster)
    # Delete nodes(in cluster) from AWS for Imported and Custom Cluster
    if (len(nodes) > 0):
        cluster_type = get_cluster_type(client, cluster)
        print(cluster_type)
        if get_cluster_type(client, cluster) in ["Imported", "Custom"]:
            nodes = client.list_node(clusterId=cluster.id).data
            filters = [
                {'Name': 'tag:Name',
                 'Values': ['testcustom*', 'teststess*']}]
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
            for node in aws_nodes:
                print(node.public_ip_address)
                AmazonWebServices().delete_nodes(aws_nodes)


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

    cmd = "ping -c 1 -W 1 " + pod_ip
    response = kubectl_pod_exec(pod1, cmd)
    print("Actual ping Response from " + pod1.name + ":" + str(response))
    if allow_connectivity:
        assert pod_ip in str(response) and " 0% packet loss" in str(response)
    else:
        assert pod_ip in str(response) and " 100% packet loss" in str(response)


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
    time.sleep(2)
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
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        pods = p_client.list_pod(workloadId=workload.id).data
    return pods


def get_admin_client_and_cluster():
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
                           nodes_not_in_active_state=[]):
    if check_intermediate_state:
        cluster = wait_for_condition(
            client, cluster,
            lambda x: x.state == intermediate_state,
            lambda x: 'State is: ' + x.state,
            timeout=MACHINE_TIMEOUT)
        assert cluster.state == intermediate_state
    cluster = wait_for_condition(
        client, cluster,
        lambda x: x.state == "active",
        lambda x: 'State is: ' + x.state,
        timeout=MACHINE_TIMEOUT)
    assert cluster.state == "active"
    wait_for_nodes_to_become_active(client, cluster,
                                    exception_list=nodes_not_in_active_state)
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
        client.delete(cluster)
        if aws_nodes is not None:
            delete_node(aws_nodes)
    else:
        env_details = "env.CATTLE_TEST_URL='" + CATTLE_TEST_URL + "'\n"
        env_details += "env.ADMIN_TOKEN='" + ADMIN_TOKEN + "'\n"
        env_details += "env.CLUSTER_NAME='" + cluster.name + "'\n"
        create_config_file(env_details)


def create_config_file(env_details):
    file = open(env_file, "w")
    file.write(env_details)
    file.close()


def validate_hostPort(p_client, workload, source_port, cluster):
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


def validate_lb(p_client, workload):
    url = get_endpoint_url_for_workload(p_client, workload)
    target_name_list = get_target_names(p_client, [workload])
    wait_until_lb_is_active(url)
    validate_http_response(url + "/name.html", target_name_list)


def validate_nodePort(p_client, workload, cluster):
    get_endpoint_url_for_workload(p_client, workload, 60)
    wl = p_client.list_workload(uuid=workload.uuid).data[0]
    source_port = wl.publicEndpoints[0]["port"]
    nodes = get_schedulable_nodes(cluster)
    pods = p_client.list_pod(workloadId=wl.id).data
    target_name_list = []
    for pod in pods:
        target_name_list.append(pod.name)
    print("target name list:" + str(target_name_list))
    for node in nodes:
        host_ip = resolve_node_ip(node)
        curl_cmd = " http://" + host_ip + ":" + \
                   str(source_port) + "/name.html"
        validate_http_response(curl_cmd, target_name_list)


def validate_clusterIp(p_client, workload, cluster_ip, test_pods):
    pods = p_client.list_pod(workloadId=workload.id).data
    target_name_list = []
    for pod in pods:
        target_name_list.append(pod["name"])
    curl_cmd = "http://" + cluster_ip + "/name.html"
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
    output = kubectl_pod_exec(pod, cmd_write)
    assert output.strip().decode('utf-8') == ""


def validate_file_content(pod, content, filename):
    cmd_get_content = "/bin/bash -c 'cat {0}' ".format(filename)
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
    time.sleep(5)
    #When the app is deployed it goes into Active state for a short
    # period of time and then into installing/deploying.
    app_data = client.list_app(id=app_id).data
    start = time.time()
    assert len(app_data) >= 1, "Cannot find app"
    application = app_data[0]
    while application.state != "active":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)
        app = client.list_app(id=app_id).data
        assert len(app) >= 1
        application = app[0]
    return application


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
                    assert quest["type"] == "string", \
                        "Cannot set default for types other than string"
                    answer = "fake"
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
    print(questions_and_answers)
    return questions_and_answers


def validate_app_deletion(client, app_id, timeout=DEFAULT_APP_DELETION_TIMEOUT):
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
        app = client.list_app(id=app_id).data
        if len(app) == 0:
            break
