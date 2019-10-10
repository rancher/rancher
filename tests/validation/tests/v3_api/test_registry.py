import os

import pytest

from .common import *  # NOQA

CLUSTER_NAME = os.environ.get("CLUSTER_NAME", "")
namespace = {"p_client": None, "ns": None, "cluster": None, "project": None}
REGISTRY_USER_NAME = os.environ.get('RANCHER_REGISTRY_USER_NAME', "None")
REGISTRY_PASSWORD = os.environ.get('RANCHER_REGISTRY_PASSWORD', "None")
TEST_CLIENT_IMAGE = os.environ.get('RANCHER_TEST_CLIENT_IMAGE', "None")
REGISTRY = os.environ.get('RANCHER_REGISTRY', "None")


def test_create_registry_single_namespace():
    p_client = namespace["p_client"]
    ns = namespace["ns"]

    create_registry_validate_workload(p_client, ns)


def test_create_registry_all_namespace():
    p_client = namespace["p_client"]
    c_client = namespace["c_client"]
    cluster = namespace["cluster"]
    project = namespace["project"]

    ns = namespace["ns"]
    create_registry_validate_workload(p_client, ns, allns=True)

    # Create and validate workload in a new namespace
    new_ns = create_ns(c_client, cluster, project)
    create_validate_workload(p_client, new_ns)


def test_delete_registry_all_namespace():
    p_client = namespace["p_client"]
    c_client = namespace["c_client"]
    cluster = namespace["cluster"]
    project = namespace["project"]
    ns = namespace["ns"]
    new_ns = create_ns(c_client, cluster, project)

    registry = create_registry_validate_workload(p_client, ns, allns=True)
    delete_registry(p_client, registry, ns)

    print("Verify workloads cannot be created in all the namespaces")
    create_validate_workload_with_invalid_registry(p_client, ns)
    create_validate_workload_with_invalid_registry(p_client, new_ns)


def test_delete_registry_single_namespace():
    p_client = namespace["p_client"]
    ns = namespace["ns"]

    registry = create_registry_validate_workload(p_client, ns)
    delete_registry(p_client, registry, ns)

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


def delete_registry(client, registry, ns):

    c_client = namespace["c_client"]
    project = namespace["project"]
    print("Project ID")
    print(project.id)
    client.delete(registry)
    registryname = registry.name
    # Sleep to allow for the registry to be deleted
    time.sleep(5)
    print("Registry list after deleting registry")
    registrydict = client.list_dockerCredential(name=registryname).data
    print(registrydict)
    if len(registrydict) == 0:
        assert True

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
        if result != 0:
            assert True


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
    wait_for_wl_to_active(p_client, workload, timeout=90)
    return workload


def create_validate_workload(p_client, ns):

    workload = create_workload(p_client, ns)
    workload = p_client.reload(workload)

    validate_workload(p_client, workload, "deployment", ns.name)


def create_validate_workload_with_invalid_registry(p_client, ns):

    workload = create_workload(p_client, ns)
    workload = p_client.reload(workload)
    print("Workload State " + workload.state)
    # Verify that the workload fails to reach active state after the registry
    # update has been made with invalid password
    if workload.state != "active":
        assert True


@pytest.fixture(scope='module', autouse="True")
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
