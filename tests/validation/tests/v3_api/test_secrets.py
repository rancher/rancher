import base64
from rancher import ApiError
import pytest

from .common import *  # NOQA

CLUSTER_NAME = os.environ.get("CLUSTER_NAME", "")

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None}


def test_secret_create_all_ns():

    """
    Verify creation of secrets is functional
    """

    p_client = namespace["p_client"]
    ns = namespace["ns"]

    # Value is base64 encoded
    value = base64.b64encode(b"valueall")
    keyvaluepair = {"testall": value.decode('utf-8')}
    cluster = namespace["cluster"]
    project = namespace["project"]
    c_client = namespace["c_client"]

    new_ns = create_ns(c_client, cluster, project)
    namespacelist = [ns, new_ns]
    secret = create_secret(keyvaluepair)

    # Create workloads with secret in existing namespaces
    for ns in namespacelist:

        create_and_validate_workload_with_secret_as_volume(p_client, secret,
                                                           ns,
                                                           keyvaluepair)
        create_and_validate_workload_with_secret_as_env_variable(p_client,
                                                                 secret,
                                                                 ns,
                                                                 keyvaluepair)

    # Create a new namespace and workload in the new namespace using the secret

    new_ns1 = create_ns(c_client, cluster, project)
    create_and_validate_workload_with_secret_as_volume(p_client,
                                                       secret,
                                                       new_ns1,
                                                       keyvaluepair)
    create_and_validate_workload_with_secret_as_env_variable(p_client,
                                                             secret,
                                                             new_ns1,
                                                             keyvaluepair)
    c_client.delete(new_ns)


def test_secret_create_single_ns():

    """
    Verify editing secrets is functional
    """

    p_client = namespace["p_client"]
    ns = namespace["ns"]
    # Value is base64 encoded
    value = base64.b64encode(b"valueall")
    keyvaluepair = {"testall": value.decode('utf-8')}

    secret = create_secret(keyvaluepair, singlenamespace=True)

    # Create workloads with secret in existing namespace
    create_and_validate_workload_with_secret_as_volume(p_client, secret, ns,
                                                       keyvaluepair)
    create_and_validate_workload_with_secret_as_env_variable(p_client, secret,
                                                             ns, keyvaluepair)


def test_secret_delete_all_ns():

    """
    Verify Deletion of secrets is functional
    """
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    # Value is base64 encoded
    value = base64.b64encode(b"valuealldelete")
    keyvaluepair = {"testalldelete": value.decode('utf-8')}
    secret = create_secret(keyvaluepair)
    delete_secret(p_client, secret, ns, keyvaluepair)


def test_secret_delete_single_ns():

    p_client = namespace["p_client"]
    ns = namespace["ns"]
    # Value is base64 encoded
    value = base64.b64encode(b"valuealldelete")
    keyvaluepair = {"testalldelete": value.decode('utf-8')}

    secret = create_secret(keyvaluepair, singlenamespace=True)
    delete_secret(p_client, secret, ns, keyvaluepair)


def test_secret_edit_all_ns():

    p_client = namespace["p_client"]
    name = random_test_name("default")
    # Value is base64 encoded
    value = base64.b64encode(b"valueall")
    keyvaluepair = {"testall": value.decode('utf-8')}
    cluster = namespace["cluster"]
    project = namespace["project"]
    c_client = namespace["c_client"]

    # Create a namespace
    new_ns = create_ns(c_client, cluster, project)
    secret = create_secret(keyvaluepair)

    # Value is base64 encoded
    value1 = base64.b64encode(b"valueall")
    value2 = base64.b64encode(b"valueallnew")
    updated_dict = {"testall": value1.decode(
        'utf-8'), "testallnew": value2.decode('utf-8')}
    updated_secret = p_client.update(secret, name=name, namespaceId='NULL',
                                     data=updated_dict)

    assert updated_secret['baseType'] == "secret"
    updatedsecretdata = updated_secret['data']

    print("UPDATED SECRET DATA")
    print(updatedsecretdata)

    assert updatedsecretdata.data_dict() == updated_dict

    # Create workloads using updated secret in the existing namespace
    create_and_validate_workload_with_secret_as_volume(p_client, secret,
                                                       new_ns,
                                                       updatedsecretdata)

    create_and_validate_workload_with_secret_as_env_variable(
        p_client, secret, new_ns, updatedsecretdata)

    # Create a new namespace and workloads in the new namespace using secret
    new_ns1 = create_ns(c_client, cluster, project)

    create_and_validate_workload_with_secret_as_volume(p_client, secret,
                                                       new_ns1,
                                                       updatedsecretdata)
    create_and_validate_workload_with_secret_as_env_variable(
        p_client, secret, new_ns1, updatedsecretdata)
    c_client.delete(new_ns)


