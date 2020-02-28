from .common import *  # NOQA
from rancher import ApiError

CLUSTER_NAME = os.environ.get("CLUSTER_NAME", "")

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None}


def test_cmap_create_single_ns_volume():

    """
    Create a configmap.Create and validate workload using
    the configmap as a volume. Create and validate the workload with config
    map as environment variable
    """

    p_client = namespace["p_client"]
    ns = namespace["ns"]
    value = "valueall"
    keyvaluepair = {"testall": value}
    configmap = create_configmap(keyvaluepair, p_client, ns)

    # Create workloads with configmap in existing namespace
    create_and_validate_workload_with_configmap_as_volume(p_client, configmap,
                                                          ns, keyvaluepair)


def test_cmap_create_single_ns_env_variable():

    """
    Create a configmap.Create and validate workload using
    the configmap as a volume. Create and validate the workload with config
    map as environment variable
    """

    p_client = namespace["p_client"]
    ns = namespace["ns"]
    value = "valueall"
    keyvaluepair = {"testall": value}
    configmap = create_configmap(keyvaluepair, p_client, ns)

    # Create workloads with configmap in existing namespace
    create_and_validate_workload_with_configmap_as_env_variable(p_client,
                                                                configmap,
                                                                ns,
                                                                keyvaluepair)


def test_cmap_delete_single_ns():

    # Create a configmap and delete the configmap
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    value = "valuetest"
    keyvaluepair = {"testalldelete": value}

    configmap = create_configmap(keyvaluepair, p_client, ns)
    delete_configmap(p_client, configmap, ns, keyvaluepair)


def test_cmap_edit_single_ns():

    """
    Create a configmap and update the configmap.
    Create and validate workload using the updated configmap
    """

    p_client = namespace["p_client"]
    ns = namespace["ns"]
    name = random_test_name("default")
    value = "valueall"
    keyvaluepair = {"testall": value}

    configmap = create_configmap(keyvaluepair, p_client, ns)

    value1 = ("valueall")
    value2 = ("valueallnew")
    updated_dict = {"testall": value1, "testallnew": value2}
    updated_configmap = p_client.update(configmap, name=name,
                                        namespaceId=ns['name'],
                                        data=updated_dict)
    updatedconfigmapdata = updated_configmap['data']

    assert updatedconfigmapdata.data_dict() == updated_dict

    # Create a workload with the updated configmap in the existing namespace
    create_and_validate_workload_with_configmap_as_volume(p_client, configmap,
                                                          ns,
                                                          updatedconfigmapdata)
    create_and_validate_workload_with_configmap_as_env_variable(
        p_client, configmap, ns, updatedconfigmapdata)


@if_test_rbac
def test_rbac_cmap_cluster_owner_create(remove_resource):

    """
    Verify cluster owner can create config map and deploy workload using
    config map
    """
    user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    project, ns = create_project_and_ns(user_token, namespace["cluster"],
                                        random_test_name("rbac-cluster-owner"))
    p_client = get_project_client_for_token(project, user_token)
    rbac_configmap_create(p_client, ns)
    remove_resource(project)


@if_test_rbac
def test_rbac_cmap_cluster_owner_edit(remove_resource):
    """
    Verify cluster owner can create config map and deploy workload using
    config map
    """
    user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    project, ns = create_project_and_ns(user_token, namespace["cluster"],
                                        random_test_name("rbac-cluster-owner"))
    p_client = get_project_client_for_token(project, user_token)
    rbac_configmap_edit(p_client, ns)
    remove_resource(project)


@if_test_rbac
def test_rbac_cmap_cluster_owner_delete(remove_resource):

    """
    Verify cluster owner can create config map and deploy workload using
    config map
    """
    user_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    project, ns = create_project_and_ns(user_token, namespace["cluster"],
                                        random_test_name("rbac-cluster-owner"))
    p_client = get_project_client_for_token(project, user_token)
    rbac_configmap_delete(p_client, ns)
    remove_resource(project)


