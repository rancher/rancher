"""
This file contains test related to add registry and
deploying workloads with those registry.
Test requirement:
Below Env variables need to set
CATTLE_TEST_URL - url to rancher server
ADMIN_TOKEN - Admin token from rancher
USER_TOKEN - User token from rancher
RANCHER_CLUSTER_NAME - Cluster name to run test on
RANCHER_REGISTRY - quay.io, dockerhub, custom etc
RANCHER_TEST_CLIENT_IMAGE - Path to image eg. quay.io/myimage/ubuntuimage
RANCHER_TEST_RBAC - Boolean (Optional), To run rbac tests
"""

from .common import *  # NOQA

CLUSTER_NAME = os.environ.get("CLUSTER_NAME", "")
namespace = {"p_client": None, "ns": None, "cluster": None, "project": None}
REGISTRY_USER_NAME = os.environ.get('RANCHER_REGISTRY_USER_NAME', "None")
REGISTRY_PASSWORD = os.environ.get('RANCHER_REGISTRY_PASSWORD', "None")
TEST_CLIENT_IMAGE = os.environ.get('RANCHER_TEST_CLIENT_IMAGE', "None")
REGISTRY = os.environ.get('RANCHER_REGISTRY', "None")

rbac_role_list = [
                  CLUSTER_OWNER,
                  CLUSTER_MEMBER,
                  PROJECT_OWNER,
                  PROJECT_MEMBER,
                  PROJECT_READ_ONLY
                 ]


def test_create_registry_single_namespace():
    """
    This test creates a namespace and creates a registry.
    Validates the workload created in same namespace using registry image
    comes up active.
    """
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    registry = create_registry_validate_workload(p_client, ns)
    delete_registry(p_client, registry)


def test_create_registry_all_namespace():
    p_client = namespace["p_client"]
    c_client = namespace["c_client"]
    cluster = namespace["cluster"]
    project = namespace["project"]

    ns = namespace["ns"]
    registry = create_registry_validate_workload(p_client, ns, allns=True)

    # Create and validate workload in a new namespace
    new_ns = create_ns(c_client, cluster, project)
    create_validate_workload(p_client, new_ns)
    delete_registry(p_client, registry)


def test_delete_registry_all_namespace():
    p_client = namespace["p_client"]
    c_client = namespace["c_client"]
    cluster = namespace["cluster"]
    project = namespace["project"]
    ns = namespace["ns"]
    new_ns = create_ns(c_client, cluster, project)

    registry = create_registry_validate_workload(p_client, ns, allns=True)
    delete_registry(p_client, registry)

    print("Verify workloads cannot be created in all the namespaces")
    create_validate_workload_with_invalid_registry(p_client, ns)
    create_validate_workload_with_invalid_registry(p_client, new_ns)


def test_delete_registry_single_namespace():
    p_client = namespace["p_client"]
    ns = namespace["ns"]

    registry = create_registry_validate_workload(p_client, ns)
    delete_registry(p_client, registry)

    print("Verify workload cannot be created in the namespace after registry")
    "deletion"
    create_validate_workload_with_invalid_registry(p_client, ns)


def test_edit_registry_single_namespace():
    p_client = namespace["p_client"]
    ns = namespace["ns"]

    name = random_test_name("registry")
    # Create registry with invalid username and password
    registries = {REGISTRY: {"username": "testabc",
                             "password": "abcdef"}}

    registry = p_client.create_namespacedDockerCredential(
        registries=registries, name=name,
        namespaceId=ns.id)

    create_validate_workload_with_invalid_registry(p_client, ns)

    # Update registry with valid username and password
    new_registries = {REGISTRY: {"username": REGISTRY_USER_NAME,
                                 "password": REGISTRY_PASSWORD}}
    p_client.update(registry, name=registry.name,
                    namespaceId=ns['name'],
                    registries=new_registries)

    # Validate workload after registry update
    create_validate_workload(p_client, ns)
    delete_registry(p_client, registry)