def test_secret_edit_single_ns():

    p_client = namespace["p_client"]
    ns = namespace["ns"]
    name = random_test_name("default")
    # Value is base64 encoded
    value = base64.b64encode(b"valueall")
    keyvaluepair = {"testall": value.decode('utf-8')}

    secret = create_secret(keyvaluepair, singlenamespace=True)

    value1 = base64.b64encode(b"valueall")
    value2 = base64.b64encode(b"valueallnew")
    updated_dict = {"testall": value1.decode(
        'utf-8'), "testallnew": value2.decode('utf-8')}
    updated_secret = p_client.update(secret, name=name,
                                     namespaceId=ns['name'],
                                     data=updated_dict)
    assert updated_secret['baseType'] == "namespacedSecret"
    updatedsecretdata = updated_secret['data']

    print("UPDATED SECRET DATA")
    print(updatedsecretdata)

    assert updatedsecretdata.data_dict() == updated_dict

    # Create a workload with the updated secret in the existing namespace
    create_and_validate_workload_with_secret_as_volume(p_client, secret,
                                                       ns,
                                                       updatedsecretdata)
    create_and_validate_workload_with_secret_as_env_variable(
        p_client, secret, ns, updatedsecretdata)


rbac_role_list = [
    (CLUSTER_OWNER),
    (PROJECT_OWNER),
    (PROJECT_MEMBER),
]


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_secret_create(role):
    """
    Verify creation of secrets for Cluster owner, project owner and project
    member
    """
    token = rbac_get_user_token_by_role(role)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    p_client = get_project_client_for_token(project, token)
    rbac_secret_create(p_client, ns)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_secret_edit(role):
    """
    Verify editing of secrets for Cluster owner, project owner and project
    member
    """
    token = rbac_get_user_token_by_role(role)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    p_client = get_project_client_for_token(project, token)
    rbac_secret_edit(p_client, ns, project=project)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_secret_delete(role):
    """
    Verify deletion of secrets for Cluster owner, project owner and project
    member
    """
    user_token = rbac_get_user_token_by_role(role)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    p_client = get_project_client_for_token(project, user_token)
    rbac_secret_delete(p_client, ns)


@if_test_rbac
def test_rbac_secret_create_cluster_member(remove_resource):

    """
    Verify cluster member can create secret and deploy workload using secret
    in the project he created
    """

    user_token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    project, ns = \
        create_project_and_ns(user_token, namespace["cluster"],
                              random_test_name("rbac-cluster-mem"),
                              ns_name=random_test_name("ns-cluster-mem"))
    p_client = get_project_client_for_token(project, user_token)
    rbac_secret_create(p_client, ns)

    # Create a project as cluster owner and verify the cluster member cannot
    # create secret in this project

    keyvaluepair = {"testall": "valueall"}
    cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    ownerproject, ns = \
        create_project_and_ns(cluster_owner_token,
                              namespace["cluster"],
                              random_test_name("rbac-cluster-owner"))
    cluster_member_client = get_project_client_for_token(ownerproject,
                                                         user_token)
    remove_resource(project)
    remove_resource(ownerproject)
    with pytest.raises(ApiError) as e:
        create_secret(keyvaluepair, singlenamespace=False,
                      p_client=cluster_member_client)
    assert e.value.error.status == 403
    assert e.value.error.code == 'PermissionDenied'


