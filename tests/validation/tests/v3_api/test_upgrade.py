import base64
import pytest


from .common import *  # NOQA
from .test_secrets import (
    create_and_validate_workload_with_secret_as_env_variable,
    create_and_validate_workload_with_secret_as_volume,
    validate_workload_with_secret,
    create_secret)
from .test_service_discovery import create_dns_record

cluster_name = CLUSTER_NAME
validate_prefix = os.environ.get('RANCHER_VALIDATE_RESOURCES_PREFIX', "step0")
create_prefix = os.environ.get('RANCHER_CREATE_RESOURCES_PREFIX', "step1")
namespace = {"p_client": None, "ns": None, "cluster": None, "project": None,
             "testclient_pods": []}
upgrade_check_stage = os.environ.get('RANCHER_UPGRADE_CHECK', "preupgrade")
validate_ingress = \
    ast.literal_eval(os.environ.get('RANCHER_INGRESS_CHECK', "True"))

sshUser = os.environ.get('RANCHER_SSH_USER', "ubuntu")
rancherVersion = os.environ.get('RANCHER_SERVER_VERSION', "master")
upgradeVersion = os.environ.get('RANCHER_SERVER_VERSION_UPGRADE', "master")
upgradeImage = os.environ.get('RANCHER_UPGRADE_IMAGE', "rancher/rancher")

value = base64.b64encode(b"valueall")
keyvaluepair = {"testall": value.decode('utf-8')}

wl_name = "-testwl"
sd_name = "-testsd"
sd_wlname1 = "-testsd1"
sd_wlname2 = "-testsd2"
ingress_name1 = "-testingress1"
ingress_name2 = "-testingress2"
ingress_wlname1 = "-testingresswl1"
ingress_wlname2 = "-testingresswl2"
project_name = "-p1"
ns_name1 = "-ns1"
ns_name2 = "-ns2"


wl_name_create = create_prefix + wl_name
sd_name_create = create_prefix + sd_name
sd_wlname1_create = create_prefix + sd_wlname1
sd_wlname2_create = create_prefix + sd_wlname2
ingress_name1_create = create_prefix + ingress_name1
ingress_name2_create = create_prefix + ingress_name2
ingress_wlname1_create = create_prefix + ingress_wlname1
ingress_wlname2_create = create_prefix + ingress_wlname2

wl_name_validate = validate_prefix + wl_name
sd_name_validate = validate_prefix + sd_name
sd_wlname1_validate = validate_prefix + sd_wlname1
sd_wlname2_validate = validate_prefix + sd_wlname2
ingress_name1_validate = validate_prefix + ingress_name1
ingress_name2_validate = validate_prefix + ingress_name2
ingress_wlname1_validate = validate_prefix + ingress_wlname1
ingress_wlname2_validate = validate_prefix + ingress_wlname2

secret_name = create_prefix + "-testsecret"
secret_wl_name1_create = create_prefix + "-testwl1withsec"
secret_wl_name2_create = create_prefix + "-testwl2withsec"

secret_wl_name1_validate = validate_prefix + "-testwl1withsec"
secret_wl_name2_validate = validate_prefix + "-testwl2withsec"

app_ns = create_prefix + "-app-ns"
app_create_name = create_prefix + "-app"
app_validate_name = validate_prefix + "-app"
# the pre_upgrade_externalId is for launching an app
pre_upgrade_externalId = \
    create_catalog_external_id("test-catalog", "mysql", "1.3.1")
# the post_upgrade_externalId is for upgrading the existing app
post_upgrade_externalId = \
    create_catalog_external_id("test-catalog", "mysql", "1.3.2")
catalogUrl = "https://github.com/rancher/integration-test-charts.git"
catalogBranch = "validation-tests"

if_post_upgrade = pytest.mark.skipif(
    upgrade_check_stage != "postupgrade",
    reason='This test is not executed for PreUpgrade checks')