def test_edit_registry_all_namespace():
    p_client = namespace["p_client"]
    c_client = namespace["c_client"]
    cluster = namespace["cluster"]
    project = namespace["project"]
    ns = namespace["ns"]

    name = random_test_name("registry")

    # Create registry with invalid username and password
    registries = {REGISTRY: {"username": "testabc",
                             "password": "abcdef"}}

    registry = p_client.create_dockerCredential(
        registries=registries, name=name)

    create_validate_workload_with_invalid_registry(p_client, ns)

    # Update registry with correct username and password
    new_registries = {REGISTRY: {"username": REGISTRY_USER_NAME,
                                 "password": REGISTRY_PASSWORD}}

    p_client.update(registry, name=registry.name,
                    registries=new_registries)

    new_ns = create_ns(c_client, cluster, project)

    # Validate workload in all namespaces after registry update
    create_validate_workload(p_client, ns)
    create_validate_workload(p_client, new_ns)
    delete_registry(p_client, registry)


def test_cross_namespace_registry_access():
    """
    This test creates two namespace and creates registry for first namespace.
    It creates two workload in each namespace.
    Validates workload created in first namespace comes up healthy but workload
    in 2nd workspace doesn't become healthy.
    """
    p_client = namespace["p_client"]
    c_client = namespace["c_client"]
    cluster = namespace["cluster"]
    project = namespace["project"]
    ns_1 = namespace["ns"]
    ns_2 = create_ns(c_client, cluster, project)
    registry = create_registry_validate_workload(p_client, ns_1)
    create_validate_workload_with_invalid_registry(p_client, ns_2)
    delete_registry(p_client, registry)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_create_registry_single_namespace(role):
    """
    Creates registry with given role to a single namespace
    Runs only if RANCHER_TEST_RBAC is True in env. variable
    @param role: User role in rancher eg. project owner, project member etc
    """
    token = rbac_get_user_token_by_role(role)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    p_client = get_project_client_for_token(project, token)
    if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
        with pytest.raises(ApiError) as e:
            registry = create_registry_validate_workload(p_client, ns)
        assert e.value.error.status == 403
        assert e.value.error.code == 'PermissionDenied'
    else:
        registry = create_registry_validate_workload(p_client, ns)
        delete_registry(p_client, registry)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_create_registry_all_namespace(role):
    """
    Runs only if RANCHER_TEST_RBAC is True in env. variable.
    Creates registry scoped all namespace and for
    multiple role passed in parameter.
    @param role: User role in rancher eg. project owner, project member etc
    """
    c_client = namespace["c_client"]
    cluster = namespace["cluster"]
    token = rbac_get_user_token_by_role(role)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    p_client = get_project_client_for_token(project, token)
    if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
        with pytest.raises(ApiError) as e:
            registry = \
                create_registry_validate_workload(p_client, ns, allns=True)
        assert e.value.error.status == 403
        assert e.value.error.code == 'PermissionDenied'
    else:
        registry = create_registry_validate_workload(p_client, ns, allns=True)

        # Create and validate workload in a new namespace
        new_ns = create_ns(c_client, cluster, project)
        create_validate_workload(p_client, new_ns)
        delete_registry(p_client, registry)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_delete_registry_single_namespace(role):
    """
    Runs only if RANCHER_TEST_RBAC is True in env. variable.
    Creates a registry for single namespace, deploys a workload
    and delete registry afterwards.
    Validates the workload which has become invalid on registry deletion.
    @param role: User role in rancher eg. project owner, project member etc
    """
    c_owner_token = rbac_get_user_token_by_role(rbac_role_list[0])
    project = rbac_get_project()
    p_client_for_c_owner = get_project_client_for_token(project, c_owner_token)
    token = rbac_get_user_token_by_role(role)
    ns = rbac_get_namespace()
    p_client = get_project_client_for_token(project, token)

    registry = create_registry_validate_workload(p_client_for_c_owner, ns)

    if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
        with pytest.raises(ApiError) as e:
            delete_registry(p_client, registry)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
        delete_registry(p_client_for_c_owner, registry)
    else:
        delete_registry(p_client, registry)

        print("Verify workload cannot be created in the namespace after "
              "registry deletion")
        create_validate_workload_with_invalid_registry(p_client, ns)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_delete_registry_all_namespace(role):
    """
    Runs only if RANCHER_TEST_RBAC is True in env. variable.
    Creates a registry scoped for all namespace, deploys a workload
    and delete registry afterwards.
    Validates the workload which has become invalid on registry deletion.
    @param role: User role in rancher eg. project owner, project member etc
    """
    c_client = namespace["c_client"]
    cluster = namespace["cluster"]
    c_owner_token = rbac_get_user_token_by_role(rbac_role_list[0])
    project = rbac_get_project()
    p_client_for_c_owner = get_project_client_for_token(project, c_owner_token)
    token = rbac_get_user_token_by_role(role)
    p_client = get_project_client_for_token(project, token)
    ns = rbac_get_namespace()
    new_ns = create_ns(c_client, cluster, project)
    registry = \
        create_registry_validate_workload(p_client_for_c_owner, ns)
    if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
        with pytest.raises(ApiError) as e:
            delete_registry(p_client, registry)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
        delete_registry(p_client_for_c_owner, registry)
    else:
        delete_registry(p_client, registry)

        print("Verify workloads cannot be created in all the namespaces")
        create_validate_workload_with_invalid_registry(p_client, ns)
        create_validate_workload_with_invalid_registry(p_client, new_ns)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_edit_registry_single_namespace(role):
    """
    Runs only if RANCHER_TEST_RBAC is True in env. variable.
    Creates registry with invalid credential for single namespace,
    deploys workload with invalid registry and validate the workload
    is not up.
    Update the registry with correct credential and validates workload.
    @param role: User role in rancher eg. project owner, project member etc
    """
    c_owner_token = rbac_get_user_token_by_role(rbac_role_list[0])
    project = rbac_get_project()
    p_client_for_c_owner = get_project_client_for_token(project, c_owner_token)
    token = rbac_get_user_token_by_role(role)
    ns = rbac_get_namespace()
    p_client = get_project_client_for_token(project, token)

    name = random_test_name("registry")
    # registry with invalid username and password
    registries = {REGISTRY: {"username": "testabc",
                             "password": "abcdef"}}
    # registry with valid username and password
    new_registries = {REGISTRY: {"username": REGISTRY_USER_NAME,
                                 "password": REGISTRY_PASSWORD}}
    # Create registry with wrong credentials
    registry = p_client_for_c_owner.create_namespacedDockerCredential(
        registries=registries, name=name, namespaceId=ns.id)

    if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
        with pytest.raises(ApiError) as e:
            p_client.update(registry, name=registry.name,
                            namespaceId=ns['name'],
                            registries=new_registries)
        assert e.value.error.status == 404
        assert e.value.error.code == 'NotFound'
        delete_registry(p_client_for_c_owner, registry)
    else:
        create_validate_workload_with_invalid_registry(p_client, ns)
        # Update registry with valid username and password
        p_client.update(registry, name=registry.name,
                        namespaceId=ns['name'],
                        registries=new_registries)

        # Validate workload after registry update
        create_validate_workload(p_client, ns)
        delete_registry(p_client, registry)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_edit_registry_all_namespace(role):
    """
    Runs only if RANCHER_TEST_RBAC is True in env. variable.
    Creates registry with invalid credential scoped for all namespace,
    deploys workload with invalid registry and validate the workload
    is not up.
    Update the registry with correct credential and validates workload.
    @param role: User role in rancher eg. project owner, project member etc
    @param role:
    """
    c_client = namespace["c_client"]
    cluster = namespace["cluster"]
    c_owner_token = rbac_get_user_token_by_role(rbac_role_list[0])
    project = rbac_get_project()
    p_client_for_c_owner = get_project_client_for_token(project, c_owner_token)
    token = rbac_get_user_token_by_role(role)
    project = rbac_get_project()
    p_client = get_project_client_for_token(project, token)
    ns = rbac_get_namespace()

    name = random_test_name("registry")
    registries = {REGISTRY: {"username": "testabc",
                             "password": "abcdef"}}
    new_registries = {REGISTRY: {"username": REGISTRY_USER_NAME,
                                 "password": REGISTRY_PASSWORD}}
    # Create registry with invalid username and password
    registry = p_client_for_c_owner.create_dockerCredential(
        registries=registries, name=name)

    if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
        with pytest.raises(ApiError) as e:
            p_client.update(registry, name=registry.name,
                            registries=new_registries)
        assert e.value.error.status == 404
        assert e.value.error.code == 'NotFound'
        delete_registry(p_client_for_c_owner, registry)
    else:
        create_validate_workload_with_invalid_registry(p_client, ns)
        # Update registry with correct username and password
        p_client.update(registry, name=registry.name,
                        registries=new_registries)

        new_ns = create_ns(c_client, cluster, project)

        # Validate workload in all namespaces after registry update
        create_validate_workload(p_client, ns)
        create_validate_workload(p_client, new_ns)
        delete_registry(p_client_for_c_owner, registry)