@if_test_rbac
def test_rbac_cmap_cluster_member_create(remove_resource):

    """
    Verify cluster member can create config map and deploy workload
    using config map
    """

    user_token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    project, ns = create_project_and_ns(user_token, namespace["cluster"],
                                        random_test_name("rbac-cluster-mem"))
    p_client = get_project_client_for_token(project, user_token)
    rbac_configmap_create(p_client, ns)
    remove_resource(project)


@if_test_rbac
def test_rbac_cmap_cluster_member_edit(remove_resource):

    """
    Verify cluster member can create config map and deploy workload
    using config map
    """

    user_token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    project, ns = create_project_and_ns(user_token, namespace["cluster"],
                                        random_test_name("rbac-cluster-mem"))
    p_client = get_project_client_for_token(project, user_token)
    rbac_configmap_edit(p_client, ns)
    remove_resource(project)


@if_test_rbac
def test_rbac_cmap_cluster_member_delete(remove_resource):

    """
    Verify cluster member can create config map and deploy workload
    using config map
    """

    user_token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    project, ns = create_project_and_ns(user_token, namespace["cluster"],
                                        random_test_name("rbac-cluster-mem"))
    p_client = get_project_client_for_token(project, user_token)
    rbac_configmap_delete(p_client, ns)
    remove_resource(project)


@if_test_rbac
def test_rbac_cmap_project_owner_create():
    user_token = rbac_get_user_token_by_role(PROJECT_OWNER)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    p_client = get_project_client_for_token(project, user_token)
    rbac_configmap_create(p_client, ns)


@if_test_rbac
def test_rbac_cmap_project_owner_edit():
    user_token = rbac_get_user_token_by_role(PROJECT_OWNER)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    p_client = get_project_client_for_token(project, user_token)
    rbac_configmap_edit(p_client, ns)


@if_test_rbac
def test_rbac_cmap_project_owner_delete():
    user_token = rbac_get_user_token_by_role(PROJECT_OWNER)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    p_client = get_project_client_for_token(project, user_token)
    rbac_configmap_delete(p_client, ns)


@if_test_rbac
def test_rbac_cmap_project_member_create():
    user_token = rbac_get_user_token_by_role(PROJECT_MEMBER)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    p_client = get_project_client_for_token(project, user_token)
    rbac_configmap_create(p_client, ns)


@if_test_rbac
def test_rbac_cmap_project_member_edit():
    user_token = rbac_get_user_token_by_role(PROJECT_MEMBER)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    p_client = get_project_client_for_token(project, user_token)
    rbac_configmap_edit(p_client, ns)


@if_test_rbac
def test_rbac_cmap_project_member_delete():
    user_token = rbac_get_user_token_by_role(PROJECT_MEMBER)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    p_client = get_project_client_for_token(project, user_token)
    rbac_configmap_delete(p_client, ns)


@if_test_rbac
def test_rbac_cmap_project_readonly_member_create():

    project = rbac_get_project()
    ns = rbac_get_namespace()
    user_token1 = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    readonly_user_client = get_project_client_for_token(project, user_token1)

    value = "valueall"
    keyvaluepair = {"testall": value}

    # Read Only member cannot create config maps
    with pytest.raises(ApiError) as e:
        create_configmap(keyvaluepair, readonly_user_client, ns)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_cmap_project_readonly_member_edit(remove_resource):

    cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    user_token1 = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    readonly_user_client = get_project_client_for_token(project, user_token1)

    value = "valueall"
    keyvaluepair = {"testall": value}

    cluster_owner_p_client = get_project_client_for_token(project,
                                                          cluster_owner_token)
    # As a cluster owner, create a config map
    configmap = create_configmap(keyvaluepair, cluster_owner_p_client, ns)

    # Readonly member cannot edit configmap
    value1 = ("valueall")
    value2 = ("valueallnew")
    updated_dict = {"testall": value1, "testallnew": value2}

    with pytest.raises(ApiError) as e:
        readonly_user_client.update(configmap,
                                    namespaceId=ns['name'],
                                    data=updated_dict)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'

    remove_resource(configmap)