if_pre_upgrade = pytest.mark.skipif(
    upgrade_check_stage != "preupgrade",
    reason='This test is not executed for PreUpgrade checks')
if_validate_ingress = pytest.mark.skipif(
    validate_ingress is False,
    reason='This test is not executed')
if_upgrade_rancher = pytest.mark.skipif(
    upgrade_check_stage != "upgrade_rancher",
    reason='This test is only for testing upgrading Rancher')


@if_post_upgrade
@pytest.mark.run(order=1)
def test_validate_existing_project_resources():
    validate_existing_project_resources()


@if_post_upgrade
@pytest.mark.run(order=2)
def test_validate_existing_wl():
    validate_wl(wl_name_validate)


@if_post_upgrade
@pytest.mark.run(order=2)
def test_validate_existing_service_discovery():
    validate_service_discovery(sd_name_validate,
                               [sd_wlname1_validate, sd_wlname2_validate])


@if_post_upgrade
@pytest.mark.run(order=2)
def test_validate_existing_wl_with_secret():
    validate_worklaods_with_secret(
        secret_wl_name1_validate, secret_wl_name2_validate)


# It's hard to find an App to support Windows case for now.
# Could we make an App to support both Windows and Linux?
@skip_test_windows_os
@if_post_upgrade
@pytest.mark.run(order=2)
def test_validate_existing_catalog_app():
    validate_catalog_app(app_validate_name, pre_upgrade_externalId)


@if_post_upgrade
@if_validate_ingress
@pytest.mark.run(order=2)
def test_validate_existing_ingress_daemon():
    validate_ingress_xip_io(ingress_name1_validate,
                            ingress_wlname1_validate)


@if_post_upgrade
@if_validate_ingress
@pytest.mark.run(order=2)
def test_validate_existing_ingress_wl():
    validate_ingress_xip_io(ingress_name2_validate,
                            ingress_wlname2_validate)


@if_post_upgrade
@pytest.mark.run(order=3)
def test_modify_workload_validate_deployment():
    modify_workload_validate_deployment()


@if_post_upgrade
@pytest.mark.run(order=3)
def test_modify_workload_validate_sd():
    modify_workload_validate_sd()


@if_post_upgrade
@pytest.mark.run(order=3)
def test_modify_workload_validate_secret():
    modify_workload_validate_secret()


# It's hard to find an App to support Windows case for now.
# Could we make an App to support both Windows and Linux?
@skip_test_windows_os
@if_post_upgrade
@pytest.mark.run(order=3)
def test_modify_catalog_app():
    modify_catalog_app()


@if_post_upgrade
@if_validate_ingress
@pytest.mark.run(order=3)
def test_modify_workload_validate_ingress():
    modify_workload_validate_ingress()


@pytest.mark.run(order=4)
def test_create_project_resources():
    create_project_resources()


@pytest.mark.run(order=5)
def test_create_and_validate_wl():
    create_and_validate_wl()


@pytest.mark.run(order=5)
def test_create_and_validate_service_discovery():
    create_and_validate_service_discovery()


@pytest.mark.run(order=5)
def test_create_validate_wokloads_with_secret():
    create_validate_wokloads_with_secret()


@if_validate_ingress
@pytest.mark.run(order=5)
def test_create_and_validate_ingress_xip_io_daemon():
    create_and_validate_ingress_xip_io_daemon()


@if_validate_ingress
@pytest.mark.run(order=5)
def test_create_and_validate_ingress_xip_io_wl():
    create_and_validate_ingress_xip_io_wl()


# It's hard to find an App to support Windows case for now.
# Could we make an App to support both Windows and Linux?
@skip_test_windows_os
@pytest.mark.run(order=5)
def test_create_and_validate_catalog_app():
    create_and_validate_catalog_app()


@pytest.mark.run(order=6)
def test_create_and_validate_ip_address_pods():
    create_and_validate_ip_address_pods()