@if_test_rbac
def test_rbac_cross_project_registry_access():
    """
    Get project1 and namespace1 from project owner role
    Creates project2 and namespace2 using same user.
    Creates registry in project1 and try to creates workload
    in project2.
    """
    cluster = namespace["cluster"]
    token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    p_client = get_project_client_for_token(project, token)

    project_2, ns_2 = create_project_and_ns(token, cluster, "testproject2")
    p2_client = get_project_client_for_token(project_2, token)
    # Create registry in project 1
    registry = create_registry_validate_workload(p_client, ns, allns=True)
    # Create workload in project 2 and validate
    create_validate_workload_with_invalid_registry(p2_client, ns_2)
    delete_registry(p_client, registry)
    p_client.delete(project_2)


def delete_registry(client, registry):

    c_client = namespace["c_client"]
    project = namespace["project"]
    print("Project ID")
    print(project.id)
    registryname = registry.name
    client.delete(registry)
    time.sleep(5)
    # Sleep to allow for the registry to be deleted
    print("Registry list after deleting registry")
    registrydict = client.list_dockerCredential(name=registryname).data
    print(registrydict)
    assert len(registrydict) == 0, "Unable to delete registry"

    namespacedict = c_client.list_namespace(projectId=project.id).data
    print("List of namespaces")
    print(namespacedict)
    len_namespace = len(namespacedict)
    namespaceData = namespacedict

    # Registry is essentially a secret, deleting the registry should delete
    # the secret. Verify secret is deleted by "kubectl get secret" command
    # for each of the namespaces
    for i in range(0, len_namespace):
        ns_name = namespaceData[i]['name']
        print(i, ns_name)

        command = " get secret " + registryname + " --namespace=" + ns_name
        print("Command to obtain the secret")
        print(command)
        result = execute_kubectl_cmd(command, json_out=False, stderr=True)
        print(result)

        print("Verify that the secret does not exist "
              "and the error code returned is non zero ")
        assert result != 0, "Error code is 0!"


