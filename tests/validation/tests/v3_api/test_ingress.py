"""
This test suite contains tests to validate ingress create/edit/delete with
different possible way and with different roles of users.
Test requirement:
Below Env variables need to set
CATTLE_TEST_URL - url to rancher server
ADMIN_TOKEN - Admin token from rancher
USER_TOKEN - User token from rancher
RANCHER_CLUSTER_NAME - Cluster name to run test on
RANCHER_TEST_RBAC - Boolean (Optional), To run role based tests.
"""

from .common import CLUSTER_MEMBER
from .common import CLUSTER_OWNER
from .common import PROJECT_READ_ONLY
from .common import PROJECT_OWNER
from .common import PROJECT_MEMBER
from .common import TEST_IMAGE
from .common import random_test_name
from .common import validate_workload
from .common import get_schedulable_nodes
from .common import validate_ingress
from .common import wait_for_pods_in_workload
from .common import validate_ingress_using_endpoint
from .common import rbac_get_user_token_by_role
from .common import pytest
from .common import rbac_get_project
from .common import rbac_get_namespace
from .common import if_test_rbac
from .common import get_project_client_for_token
from .common import ApiError
from .common import time
from .common import get_user_client_and_cluster
from .common import create_kubeconfig
from .common import create_project_and_ns
from .common import USER_TOKEN
from .common import get_user_client
from .common import DEFAULT_TIMEOUT
from .common import rbac_get_workload
from .common import wait_for_ingress_to_active


namespace = {"p_client": None, "ns": None, "cluster": None, "project": None}
rbac_role_list = [
                  CLUSTER_OWNER,
                  CLUSTER_MEMBER,
                  PROJECT_OWNER,
                  PROJECT_MEMBER,
                  PROJECT_READ_ONLY
                 ]


def test_ingress():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    cluster = namespace["cluster"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    validate_workload(p_client, workload, "daemonSet", ns.name,
                      len(get_schedulable_nodes(cluster)))
    host = "test1.com"
    path = "/name.html"
    rule = {"host": host,
            "paths": [{"workloadIds": [workload.id], "targetPort": "80"}]}
    p_client.create_ingress(name=name,
                            namespaceId=ns.id,
                            rules=[rule])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], host, path)


def test_ingress_with_same_rules_having_multiple_targets():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    cluster = namespace["cluster"]
    con = [{"name": "testm1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload1 = p_client.create_workload(name=name,
                                         containers=con,
                                         namespaceId=ns.id,
                                         daemonSetConfig={})
    validate_workload(p_client, workload1, "daemonSet", ns.name,
                      len(get_schedulable_nodes(cluster)))
    name = random_test_name("default")
    workload2 = p_client.create_workload(name=name,
                                         containers=con,
                                         namespaceId=ns.id,
                                         daemonSetConfig={})
    validate_workload(p_client, workload2, "daemonSet", ns.name,
                      len(get_schedulable_nodes(cluster)))
    host = "testm1.com"
    path = "/name.html"
    rule1 = {"host": host,
             "paths": [{"workloadIds": [workload1.id], "targetPort": "80"}]}
    rule2 = {"host": host,
             "paths": [{"workloadIds": [workload2.id], "targetPort": "80"}]}
    p_client.create_ingress(name=name,
                            namespaceId=ns.id,
                            rules=[rule1, rule2])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload1, workload2], host, path)


def test_ingress_edit_target():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload1 = p_client.create_workload(name=name,
                                         containers=con,
                                         namespaceId=ns.id,
                                         scale=2)
    validate_workload(p_client, workload1, "deployment", ns.name, pod_count=2)
    name = random_test_name("default")
    workload2 = p_client.create_workload(name=name,
                                         containers=con,
                                         namespaceId=ns.id,
                                         scale=2)
    validate_workload(p_client, workload2, "deployment", ns.name, pod_count=2)

    host = "test2.com"
    path = "/name.html"
    rule = {"host": host,
            "paths": [{"workloadIds": [workload1.id], "targetPort": "80"}]}
    ingress = p_client.create_ingress(name=name,
                                      namespaceId=ns.id,
                                      rules=[rule])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload1], host, path)

    rule = {"host": host,
            "paths": [{"workloadIds": [workload2.id], "targetPort": "80"}]}
    ingress = p_client.update(ingress, rules=[rule])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload2], host, path)


def test_ingress_edit_host():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    cluster = namespace["cluster"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    validate_workload(p_client, workload, "daemonSet", ns.name,
                      len(get_schedulable_nodes(cluster)))
    host = "test3.com"
    path = "/name.html"
    rule = {"host": host,
            "paths": [{"workloadIds": [workload.id], "targetPort": "80"}]}
    ingress = p_client.create_ingress(name=name,
                                      namespaceId=ns.id,
                                      rules=[rule])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], host, path)
    host = "test4.com"
    rule = {"host": host,
            "paths": [{"workloadIds": [workload.id], "targetPort": "80"}]}
    ingress = p_client.update(ingress, rules=[rule])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], host, path)


