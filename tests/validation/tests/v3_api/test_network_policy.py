import pytest

from .common import *  # NOQA

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None}
random_password = random_test_name("pass")

PROJECT_ISOLATION = os.environ.get('RANCHER_PROJECT_ISOLATION', "disabled")


def test_connectivity_between_pods():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    cluster = namespace["cluster"]

    con = [{"name": "test1",
            "image": TEST_IMAGE,
            }]
    name = random_test_name("default")
    schedulable_node_count = len(get_schedulable_nodes(cluster))

    # Check connectivity between pods in the same namespace

    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id,
                                        daemonSetConfig={})
    validate_workload(p_client, workload, "daemonSet", ns.name,
                      schedulable_node_count)
    check_connectivity_between_workload_pods(p_client, workload)

    # Create another namespace in the same project
    # Deploy workloads in this namespace
    # Check that pods belonging to different namespace within the
    # same project can communicate

    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)
    ns1 = create_ns(c_client, cluster, namespace["project"])
    workload1 = p_client.create_workload(name=name,
                                         containers=con,
                                         namespaceId=ns1.id,
                                         daemonSetConfig={})
    validate_workload(p_client, workload1, "daemonSet", ns1.name,
                      schedulable_node_count)
    check_connectivity_between_workload_pods(p_client, workload1)
    check_connectivity_between_workloads(p_client, workload, p_client,
                                         workload1)

    # Create new project in the same cluster
    # Create namespace and deploy workloads
    # Check communication between pods belonging to different namespace across
    # different projects

    p2, ns2 = create_project_and_ns(USER_TOKEN, cluster)
    p2_client = get_project_client_for_token(p2, USER_TOKEN)

    workload2 = p2_client.create_workload(name=name,
                                          containers=con,
                                          namespaceId=ns2.id,
                                          daemonSetConfig={})
    validate_workload(p2_client, workload2, "daemonSet", ns2.name,
                      schedulable_node_count)
    check_connectivity_between_workload_pods(p2_client, workload2)
    allow_connectivity = True
    if PROJECT_ISOLATION == "enabled":
        allow_connectivity = False
    check_connectivity_between_workloads(
        p_client, workload, p2_client, workload2,
        allow_connectivity=allow_connectivity)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testnp")
    p_client = get_project_client_for_token(p, USER_TOKEN)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p

    def fin():
        client = get_user_client()
        client.delete(namespace["project"])
    request.addfinalizer(fin)
