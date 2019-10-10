import os
import pytest
from .common import *  # NOQA

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None}

CLUSTER_NAME = os.environ.get("CLUSTER_NAME", "")
RANCHER_CLEANUP_PROJECT = os.environ.get("RANCHER_CLEANUP_PROJECT", "True")


@pytest.fixture
def ns_default_quota():
    return {"limit": {"pods": "5",
                      "requestsCpu": "500m"}}


@pytest.fixture
def default_project_quota():
    return {"limit": {"pods": "20",
                      "requestsCpu": "2000m"}}


def ns_quota():
    return {"limit": {"pods": "10",
                      "requestsCpu": "500m"}}


def test_create_project_quota():
    # Create Project Resource Quota and verify quota is created
    # successfully. Verify namespacedefault resource quota is set

    cluster = namespace["cluster"]
    client = get_user_client()
    c_client = namespace["c_client"]

    quota = default_project_quota()
    nsquota = ns_default_quota()
    proj = client.create_project(name='test-' + random_str(),
                                 clusterId=cluster.id,
                                 resourceQuota=quota,
                                 namespaceDefaultResourceQuota=nsquota)
    proj = client.wait_success(proj)

    assert proj.resourceQuota is not None
    assert proj.resourceQuota.limit.pods == quota["limit"]["pods"]
    assert proj.resourceQuota.limit.requestsCpu == \
        quota["limit"]["requestsCpu"]
    assert proj.namespaceDefaultResourceQuota is not None
    assert proj.namespaceDefaultResourceQuota.limit.pods == \
        nsquota["limit"]["pods"]

    ns = create_ns(c_client, cluster, proj)
    print(ns)

    assert ns is not None
    assert ns.resourceQuota is not None
    assert ns.resourceQuota.limit.pods == nsquota["limit"]["pods"]
    assert ns.resourceQuota.limit.requestsCpu == \
        nsquota["limit"]["requestsCpu"]

    validate_resoucequota_thru_kubectl(ns)


def test_resource_quota_create_namespace_with_ns_quota():

    # Create project quota and namspaces and verify
    # namespace creation is allowed within the quota

    cluster = namespace["cluster"]
    client = get_user_client()
    c_client = namespace["c_client"]

    quota = default_project_quota()
    nsquota = ns_quota()
    proj = client.create_project(name='test-' + random_str(),
                                 clusterId=cluster.id,
                                 resourceQuota=quota,
                                 namespaceDefaultResourceQuota=quota)
    proj = client.wait_success(proj)

    assert proj.resourceQuota is not None

    # Create a namespace with namespace resource quota
    ns_name = random_str()
    ns = c_client.create_namespace(name=ns_name,
                                   projectId=proj.id,
                                   resourceQuota=ns_quota())
    ns = c_client.wait_success(ns)

    assert ns is not None
    assert ns.resourceQuota is not None
    assert ns.resourceQuota.limit.pods == nsquota["limit"]["pods"]
    assert ns.resourceQuota.limit.requestsCpu == \
        nsquota["limit"]["requestsCpu"]

    validate_resoucequota_thru_kubectl(ns)

    # Create another namespace with quota and it should succeed

    ns1 = c_client.create_namespace(name=random_str(),
                                    projectId=proj.id,
                                    resourceQuota=nsquota)
    ns1 = c_client.wait_success(ns1)
    print(ns1)

    assert ns1 is not None
    assert ns1.resourceQuota is not None
    assert ns1.resourceQuota.limit.pods == nsquota["limit"]["pods"]
    assert ns1.resourceQuota.limit.requestsCpu == \
        nsquota["limit"]["requestsCpu"]

    validate_resoucequota_thru_kubectl(ns1)

    # Creating another namespace should fail as it exceeds the allotted limit
    try:
        c_client.create_namespace(name=random_str(),
                                  projectId=proj.id,
                                  resourceQuota=ns_quota())
    except Exception as e:
        errorstring = str(e)
        print(str(e))
    assert "MaxLimitExceeded" in errorstring


def validate_resoucequota_thru_kubectl(namespace):

    # This method executes `kubectl describe quota command` and verifies if the
    # resource quota from kubectl and the quota assigned for the namespace
    # through API are the same

    command = "get quota --namespace " + namespace['id']
    print(command)

    result = execute_kubectl_cmd(command, json_out=True)
    print("Kubectl command result")
    print(result)
    testdict = namespace['resourceQuota']

    response = result["items"]
    assert "spec" in response[0]
    quotadict = (response[0]["spec"])
    assert quotadict['hard']['pods'] == testdict['limit']['pods']
    assert \
        quotadict['hard']['requests.cpu'] == testdict['limit']['requestsCpu']


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testworkload")
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