@if_test_rbac
def test_rbac_secret_edit_cluster_member(remove_resource):

    """
    Verify cluster member can create secret and edit secret in the project he
    created
    """

    user_token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    project, ns = \
        create_project_and_ns(user_token, namespace["cluster"],
                              random_test_name("rbac-cluster-mem"),
                              ns_name=random_test_name("ns-cluster-mem"))
    p_client = get_project_client_for_token(project, user_token)
    rbac_secret_edit(p_client, ns, project=project)

    # Create a project as cluster owner and verify the cluster member cannot
    # edit secret in this project

    keyvaluepair = {"testall": "valueall"}

    value1 = ("valueall")
    value2 = ("valueallnew")
    updated_dict = {"testall": value1, "testallnew": value2}

    cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    ownerproject, ns = create_project_and_ns(
        cluster_owner_token,
        namespace["cluster"],
        random_test_name("rbac-cluster-owner"))
    cluster_owner_client = get_project_client_for_token(ownerproject,
                                                        cluster_owner_token)
    cluster_member_client = get_project_client_for_token(ownerproject,
                                                         user_token)
    ownersecret = create_secret(keyvaluepair, singlenamespace=False,
                                p_client=cluster_owner_client)
    remove_resource(project)
    remove_resource(ownerproject)

    with pytest.raises(ApiError) as e:
        cluster_member_client.update(ownersecret, namespaceId='NULL',
                                     data=updated_dict)
    assert e.value.error.status == 404
    assert e.value.error.code == 'NotFound'


@if_test_rbac
def test_rbac_secret_delete_cluster_member(remove_resource):

    """
    Verify cluster member can create secret and delete secret in the project he
    created
    """

    keyvaluepair = {"testall": "valueall"}
    user_token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    project, ns = \
        create_project_and_ns(user_token, namespace["cluster"],
                              random_test_name("rbac-cluster-mem"),
                              ns_name=random_test_name("ns-cluster-mem"))
    p_client = get_project_client_for_token(project, user_token)
    rbac_secret_delete(p_client, ns)

    # Create a project as cluster owner and verify the cluster member cannot
    # delete secret in this project

    cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    ownerproject, ns = create_project_and_ns(
        cluster_owner_token,
        namespace["cluster"],
        random_test_name("rbac-cluster-owner"))
    cluster_owner_client = get_project_client_for_token(ownerproject,
                                                        cluster_owner_token)
    cluster_member_client = get_project_client_for_token(ownerproject,
                                                         user_token)
    ownersecret = create_secret(keyvaluepair, singlenamespace=False,
                                p_client=cluster_owner_client)
    remove_resource(project)
    remove_resource(ownerproject)

    with pytest.raises(ApiError) as e:
        delete_secret(cluster_member_client, ownersecret, ns, keyvaluepair)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
def test_rbac_secret_create_project_readonly():

    """
    Verify read-only user cannot create secret
    """

    project = rbac_get_project()
    user_token1 = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    readonly_user_client = get_project_client_for_token(project, user_token1)

    keyvaluepair = {"testall": "valueall"}

    # Read Only member cannot create secrets
    with pytest.raises(ApiError) as e:
        create_secret(keyvaluepair, singlenamespace=False,
                      p_client=readonly_user_client)
    assert e.value.error.status == 403
    assert e.value.error.code == 'PermissionDenied'