def test_ingress_edit_path():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    cluster = namespace["cluster"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    validate_workload(p_client, workload, "daemonSet", ns.name,
                      len(get_schedulable_nodes(cluster)))
    host = "test5.com"
    path = "/name.html"
    rule = {"host": host,
            "paths": [{"workloadIds": [workload.id], "targetPort": "80"}]}
    ingress = p_client.create_ingress(name=name,
                                      namespaceId=ns.id,
                                      rules=[rule])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], host, path)
    path = "/service1.html"
    rule = {"host": host,
            "paths": [{"workloadIds": [workload.id], "targetPort": "80"}]}
    ingress = p_client.update(ingress, rules=[rule])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], host, path)


def test_ingress_edit_add_more_rules():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload1 = p_client.create_workload(name=name,
                                         containers=con,
                                         namespaceId=ns.id,
                                         scale=2)
    validate_workload(p_client, workload1, "deployment", ns.name, pod_count=2)
    name = random_test_name("default")
    workload2 = p_client.create_workload(name=name,
                                         containers=con,
                                         namespaceId=ns.id,
                                         scale=2)
    validate_workload(p_client, workload2, "deployment", ns.name, pod_count=2)

    host1 = "test6.com"
    path = "/name.html"
    rule1 = {"host": host1,
             "paths": [{"workloadIds": [workload1.id], "targetPort": "80"}]}
    ingress = p_client.create_ingress(name=name,
                                      namespaceId=ns.id,
                                      rules=[rule1])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload1], host1, path)

    host2 = "test7.com"
    rule2 = {"host": host2,
             "paths": [{"workloadIds": [workload2.id], "targetPort": "80"}]}
    ingress = p_client.update(ingress, rules=[rule1, rule2])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload2], host2, path)
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload1], host1, path)


def test_ingress_scale_up_target():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        scale=2)
    validate_workload(p_client, workload, "deployment", ns.name, pod_count=2)

    host = "test8.com"
    path = "/name.html"
    rule = {"host": host,
            "paths": [{"workloadIds": [workload.id], "targetPort": "80"}]}
    p_client.create_ingress(name=name,
                            namespaceId=ns.id,
                            rules=[rule])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], host, path)
    workload = p_client.update(workload, scale=4, containers=con)
    validate_workload(p_client, workload, "deployment", ns.name, pod_count=4)
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], host, path)


def test_ingress_upgrade_target():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = {"name": "test1",
           "image": TEST_IMAGE}
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=[con],
                                        namespaceId=ns.id,
                                        scale=2)
    validate_workload(p_client, workload, "deployment", ns.name, pod_count=2)

    host = "test9.com"
    path = "/name.html"
    rule = {"host": host,
            "paths": [{"workloadIds": [workload.id], "targetPort": "80"}]}
    p_client.create_ingress(name=name,
                            namespaceId=ns.id,
                            rules=[rule])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], host, path)
    con["environment"] = {"test1": "value1"}
    workload = p_client.update(workload, containers=[con])
    wait_for_pods_in_workload(p_client, workload, pod_count=2)
    validate_workload(p_client, workload, "deployment", ns.name, pod_count=2)
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], host, path)


def test_ingress_rule_with_only_path():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = {"name": "test1",
           "image": TEST_IMAGE}
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=[con],
                                        namespaceId=ns.id,
                                        scale=2)
    validate_workload(p_client, workload, "deployment", ns.name, pod_count=2)

    host = ""
    path = "/service2.html"
    rule = {"host": host,
            "paths": [{"workloadIds": [workload.id], "targetPort": "80"}]}
    p_client.create_ingress(name=name,
                            namespaceId=ns.id,
                            rules=[rule])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], "", path, True)


def test_ingress_rule_with_only_host():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    con = {"name": "test1",
           "image": TEST_IMAGE}
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=[con],
                                        namespaceId=ns.id,
                                        scale=2)
    validate_workload(p_client, workload, "deployment", ns.name, pod_count=2)

    host = "test10.com"
    rule = {"host": host,
            "paths": [{"workloadIds": [workload.id], "targetPort": "80"}]}
    p_client.create_ingress(name=name,
                            namespaceId=ns.id,
                            rules=[rule])
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], host, "/name.html")
    validate_ingress(namespace["p_client"], namespace["cluster"],
                     [workload], host, "/service1.html")