def create_registry_validate_workload(p_client, ns=None, allns=False):

    name = random_test_name("registry")
    print(REGISTRY_USER_NAME)
    print(REGISTRY_PASSWORD)
    registries = {REGISTRY: {"username": REGISTRY_USER_NAME,
                             "password": REGISTRY_PASSWORD}}
    if allns:
        registry = p_client.create_dockerCredential(
            registries=registries, name=name)
    else:
        registry = p_client.create_namespacedDockerCredential(
            registries=registries, name=name,
            namespaceId=ns.id)

    create_validate_workload(p_client, ns)
    return registry


def create_workload(p_client, ns):
    workload_name = random_test_name("newtestwk")
    con = [{"name": "test",
            "image": TEST_CLIENT_IMAGE,
            "runAsNonRoot": False,
            "stdin": True,
            "imagePullPolicy": "Always"
            }]
    workload = p_client.create_workload(name=workload_name,
                                        containers=con,
                                        namespaceId=ns.id)
    return workload


def create_validate_workload(p_client, ns):

    workload = create_workload(p_client, ns)
    workload = p_client.reload(workload)

    validate_workload(p_client, workload, "deployment", ns.name)
    p_client.delete(workload)


def create_validate_workload_with_invalid_registry(p_client, ns):

    workload = create_workload(p_client, ns)
    # Validate workload fails to come up active
    validate_wl_fail_to_pullimage(p_client, workload)
    workload = p_client.reload(workload)
    print(workload)
    assert workload.state != "active", "Invalid workload came up active!"
    p_client.delete(workload)


def validate_wl_fail_to_pullimage(client, workload, timeout=DEFAULT_TIMEOUT):
    """
    This method checks if workload is failing to pull image
    @param client: Project client object
    @param workload: Workload to test on
    @param timeout: Max time of waiting for failure
    """
    time.sleep(2)
    start = time.time()
    pods = client.list_pod(workloadId=workload.id).data
    assert len(pods) != 0, "No pods in workload - {}".format(workload)
    message = pods[0].containers[0].transitioningMessage

    while 'ImagePullBackOff:' not in message:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for workload to get "
                "'ImagePullBackOff' error")
        time.sleep(1)
        pods = client.list_pod(workloadId=workload.id).data
        assert len(pods) != 0, "No pods in workload - {}".format(workload)
        message = pods[0].containers[0].transitioningMessage
    print("{} - fails to pull image".format(workload))


@ pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testregistry")
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