# the flag if_upgarde_rancher is false all the time
# because we do not have this option for the variable RANCHER_UPGRADE_CHECK
# instead, we will have a new pipeline that calls this function directly
@if_upgrade_rancher
def test_rancher_upgrade():
    upgrade_rancher_server(CATTLE_TEST_URL)
    client = get_user_client()
    version = client.list_setting(name="server-version").data[0].value
    assert version == upgradeVersion


def create_and_validate_wl():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    p_client.create_workload(name=wl_name_create, containers=con,
                             namespaceId=ns.id, scale=2)
    validate_wl(wl_name_create)


def validate_wl(workload_name, pod_count=2):
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    workloads = p_client.list_workload(name=workload_name,
                                       namespaceId=ns.id).data
    assert len(workloads) == 1
    workload = workloads[0]
    validate_workload(
        p_client, workload, "deployment", ns.name, pod_count=pod_count)
    validate_service_discovery(workload_name, [workload_name])


def create_and_validate_ingress_xip_io_daemon():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    cluster = namespace["cluster"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]

    # Ingress with daemonSet target
    workload = p_client.create_workload(name=ingress_wlname1_create,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    validate_workload(p_client, workload, "daemonSet", ns.name,
                      len(get_schedulable_nodes(cluster)))
    path = "/name.html"
    rule = {"host": "xip.io",
            "paths":
                [{"workloadIds": [workload.id], "targetPort": "80",
                  "path": path}]}
    p_client.create_ingress(name=ingress_name1_create,
                            namespaceId=ns.id,
                            rules=[rule])
    validate_ingress_xip_io(ingress_name1_create, ingress_wlname1_create)


def create_and_validate_ingress_xip_io_wl():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]

    # Ingress with Deployment target
    workload = p_client.create_workload(name=ingress_wlname2_create,
                                        containers=con,
                                        namespaceId=ns.id, scale=2)
    validate_wl(ingress_wlname2_create, 2)
    path = "/name.html"
    rule = {"host": "xip.io",
            "paths":
                [{"workloadIds": [workload.id], "targetPort": "80",
                  "path": path}]}
    p_client.create_ingress(name=ingress_name2_create,
                            namespaceId=ns.id,
                            rules=[rule])
    validate_ingress_xip_io(ingress_name2_create, ingress_wlname2_create)


def modify_workload_validate_deployment():

    # This method increments the deployment scale and validates it
    p_client = namespace["p_client"]
    ns = namespace["ns"]

    workload = p_client.list_workload(
        name=wl_name_validate, namespace=validate_prefix + ns.id).data[0]
    p_client.update(workload, scale=4, containers=workload.containers)
    validate_wl(wl_name_validate, 4)


def modify_workload_validate_ingress():

    # This method increments the workload scale and validates the ingress
    # pointing to it
    p_client = namespace["p_client"]
    ns = namespace["ns"]

    # Get workload and update
    ing_workload = p_client.list_workload(
        name=ingress_wlname2_validate, namespace=ns.id).data[0]
    print(ing_workload)
    # Increment workload
    ing_workload = p_client.update(ing_workload, scale=4,
                                   containers=ing_workload.containers)
    wait_for_pods_in_workload(p_client, ing_workload, 4)
    validate_wl(ing_workload.name, 4)

    # Validate ingress after workload scale up
    validate_ingress_xip_io(ingress_name2_validate, ingress_wlname2_validate)


def modify_workload_validate_sd():

    # This method increments the workload scale and validates
    # service discovery
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    # Get sd workloads and validate service discovery
    sd_workload = p_client.list_workload(
        name=sd_wlname2_validate, namespace=ns.id).data[0]
    p_client.update(sd_workload, scale=3, containers=sd_workload.containers)
    validate_wl(sd_wlname2_validate, 3)

    validate_service_discovery(sd_name_validate,
                               [sd_wlname1_validate, sd_wlname2_validate])


