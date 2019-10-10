import pytest

from .common import *  # NOQA

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None}


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