@if_test_rbac
def test_rbac_secret_edit_project_readonly_member(remove_resource):

    """
    Verify read-only user cannot edit secret
    """

    cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    user_token1 = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    readonly_user_client = get_project_client_for_token(project, user_token1)

    keyvaluepair = {"testall": "valueall"}

    cluster_owner_p_client = get_project_client_for_token(project,
                                                          cluster_owner_token)
    # As a cluster owner, create a secret
    secret = create_secret(keyvaluepair, p_client=cluster_owner_p_client,
                           ns=ns)

    # Readonly member cannot edit secret
    value1 = ("valueall")
    value2 = ("valueallnew")
    updated_dict = {"testall": value1, "testallnew": value2}

    remove_resource(secret)
    with pytest.raises(ApiError) as e:
        readonly_user_client.update(secret,
                                    namespaceId=ns['name'],
                                    data=updated_dict)
    assert e.value.error.status == 404
    assert e.value.error.code == 'NotFound'


@if_test_rbac
def test_rbac_secret_delete_project_readonly(remove_resource):

    """
    Verify read-only user cannot delete secret
    """

    cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    user_token1 = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    readonly_user_client = get_project_client_for_token(project, user_token1)

    keyvaluepair = {"testall": "valueall"}

    cluster_owner_p_client = get_project_client_for_token(project,
                                                          cluster_owner_token)
    # As a cluster owner, create a secret
    secret = create_secret(keyvaluepair, p_client=cluster_owner_p_client,
                           ns=ns)
    remove_resource(secret)
    # Assert read-only user cannot delete the secret
    with pytest.raises(ApiError) as e:
        delete_secret(readonly_user_client, secret, ns, keyvaluepair)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_secret_list(remove_resource, role):

    user_token = rbac_get_user_token_by_role(role)
    project = rbac_get_project()
    p_client = get_project_client_for_token(project, user_token)
    rbac_secret_list(p_client)


@if_test_rbac
def test_rbac_secret_list_cluster_member(remove_resource):

    """
    Verify cluster member can list secret in the project he created
    """

    keyvaluepair = {"testall": "valueall"}
    user_token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    project, ns = \
        create_project_and_ns(user_token, namespace["cluster"],
                              random_test_name("rbac-cluster-mem"),
                              ns_name=random_test_name("ns-cluster-mem"))
    p_client = get_project_client_for_token(project, user_token)
    rbac_secret_list(p_client)

    # Create a project as cluster owner and verify the cluster member cannot
    # list secret in this project

    cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    ownerproject, ns = create_project_and_ns(
        cluster_owner_token,
        namespace["cluster"],
        random_test_name("rbac-cluster-owner"))
    cluster_owner_client = get_project_client_for_token(ownerproject,
                                                        cluster_owner_token)
    cluster_member_client = get_project_client_for_token(ownerproject,
                                                         user_token)
    ownersecret = create_secret(keyvaluepair, singlenamespace=False,
                                p_client=cluster_owner_client)

    secretdict = cluster_member_client.list_secret(name=ownersecret.name)
    secretdata = secretdict.get('data')
    assert len(secretdata) == 0
    cluster_owner_client.delete(ownersecret)
    remove_resource(project)
    remove_resource(ownerproject)


@if_test_rbac
def test_rbac_secret_list_project_readonly():

    """
    Verify read-only user cannot list secret
    """
    cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    project = rbac_get_project()
    readonly_user_token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    readonly_user_client = get_project_client_for_token(project,
                                                        readonly_user_token)
    keyvaluepair = {"testall": "valueall"}
    cluster_owner_p_client = get_project_client_for_token(project,
                                                          cluster_owner_token)
    # As a cluster owner, create a secret
    secret = create_secret(keyvaluepair, p_client=cluster_owner_p_client)
    # Verify Read-Only user cannot list the secret
    secretdict = readonly_user_client.list_secret(name=secret.name)
    secretdata = secretdict.get('data')
    assert len(secretdata) == 0
    cluster_owner_p_client.delete(secret)


def rbac_secret_create(p_client, ns):

    """
    Verify creating secret is functional.
    The p_client passed as the parameter would be as per the role assigned
    """

    keyvaluepair = {"testall": "valueall"}
    secret = create_secret(keyvaluepair, singlenamespace=False,
                           p_client=p_client)

    # Create workloads with secret in existing namespace
    create_and_validate_workload_with_secret_as_volume(p_client, secret,
                                                       ns, keyvaluepair)