def modify_workload_validate_secret():

    # This method increments the scale of worlkoad with secret and validates it

    p_client = namespace["p_client"]
    ns = namespace["ns"]

    secret_workload1 = p_client.list_workload(
        name=secret_wl_name1_validate, namespace=ns.id).data[0]

    secret_workload1 = p_client.update(secret_workload1, scale=3,
                                       containers=secret_workload1.containers)
    wait_for_pods_in_workload(p_client, secret_workload1, 3)
    validate_workload_with_secret(
        p_client, secret_workload1, "deployment", ns.name,
        keyvaluepair, workloadwithsecretasVolume=True, podcount=3)

    secret_workload2 = p_client.list_workload(name=secret_wl_name2_validate,
                                              namespace=ns.id).data[0]

    secret_workload2 = p_client.update(secret_workload2, scale=3,
                                       containers=secret_workload2.containers)
    wait_for_pods_in_workload(p_client, secret_workload2, 3)
    validate_workload_with_secret(
        p_client, secret_workload2, "deployment", ns.name,
        keyvaluepair, workloadwithsecretasenvvar=True, podcount=3)


def validate_ingress_xip_io(ing_name, workload_name):
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    workloads = p_client.list_workload(name=workload_name,
                                       namespaceId=ns.id).data
    assert len(workloads) == 1
    workload = workloads[0]
    ingresses = p_client.list_ingress(name=ing_name,
                                      namespaceId=ns.id).data
    assert len(ingresses) == 1
    ingress = ingresses[0]

    validate_ingress_using_endpoint(p_client, ingress, [workload])