@if_test_rbac
def test_rbac_cmap_project_readonly_delete(remove_resource):

    cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    user_token1 = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    readonly_user_client = get_project_client_for_token(project, user_token1)

    value = "valueall"
    keyvaluepair = {"testall": value}

    cluster_owner_p_client = get_project_client_for_token(project,
                                                          cluster_owner_token)

    # As a cluster owner, create a config map
    configmap = create_configmap(keyvaluepair, cluster_owner_p_client, ns)

    # Assert read-only user cannot delete the config map
    with pytest.raises(ApiError) as e:
        delete_configmap(readonly_user_client, configmap, ns, keyvaluepair)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'

    remove_resource(configmap)


def rbac_configmap_create(p_client, ns):

    """
    Verify creating, editing and deleting config maps is functional.
    The p_client passed as the parameter would be as per the role assigned
    """

    value = "valueall"
    keyvaluepair = {"testall": value}
    configmap = create_configmap(keyvaluepair, p_client, ns)

    # Create workloads with configmap in existing namespace
    create_and_validate_workload_with_configmap_as_volume(p_client, configmap,
                                                          ns, keyvaluepair)


def rbac_configmap_edit(p_client, ns):

    """
    Verify creating, editing and deleting config maps is functional.
    The p_client passed as the parameter would be as per the role assigned
    """

    value = "valueall"
    keyvaluepair = {"testall": value}
    configmap = create_configmap(keyvaluepair, p_client, ns)

    # Create workloads with configmap in existing namespace
    create_and_validate_workload_with_configmap_as_volume(p_client, configmap,
                                                          ns, keyvaluepair)

    # Verify editing of configmap in the project
    value1 = ("valueall")
    value2 = ("valueallnew")
    updated_dict = {"testall": value1, "testallnew": value2}
    updated_configmap = p_client.update(configmap,
                                        namespaceId=ns['name'],
                                        data=updated_dict)
    updatedconfigmapdata = updated_configmap['data']
    create_and_validate_workload_with_configmap_as_volume(p_client, configmap,
                                                          ns,
                                                          updatedconfigmapdata)


def rbac_configmap_delete(p_client, ns):

    """
    Verify creating, editing and deleting config maps is functional.
    The p_client passed as the parameter would be as per the role assigned
    """
    value = "valueall"
    keyvaluepair = {"testall": value}
    configmap = create_configmap(keyvaluepair, p_client, ns)
    # Verify deletion of config map
    delete_configmap(p_client, configmap, ns, keyvaluepair)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testconfigmap")
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)

    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["c_client"] = c_client

    def fin():
        client = get_user_client()
        client.delete(namespace["project"])
    request.addfinalizer(fin)


def validate_workload_with_configmap(p_client, workload,
                                     type, ns_name, keyvaluepair,
                                     workloadwithconfigmapasvolume=False,
                                     workloadwitconfigmapasenvvar=False,
                                     podcount=1):

    validate_workload(p_client, workload, type, ns_name, pod_count=podcount)

    pod_list = p_client.list_pod(workloadId=workload.id).data
    mountpath = "/test"
    for i in range(0, len(keyvaluepair)):
        key = list(keyvaluepair.keys())[i]
        if workloadwithconfigmapasvolume:
            key_file_in_pod = mountpath + "/" + key
            print(key_file_in_pod)
            command = "cat " + key_file_in_pod + ''
            print(" Command to display configmap value from container is: ")
            print(command)
            result = kubectl_pod_exec(pod_list[0], command)
            assert result.decode("utf-8") == (list(keyvaluepair.values())[i])
        elif workloadwitconfigmapasenvvar:
            command = 'env'
            result = kubectl_pod_exec(pod_list[0], command)
            print(list(keyvaluepair.values())[i])
            if list(keyvaluepair.values())[i] in result.decode("utf-8"):
                assert True


