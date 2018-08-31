import pytest
from .common import random_str
import time
from rancher import ApiError


@pytest.fixture
def ns_default_quota():
    return {"limit": {"pods": "4"}}


@pytest.fixture
def ns_quota():
    return {"limit": {"pods": "4"}}


@pytest.fixture
def ns_small_quota():
    return {"limit": {"pods": "1"}}


@pytest.fixture
def ns_large_quota():
    return {"limit": {"pods": "200"}}


@pytest.fixture
def default_project_quota():
    return {"limit": {"pods": "100",
                      "services": "100",
                      "replicationControllers": "100",
                      "secrets": "100",
                      "configMaps": "100",
                      "persistentVolumeClaims": "100",
                      "servicesNodePorts": "100",
                      "servicesLoadBalancers": "100",
                      "requestsCpu": "100",
                      "requestsMemory": "100",
                      "requestsStorage": "100",
                      "requestsEphemeralStorage": "100",
                      "limitsCpu": "100",
                      "limitsMemory": "100",
                      "limitsEphemeralStorage": "100"}}


def wait_for_applied_quota_set(admin_cc_client, ns, timeout=30):
    start = time.time()
    ns = admin_cc_client.reload(ns)
    a = ns.annotations.data_dict()["cattle.io/status"]
    while a is None:
        time.sleep(.5)
        ns = admin_cc_client.reload(ns)
        a = ns.annotations["cattle.io/status"]
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for'
                            ' resource quota to be validated')

    while "Validating resource quota" in a or "exceeds project limit" in a:
        time.sleep(.5)
        ns = admin_cc_client.reload(ns)
        a = ns.annotations.data_dict()["cattle.io/status"]
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for'
                            ' resource quota to be validated')


def test_namespace_resource_quota(admin_cc, admin_pc):
    q = default_project_quota()
    p = admin_cc.management.client.update(admin_pc.project,
                                          resourceQuota=q,
                                          namespaceDefaultResourceQuota=q)
    p = admin_cc.management.client.wait_success(p)
    assert p.resourceQuota is not None
    client = admin_pc.cluster.client
    ns = client.create_namespace(name=random_str(),
                                 projectId=p.id,
                                 resourceQuota=ns_quota())
    assert ns is not None
    assert ns.resourceQuota is not None
    wait_for_applied_quota_set(admin_pc.cluster.client,
                               ns)


def test_project_resource_quota_fields(admin_cc):
    client = admin_cc.management.client
    q = default_project_quota()
    p = client.create_project(name='test-' + random_str(),
                              clusterId=admin_cc.cluster.id,
                              resourceQuota=q,
                              namespaceDefaultResourceQuota=q)
    p = admin_cc.management.client.wait_success(p)

    assert p.resourceQuota is not None
    assert p.resourceQuota.limit.pods == '100'
    assert p.namespaceDefaultResourceQuota is not None
    assert p.namespaceDefaultResourceQuota.limit.pods == '100'


def test_resource_quota_ns_create(admin_cc, admin_pc):
    q = default_project_quota()
    p = admin_cc.management.client.update(admin_pc.project,
                                          resourceQuota=q,
                                          namespaceDefaultResourceQuota=q)
    p = admin_cc.management.client.wait_success(p)
    assert p.resourceQuota is not None
    assert p.resourceQuota.limit.pods == '100'

    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=p.id,
                                                  resourceQuota=ns_quota())
    assert ns is not None
    assert ns.resourceQuota is not None
    wait_for_applied_quota_set(admin_pc.cluster.client, ns)


def test_default_resource_quota_ns_set(admin_cc, admin_pc):
    q = ns_default_quota()
    pq = default_project_quota()
    p = admin_cc.management.client.update(admin_pc.project,
                                          resourceQuota=pq,
                                          namespaceDefaultResourceQuota=q)
    assert p.resourceQuota is not None
    assert p.namespaceDefaultResourceQuota is not None

    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=p.id)
    wait_for_applied_quota_set(admin_pc.cluster.client,
                               ns)