def create_and_validate_service_discovery():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    cluster = namespace["cluster"]

    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    workload = p_client.create_workload(name=sd_wlname1_create,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    validate_workload(p_client, workload, "daemonSet", ns.name,
                      len(get_schedulable_nodes(cluster)))

    additional_workload = p_client.create_workload(name=sd_wlname2_create,
                                                   containers=con,
                                                   namespaceId=ns.id,
                                                   scale=1)
    wait_for_wl_to_active(p_client, additional_workload)
    awl_pods = wait_for_pods_in_workload(p_client, additional_workload, 1)
    wait_for_pod_to_running(p_client, awl_pods[0])

    record = {"type": "dnsRecord",
              "targetWorkloadIds": [workload["id"], additional_workload["id"]],
              "name": sd_name_create,
              "namespaceId": ns.id}

    create_dns_record(record, p_client)
    validate_service_discovery(sd_name_create,
                               [sd_wlname1_create, sd_wlname2_create])


def validate_service_discovery(sd_record_name, workload_names):
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    target_wls = []
    for wl_name_create in workload_names:
        workloads = p_client.list_workload(
            name=wl_name_create, namespaceId=ns.id).data
        assert len(workloads) == 1
        workload = workloads[0]
        target_wls.append(workload)

    records = p_client.list_dns_record(
        name=sd_record_name, namespaceId=ns.id).data
    assert len(records) == 1
    record = records[0]

    testclient_pods = namespace["testclient_pods"]
    expected_ips = []
    for wl in target_wls:
        pods = p_client.list_pod(workloadId=wl["id"]).data
        for pod in pods:
            expected_ips.append(pod["status"]["podIp"])

    assert len(testclient_pods) > 0
    for pod in testclient_pods:
        validate_dns_record(pod, record, expected_ips)


def create_validate_wokloads_with_secret():

    p_client = namespace["p_client"]
    ns = namespace["ns"]

    secret_name = create_prefix + "-testsecret"

    secret_wl_name_create1 = create_prefix + "-testwl1withsec"
    secret_wl_name_create2 = create_prefix + "-testwl2withsec"

    secret = create_secret(keyvaluepair, p_client=p_client, name=secret_name)
    create_and_validate_workload_with_secret_as_volume(
        p_client, secret, ns, keyvaluepair, name=secret_wl_name_create1)
    create_and_validate_workload_with_secret_as_env_variable(
        p_client, secret, ns, keyvaluepair, name=secret_wl_name_create2)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client = get_user_client()
    admin_client = get_admin_client()
    clusters = client.list_cluster(name=cluster_name).data
    assert len(clusters) == 1
    cluster = clusters[0]
    create_kubeconfig(cluster)
    namespace["cluster"] = cluster
    if len(admin_client.list_catalog(name="test-catalog")) == 0:
        catalog = admin_client.create_catalog(
            name="test-catalog",
            baseType="catalog",
            branch=catalogBranch,
            kind="helm",
            url=catalogUrl)
        catalog = wait_for_catalog_active(admin_client, catalog)


def create_project_resources():
    cluster = namespace["cluster"]
    p, ns = create_project_and_ns(USER_TOKEN, cluster,
                                  project_name=create_prefix + project_name,
                                  ns_name=create_prefix + ns_name1)
    p_client = get_project_client_for_token(p, USER_TOKEN)

    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["project"] = p
    namespace["testclient_pods"] = []

    # Create pods in existing namespace and new namespace that will be used
    # as test clients from which DNS resolution will be tested

    wlname = create_prefix + "-testsdclient"

    con = [{"name": "test1",
            "image": TEST_IMAGE}]

    workload = p_client.create_workload(name=wlname,
                                        containers=con,
                                        namespaceId=ns.id,
                                        scale=1)
    wait_for_wl_to_active(p_client, workload)
    namespace["workload"] = workload

    pods = wait_for_pods_in_workload(p_client, workload, 1)
    pod = wait_for_pod_to_running(p_client, pods[0])
    namespace["testclient_pods"].append(pod)

    new_ns = create_ns(get_cluster_client_for_token(cluster, USER_TOKEN),
                       cluster, p, ns_name=create_prefix + ns_name2)

    workload = p_client.create_workload(name=wlname,
                                        containers=con,
                                        namespaceId=new_ns.id,
                                        scale=1)
    wait_for_wl_to_active(p_client, workload)
    pods = wait_for_pods_in_workload(p_client, workload, 1)
    pod = wait_for_pod_to_running(p_client, pods[0])
    namespace["testclient_pods"].append(pod)
    assert len(namespace["testclient_pods"]) == 2


def validate_existing_project_resources():
    cluster = namespace["cluster"]
    p_name = validate_prefix + project_name
    ns_name = validate_prefix + ns_name1
    ns2_name = validate_prefix + ns_name2

    # Get existing project
    client = get_user_client()
    projects = client.list_project(name=p_name,
                                   clusterId=cluster.id).data
    assert len(projects) == 1
    project = projects[0]

    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)
    p_client = get_project_client_for_token(project, USER_TOKEN)

    # Get existing namespace
    nss = c_client.list_namespace(name=ns_name).data
    assert len(nss) == 1
    ns = nss[0]

    # 2nd namespace
    nss = c_client.list_namespace(name=ns2_name).data
    assert len(nss) == 1
    ns2 = nss[0]

    # Get existing SD client pods
    workload_name = validate_prefix + "-testsdclient"
    workloads = p_client.list_workload(name=workload_name,
                                       namespaceId=ns.id).data
    assert len(workloads) == 1
    wl1_pods = p_client.list_pod(workloadId=workloads[0].id).data
    assert len(wl1_pods) == 1

    workload_name = validate_prefix + "-testsdclient"

    workloads = p_client.list_workload(name=workload_name,
                                       namespaceId=ns2.id).data
    assert len(workloads) == 1
    wl2_pods = p_client.list_pod(workloadId=workloads[0].id).data
    assert len(wl2_pods) == 1

    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["project"] = project
    namespace["testclient_pods"] = [wl1_pods[0], wl2_pods[0]]