def test_ingress_xip_io():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    cluster = namespace["cluster"]
    con = [{"name": "test1",
            "image": TEST_IMAGE}]
    name = random_test_name("default")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    validate_workload(p_client, workload, "daemonSet", ns.name,
                      len(get_schedulable_nodes(cluster)))
    path = "/name.html"
    rule = {"host": "xip.io",
            "paths": [{"path": path,
                       "workloadIds": [workload.id], "targetPort": "80"}]}
    ingress = p_client.create_ingress(name=name,
                                      namespaceId=ns.id,
                                      rules=[rule])
    validate_ingress_using_endpoint(namespace["p_client"], ingress, [workload])


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_ingress_create(role):
    """
    This test creates first workload as cluster owner and then creates ingress
    as user i.e. role in parameter and validates the ingress created.
    @param role: User role in rancher eg. project owner, project member etc
    """
    token = rbac_get_user_token_by_role(role)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    workload = rbac_get_workload()
    p_client = get_project_client_for_token(project, token)
    name = random_test_name("default")

    host = "xip.io"
    rule = {"host": host,
            "paths": [{"workloadIds": [workload.id], "targetPort": "80"}]}
    if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
        with pytest.raises(ApiError) as e:
            p_client.create_ingress(name=name,
                                    namespaceId=ns.id,
                                    rules=[rule])
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
    else:
        ingress = p_client.create_ingress(name=name,
                                          namespaceId=ns.id,
                                          rules=[rule])
        wait_for_ingress_to_active(p_client, ingress)
        p_client.delete(ingress)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_ingress_edit(role):
    """
    This test creates two workloads and then creates ingress with two targets
    and validates it.
    @param role: User role in rancher eg. project owner, project member etc
    """
    c_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    token = rbac_get_user_token_by_role(role)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    workload = rbac_get_workload()
    p_client_for_c_owner = get_project_client_for_token(project, c_owner_token)
    p_client = get_project_client_for_token(project, token)

    host = "xip.io"
    path = "/name.html"
    rule_1 = {"host": host,
              "paths": [{"workloadIds": [workload.id], "targetPort": "80"}]}
    rule_2 = {"host": host,
              "paths": [{"path": path, "workloadIds": [workload.id],
                         "targetPort": "80"}]}
    name = random_test_name("default")
    ingress = p_client_for_c_owner.create_ingress(name=name, namespaceId=ns.id,
                                                  rules=[rule_1])
    wait_for_ingress_to_active(p_client_for_c_owner, ingress)
    if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
        with pytest.raises(ApiError) as e:
            ingress = p_client.update(ingress, rules=[rule_2])
            wait_for_ingress_to_active(p_client, ingress)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
    else:
        ingress = p_client.update(ingress, rules=[rule_2])
        wait_for_ingress_to_active(p_client, ingress)
    p_client_for_c_owner.delete(ingress)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_ingress_delete(role):
    """
    This test creates two workloads and then creates ingress with two targets
    and validates it.
    @param role: User role in rancher eg. project owner, project member etc
    """
    c_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    token = rbac_get_user_token_by_role(role)
    project = rbac_get_project()
    ns = rbac_get_namespace()
    workload = rbac_get_workload()
    p_client_for_c_owner = get_project_client_for_token(project, c_owner_token)
    p_client = get_project_client_for_token(project, token)
    name = random_test_name("default")

    host = "xip.io"
    rule = {"host": host,
            "paths": [{"workloadIds": [workload.id], "targetPort": "80"}]}

    ingress = p_client_for_c_owner.create_ingress(name=name, namespaceId=ns.id,
                                                  rules=[rule])
    wait_for_ingress_to_active(p_client_for_c_owner, ingress)
    if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
        with pytest.raises(ApiError) as e:
            p_client.delete(ingress)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
        p_client_for_c_owner.delete(ingress)
    else:
        p_client.delete(ingress)
        validate_ingress_deleted(p_client, ingress)


def validate_ingress_deleted(client, ingress, timeout=DEFAULT_TIMEOUT):
    """
    Checks whether ingress got deleted successfully.
    Validates if ingress is null in for current object client.
    @param client: Object client use to create ingress
    @param ingress: ingress object subjected to be deleted
    @param timeout: Max time to keep checking whether ingress is deleted or not
    """
    time.sleep(2)
    start = time.time()
    ingresses = client.list_ingress(uuid=ingress.uuid).data
    while len(ingresses) != 0:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for ingress to be deleted")
        time.sleep(.5)
        ingresses = client.list_ingress(uuid=ingress.uuid).data


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testingress")
    p_client = get_project_client_for_token(p, USER_TOKEN)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p

    def fin():
        client = get_user_client()
        client.delete(namespace["project"])
    request.addfinalizer(fin)