def test_quota_ns_create_exceed(admin_cc, admin_pc, ns_large_quota):
    q = ns_default_quota()
    q = default_project_quota()
    p = admin_cc.management.client.update(admin_pc.project,
                                          resourceQuota=q,
                                          namespaceDefaultResourceQuota=q)
    p = admin_cc.management.client.wait_success(p)
    assert p.resourceQuota is not None

    # namespace quota exceeding project resource quota
    with pytest.raises(ApiError) as e:
        admin_pc.cluster.client.create_namespace(name=random_str(),
                                                 projectId=p.id,
                                                 resourceQuota=ns_large_quota)

    assert e.value.error.status == 422


def test_default_resource_quota_ns_create_invalid_combined(admin_cc, admin_pc,
                                                           ns_quota,
                                                           ns_large_quota,
                                                           ns_small_quota):
    pq = default_project_quota()
    q = ns_default_quota()
    p = admin_cc.management.client.update(admin_pc.project,
                                          resourceQuota=pq,
                                          namespaceDefaultResourceQuota=q)
    p = admin_cc.management.client.wait_success(p)
    assert p.resourceQuota is not None
    assert p.namespaceDefaultResourceQuota is not None

    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=p.id,
                                                  resourceQuota=ns_quota)
    assert ns is not None
    assert ns.resourceQuota is not None

    # namespace quota exceeding project resource quota
    with pytest.raises(ApiError) as e:
        admin_pc.cluster.client.create_namespace(name=random_str(),
                                                 projectId=p.id,
                                                 resourceQuota=ns_large_quota)

    assert e.value.error.status == 422

    # quota within limits
    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=p.id,
                                                  resourceQuota=ns_small_quota)
    assert ns is not None
    assert ns.resourceQuota is not None
    wait_for_applied_quota_set(admin_pc.cluster.client, ns, 10)
    ns = admin_cc.client.reload(ns)

    # update namespace with exceeding quota
    with pytest.raises(ApiError) as e:
        admin_pc.cluster.client.update(ns,
                                       projectId=p.id,
                                       resourceQuota=ns_large_quota)

    assert e.value.error.status == 422


def test_project_used_quota(admin_cc, admin_pc):
    pq = default_project_quota()
    q = ns_default_quota()
    p = admin_cc.management.client.update(admin_pc.project,
                                          resourceQuota=pq,
                                          namespaceDefaultResourceQuota=q)
    p = admin_cc.management.client.wait_success(p)
    assert p.resourceQuota is not None
    assert p.namespaceDefaultResourceQuota is not None

    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=p.id)
    wait_for_applied_quota_set(admin_pc.cluster.client,
                               ns)
    used = wait_for_used_limit_set(admin_cc.management.client, p)
    assert used.pods == "4"


def wait_for_used_limit_set(admin_cc_client, project, timeout=30):
    start = time.time()
    project = admin_cc_client.reload(project)
    while "usedLimit" not in project.resourceQuota \
            or "pods" not in project.resourceQuota.usedLimit:
        time.sleep(.5)
        project = admin_cc_client.reload(project)
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for'
                            ' project.usedLimit to be set')
    return project.resourceQuota.usedLimit


def test_default_resource_quota_project_update(admin_cc, admin_pc):
    p = admin_pc.project
    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=p.id)
    wait_for_applied_quota_set(admin_pc.cluster.client, ns, 10)

    pq = default_project_quota()
    q = ns_default_quota()
    p = admin_cc.management.client.update(admin_pc.project,
                                          resourceQuota=pq,
                                          namespaceDefaultResourceQuota=q)
    assert p.resourceQuota is not None
    assert p.namespaceDefaultResourceQuota is not None
    wait_for_applied_quota_set(admin_pc.cluster.client,
                               ns)


def test_api_validation_project(admin_cc, ns_large_quota):
    client = admin_cc.management.client
    q = default_project_quota()
    with pytest.raises(ApiError) as e:
        client.create_project(name='test-' + random_str(),
                              clusterId=admin_cc.cluster.id,
                              resourceQuota=q)

    assert e.value.error.status == 422

    with pytest.raises(ApiError) as e:
        client.create_project(name='test-' + random_str(),
                              clusterId=admin_cc.cluster.id,
                              namespaceDefaultResourceQuota=q)

    assert e.value.error.status == 422

    lq = ns_large_quota
    with pytest.raises(ApiError) as e:
        client.create_project(name='test-' + random_str(),
                              clusterId=admin_cc.cluster.id,
                              resourceQuota=q,
                              namespaceDefaultResourceQuota=lq)

    assert e.value.error.status == 422