def validate_worklaods_with_secret(workload_name1, workload_name2):
    p_client = namespace["p_client"]
    ns = namespace["ns"]

    wk1 = p_client.list_workload(name=workload_name1, namespace=ns.id).data[0]
    wk2 = p_client.list_workload(name=workload_name2, namespace=ns.id).data[0]
    validate_workload_with_secret(
        p_client, wk1, "deployment", ns.name, keyvaluepair,
        workloadwithsecretasVolume=True)
    validate_workload_with_secret(
        p_client, wk2, "deployment", ns.name, keyvaluepair,
        workloadwithsecretasenvvar=True)


def upgrade_rancher_server(serverIp,
                           sshKeyPath=".ssh/jenkins-rke-validation.pem",
                           containerName="rancher-server"):
    if serverIp.startswith('https://'):
        serverIp = serverIp[8:]

    stopCommand = "docker stop " + containerName
    print(exec_shell_command(serverIp, 22, stopCommand, "",
          sshUser, sshKeyPath))

    createVolumeCommand = "docker create --volumes-from " + containerName + \
                          " --name rancher-data rancher/rancher:" + \
                          rancherVersion

    print(exec_shell_command(serverIp, 22, createVolumeCommand, "",
          sshUser, sshKeyPath))

    removeCommand = "docker rm " + containerName
    print(exec_shell_command(serverIp, 22, removeCommand, "",
          sshUser, sshKeyPath))

    runCommand = "docker run -d --volumes-from rancher-data " \
                 "--restart=unless-stopped " \
                 "-p 80:80 -p 443:443 " + upgradeImage + ":" + upgradeVersion
    print(exec_shell_command(serverIp, 22, runCommand, "",
          sshUser, sshKeyPath))

    wait_until_active(CATTLE_TEST_URL)


def create_and_validate_catalog_app():
    cluster = namespace["cluster"]
    p_client = namespace['p_client']
    ns = create_ns(get_cluster_client_for_token(cluster, USER_TOKEN),
                   cluster, namespace["project"], ns_name=app_ns)
    print(pre_upgrade_externalId)
    app = p_client.create_app(
        answers=get_defaut_question_answers(get_user_client(),
                                            pre_upgrade_externalId),
        externalId=pre_upgrade_externalId,
        name=app_create_name,
        projectId=namespace["project"].id,
        prune=False,
        targetNamespace=ns.id
    )
    validate_catalog_app(app.name, pre_upgrade_externalId)


def modify_catalog_app():
    p_client = namespace["p_client"]
    app = wait_for_app_to_active(p_client, app_validate_name)
    # upgrade the catalog app to a newer version
    p_client.action(obj=app, action_name="upgrade",
                    answers=get_defaut_question_answers(
                        get_user_client(),
                        post_upgrade_externalId),
                    externalId=post_upgrade_externalId)
    validate_catalog_app(app.name, post_upgrade_externalId)


def validate_catalog_app(app_name, external_id):
    p_client = namespace["p_client"]
    app = wait_for_app_to_active(p_client, app_name)
    assert app.externalId == external_id, \
        "the version of the app is not correct"
    # check if associated workloads are active
    ns = app.targetNamespace
    pramaters = external_id.split('&')
    chart = pramaters[1].split("=")[1] + "-" + pramaters[2].split("=")[1]
    workloads = p_client.list_workload(namespaceId=ns).data
    assert len(workloads) == 1, "expected only 1 workload in the namespace"
    for wl in workloads:
        assert wl.state == "active"
        assert wl.workloadLabels.chart == chart, \
            "the chart version is wrong"


def create_and_validate_ip_address_pods():
    get_pods = "get pods --all-namespaces -o wide | grep ' 172.'"
    pods_result = execute_kubectl_cmd(get_pods, json_out=False, stderr=True)
    print(pods_result.decode('ascii'))
    assert pods_result.decode('ascii') is '', "Pods have 172 IP address"