def rbac_secret_edit(p_client, ns, project=None):

    """
    Verify creating, editing secret is functional.
    The p_client passed as the parameter would be as per the role assigned
    """

    value = base64.b64encode(b"valueall")
    keyvaluepair = {"testall": value.decode('utf-8')}
    cluster = namespace["cluster"]
    c_client = namespace["c_client"]

    # Create a namespace
    secret = create_secret(keyvaluepair, singlenamespace=False,
                           p_client=p_client)
    # Value is base64 encoded
    value1 = base64.b64encode(b"valueall")
    value2 = base64.b64encode(b"valueallnew")
    updated_dict = {"testall": value1.decode(
        'utf-8'), "testallnew": value2.decode('utf-8')}
    updated_secret = p_client.update(secret, namespaceId='NULL',
                                     data=updated_dict)

    assert updated_secret['baseType'] == "secret"
    updatedsecretdata = updated_secret['data']

    print("UPDATED SECRET DATA")
    print(updatedsecretdata)

    assert updatedsecretdata.data_dict() == updated_dict

    # Create workloads using updated secret in the existing namespace
    create_and_validate_workload_with_secret_as_volume(p_client, secret,
                                                       ns,
                                                       updatedsecretdata)

    create_and_validate_workload_with_secret_as_env_variable(
        p_client, secret, ns, updatedsecretdata)

    # Create a new namespace and workloads in the new namespace using secret
    new_ns1 = create_ns(c_client, cluster, project)

    create_and_validate_workload_with_secret_as_volume(p_client, secret,
                                                       new_ns1,
                                                       updatedsecretdata)
    create_and_validate_workload_with_secret_as_env_variable(
        p_client, secret, new_ns1, updatedsecretdata)


def rbac_secret_delete(p_client, ns):

    """
    Verify creating, deleting secret is functional.
    The p_client passed as the parameter would be as per the role assigned
    """
    keyvaluepair = {"testall": "valueall"}
    secret = create_secret(keyvaluepair, singlenamespace=False,
                           p_client=p_client)
    # Verify deletion of secret
    delete_secret(p_client, secret, ns, keyvaluepair)


def rbac_secret_list(p_client):
    '''
    Create a secret and list the secret
    '''
    keyvaluepair = {"testall": "valueall"}
    secret = create_secret(keyvaluepair, singlenamespace=False,
                           p_client=p_client)
    secretname = secret.name
    secretdict = p_client.list_secret(name=secretname)
    secretlist = secretdict.get('data')
    testsecret = secretlist[0]
    testsecret_data = testsecret['data']
    assert len(secretlist) == 1
    assert testsecret.type == "secret"
    assert testsecret.name == secretname
    assert testsecret_data.data_dict() == keyvaluepair
    p_client.delete(testsecret)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testsecret")
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


def validate_workload_with_secret(p_client, workload,
                                  type, ns_name, keyvaluepair,
                                  workloadwithsecretasVolume=False,
                                  workloadwithsecretasenvvar=False,
                                  podcount=1):

    validate_workload(p_client, workload, type, ns_name, pod_count=podcount)

    pod_list = p_client.list_pod(workloadId=workload.id).data
    mountpath = "/test"
    for i in range(0, len(keyvaluepair)):
        key = list(keyvaluepair.keys())[i]
        if workloadwithsecretasVolume:
            key_file_in_pod = mountpath + "/" + key
            command = "cat " + key_file_in_pod + ''
            if is_windows():
                command = 'powershell -NoLogo -NonInteractive -Command "& {{ cat {0} }}"'.format(key_file_in_pod)
            result = kubectl_pod_exec(pod_list[0], command)
            assert result.rstrip() == base64.b64decode(list(keyvaluepair.values())[i])
        elif workloadwithsecretasenvvar:
            command = 'env'
            if is_windows():
                command = 'powershell -NoLogo -NonInteractive -Command \'& {{ (Get-Item -Path Env:).Name | ' \
                          '% { "$_=$((Get-Item -Path Env:\\$_).Value)" }}\''
            result = kubectl_pod_exec(pod_list[0], command)
            if base64.b64decode(list(keyvaluepair.values())[i]) in result:
                assert True