def delete_configmap(client, configmap, ns, keyvaluepair):

    key = list(keyvaluepair.keys())[0]
    print("Delete Configmap")
    client.delete(configmap)
    # Sleep to allow for the configmap to be deleted
    time.sleep(5)
    timeout = 30
    configmapname = configmap.name
    print("Config Map list after deleting config map")
    configmapdict = client.list_configMap(name=configmapname)
    start = time.time()
    if len(configmapdict.get('data')) > 0:
        testdata = configmapdict.get('data')
        print("TESTDATA")
        print(testdata[0]['data'])
        while key in testdata[0]['data']:
            if time.time() - start > timeout:
                raise AssertionError("Timed out waiting for deletion")
            time.sleep(.5)
            configmapdict = client.list_configMap(name=configmapname)
            testdata = configmapdict.get('data')
        assert True

    if len(configmapdict.get('data')) == 0:
        assert True

    # Verify configmap is deleted by "kubectl get configmap" command
    command = " get configmap " + configmap['name'] + " --namespace=" + ns.name
    print("Command to obtain the configmap")
    print(command)
    result = execute_kubectl_cmd(command, json_out=False, stderr=True)
    print(result)

    print("Verify that the configmap does not exist "
          "and the error code returned is non zero ")
    if result != 0:
        assert True


def create_and_validate_workload_with_configmap_as_volume(p_client, configmap,
                                                          ns, keyvaluepair):
    workload_name = random_test_name("test")
    # Create Workload with configmap as volume
    mountpath = "/test"
    volumeMounts = [{"readOnly": False, "type": "volumeMount",
                     "mountPath": mountpath, "name": "vol1"}]
    con = [{"name": "test1",
            "image": TEST_IMAGE,
            "volumeMounts": volumeMounts}]

    configmapname = configmap['name']

    volumes = [{"type": "volume", "name": "vol1",
                "configMap": {"type": "configMapVolumeSource",
                              "defaultMode": 256,
                              "name": configmapname,
                              "optional": False}}]

    workload = p_client.create_workload(name=workload_name,
                                        containers=con,
                                        namespaceId=ns.id, volumes=volumes)
    validate_workload_with_configmap(p_client, workload, "deployment",
                                     ns.name, keyvaluepair,
                                     workloadwithconfigmapasvolume=True)

    # Delete workload
    p_client.delete(workload)


def create_and_validate_workload_with_configmap_as_env_variable(p_client,
                                                                configmap,
                                                                ns,
                                                                keyvaluepair):
    workload_name = random_test_name("test")

    # Create Workload with configmap as env variable
    configmapname = configmap['name']

    environmentdata = [{
        "source": "configMap",
        "sourceKey": None,
        "sourceName": configmapname
    }]
    con = [{"name": "test",
            "image": TEST_IMAGE,
            "environmentFrom": environmentdata}]

    workload = p_client.create_workload(name=workload_name,
                                        containers=con,
                                        namespaceId=ns.id)
    validate_workload_with_configmap(p_client, workload, "deployment",
                                     ns.name, keyvaluepair,
                                     workloadwitconfigmapasenvvar=True)
    # Delete workload
    p_client.delete(workload)


def create_configmap(keyvaluepair, p_client=None, ns=None):

    if p_client is None:
        p_client = namespace["p_client"]

    if ns is None:
        ns = namespace["ns"]

    name = random_test_name("testconfigmap")

    configmap = p_client.create_configMap(name=name, namespaceId=ns['name'],
                                          data=keyvaluepair)
    assert configmap['baseType'] == "configMap"
    print(configmap)
    configdata = configmap['data']
    assert configdata.data_dict() == keyvaluepair

    return configmap