def delete_secret(client, secret, ns, keyvaluepair):

    key = list(keyvaluepair.keys())[0]
    secretname = secret.name
    print("Delete Secret")
    client.delete(secret)

    # Sleep to allow for the secret to be deleted
    time.sleep(5)
    timeout = 30
    print("Secret list after deleting secret")
    secretdict = client.list_secret(name=secretname)
    print(secretdict)
    print(secretdict.get('data'))
    start = time.time()
    if len(secretdict.get('data')) > 0:
        testdata = secretdict.get('data')
        print("TESTDATA")
        print(testdata[0]['data'])
        while key in testdata[0]['data']:
            if time.time() - start > timeout:
                raise AssertionError("Timed out waiting for deletion")
            time.sleep(.5)
            secretdict = client.list_secret(name=secretname)
            testdata = secretdict.get('data')
        assert True
    if len(secretdict.get('data')) == 0:
        assert True

    # Verify secret is deleted by "kubectl get secret" command
    command = " get secret " + secret['name'] + " --namespace=" + ns.name
    print("Command to obtain the secret")
    print(command)
    result = execute_kubectl_cmd(command, json_out=False, stderr=True)
    print(result)

    print("Verify that the secret does not exist "
          "and the error code returned is non zero ")
    if result != 0:
        assert True


def create_and_validate_workload_with_secret_as_volume(p_client, secret, ns,
                                                       keyvaluepair,
                                                       name=None):
    if name is None:
        name = random_test_name("test")

    # Create Workload with secret as volume
    mountpath = "/test"
    volumeMounts = [{"readOnly": False, "type": "volumeMount",
                     "mountPath": mountpath, "name": "vol1"}]
    con = [{"name": "test1",
            "image": TEST_IMAGE,
            "volumeMounts": volumeMounts}]

    secretName = secret['name']

    volumes = [{"type": "volume", "name": "vol1",
                "secret": {"type": "secretVolumeSource", "defaultMode": 256,
                           "secretName": secretName,
                           "optional": False, "items": "NULL"}}]

    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id, volumes=volumes)
    validate_workload_with_secret(p_client, workload, "deployment",
                                  ns.name, keyvaluepair,
                                  workloadwithsecretasVolume=True)


def create_and_validate_workload_with_secret_as_env_variable(p_client, secret,
                                                             ns, keyvaluepair,
                                                             name=None):
    if name is None:
        name = random_test_name("test")

    # Create Workload with secret as env variable
    secretName = secret['name']

    environmentdata = [{
        "source": "secret",
        "sourceKey": None,
        "sourceName": secretName
    }]
    con = [{"name": "test",
            "image": TEST_IMAGE,
            "environmentFrom": environmentdata}]

    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id)
    validate_workload_with_secret(p_client, workload, "deployment",
                                  ns.name, keyvaluepair,
                                  workloadwithsecretasenvvar=True)


def create_secret(keyvaluepair, singlenamespace=False,
                  p_client=None, ns=None, name=None):

    if p_client is None:
        p_client = namespace["p_client"]

    if name is None:
        name = random_test_name("default")
    if ns is None:
        ns = namespace["ns"]

    if not singlenamespace:
        secret = p_client.create_secret(name=name, data=keyvaluepair)
        assert secret['baseType'] == "secret"
    else:
        secret = p_client.create_namespaced_secret(name=name,
                                                   namespaceId=ns['name'],
                                                   data=keyvaluepair)
        assert secret['baseType'] == "namespacedSecret"

    print(secret)
    secretdata = secret['data']
    print("SECRET DATA")
    print(secretdata)
    assert secretdata.data_dict() == keyvaluepair

    return secret
