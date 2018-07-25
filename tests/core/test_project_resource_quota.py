import pytest
from .common import random_str
import time


def test_template(admin_cc):
    limit = {"pods": "4",
             "services": "4",
             "replicationControllers": "4",
             "secrets": "4",
             "configMaps": "4",
             "persistentVolumeClaims": "4",
             "servicesNodePorts": "4",
             "servicesLoadBalancers": "4",
             "requestsCpu": "4",
             "requestsMemory": "4",
             "requestsStorage": "4",
             "requestsEphemeralStorage": "4",
             "limitsCpu": "4",
             "limitsMemory": "4",
             "limitsEphemeralStorage": "4"}
    ns_id = admin_cc.cluster.id
    client = admin_cc.management.client
    t = client.create_resourceQuotaTemplate(name=random_str(),
                                            limit=limit,
                                            namespaceId=ns_id,
                                            clusterId=ns_id)
    assert t is not None
    assert t.limit.pods == "4"
    assert t.limit.services == "4"
    assert t.limit.replicationControllers == "4"
    assert t.limit.secrets == "4"
    assert t.limit.configMaps == "4"
    assert t.limit.persistentVolumeClaims == "4"
    assert t.limit.servicesNodePorts == "4"
    assert t.limit.servicesLoadBalancers == "4"
    assert t.limit.requestsCpu == "4"
    assert t.limit.requestsMemory == "4"
    assert t.limit.requestsStorage == "4"
    assert t.limit.limitsCpu == "4"
    assert t.limit.limitsMemory == "4"


@pytest.fixture
def default_template(admin_cc):
    ns_id = admin_cc.cluster.id
    client = admin_cc.management.client
    tmpl = client.list_resource_quota_template(clusterId=ns_id)
    for t in tmpl:
        if t.isDefault:
            return t

    limit = {"pods": "4"}
    ns_id = admin_cc.cluster.id
    client = admin_cc.management.client
    t = client.create_resourceQuotaTemplate(name=random_str(),
                                            limit=limit,
                                            isDefault=True,
                                            namespaceId=ns_id,
                                            clusterId=ns_id)
    return t


@pytest.fixture
def template(admin_cc):
    limit = {"pods": "4"}
    ns_id = admin_cc.cluster.id
    client = admin_cc.management.client
    t = client.create_resourceQuotaTemplate(name=random_str(),
                                            limit=limit,
                                            namespaceId=ns_id,
                                            clusterId=ns_id)
    return t


@pytest.fixture
def small_template(admin_cc):
    limit = {"pods": "1", }
    ns_id = admin_cc.cluster.id
    client = admin_cc.management.client
    t = client.create_resourceQuotaTemplate(name=random_str(),
                                            limit=limit,
                                            namespaceId=ns_id,
                                            clusterId=ns_id)
    return t


@pytest.fixture
def large_template(admin_cc):
    limit = {"pods": "200", }
    ns_id = admin_cc.cluster.id
    client = admin_cc.management.client
    t = client.create_resourceQuotaTemplate(name=random_str(),
                                            limit=limit,
                                            namespaceId=ns_id,
                                            clusterId=ns_id)
    return t


@pytest.fixture
def default_quota():
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


def wait_for_applied_template_set(admin_cc_client, ns, timeout=30):
    start = time.time()
    ns = admin_cc_client.reload(ns)
    while ns.resourceQuotaAppliedTemplateId is None:
        time.sleep(.5)
        ns = admin_cc_client.reload(ns)
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for'
                            ' resourceQuotaAppliedTemplateId to be set')
    return ns.resourceQuotaAppliedTemplateId


def test_namespace_resource_quota(admin_cc, admin_pc, template):
    p = admin_cc.management.client.update(admin_pc.project,
                                          resourceQuota=default_quota())
    p = admin_cc.management.client.wait_success(p)
    assert p.resourceQuota is not None
    t_id = template.id
    client = admin_pc.cluster.client
    ns = client.create_namespace(name=random_str(),
                                 projectId=p.id,
                                 resourceQuotaTemplateId=t_id)
    assert ns is not None
    assert ns.resourceQuotaTemplateId is not None
    assert ns.resourceQuotaTemplateId == template.id
    applied = wait_for_applied_template_set(admin_pc.cluster.client,
                                            ns)
    assert applied == template.id


def test_project_resource_quota(admin_cc):
    client = admin_cc.management.client
    p = client.create_project(name='test-' + random_str(),
                              clusterId=admin_cc.cluster.id,
                              resourceQuota=default_quota())
    p = admin_cc.management.client.wait_success(p)

    assert p.resourceQuota is not None
    assert p.resourceQuota.limit.pods == '100'


def test_resource_quota_ns_create(admin_cc, admin_pc, template):
    p = admin_cc.management.client.update(admin_pc.project,
                                          resourceQuota=default_quota())
    p = admin_cc.management.client.wait_success(p)
    assert p.resourceQuota is not None
    assert p.resourceQuota.limit.pods == '100'

    t_id = template.id
    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=p.id,
                                                  resourceQuotaTemplateId=t_id)
    assert ns is not None
    assert ns.resourceQuotaTemplateId == t_id
    applied = wait_for_applied_template_set(admin_pc.cluster.client,
                                            ns)
    assert applied == t_id


def test_default_resource_quota_ns_set(admin_cc, admin_pc, default_template):
    p = admin_cc.management.client.update(admin_pc.project,
                                          resourceQuota=default_quota())
    assert p.resourceQuota is not None

    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=p.id)
    wait_for_applied_template_set(admin_pc.cluster.client,
                                  ns)


def test_quota_ns_create_exceed(admin_cc, admin_pc, large_template):
    p = admin_cc.management.client.update(admin_pc.project,
                                          resourceQuota=default_quota())
    p = admin_cc.management.client.wait_success(p)
    assert p.resourceQuota is not None

    # namespace quota exceeding project resource quota
    l_id = large_template.id
    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=p.id,
                                                  resourceQuotaTemplateId=l_id)
    with pytest.raises(Exception):
        wait_for_applied_template_set(admin_pc.cluster.client, ns, 5)


def test_default_resource_quota_ns_create_invalid_combined(admin_cc, admin_pc,
                                                           template,
                                                           large_template,
                                                           small_template,
                                                           default_template):
    p = admin_cc.management.client.update(admin_pc.project,
                                          resourceQuota=default_quota())
    p = admin_cc.management.client.wait_success(p)
    assert p.resourceQuota is not None

    t_id = template.id
    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=p.id,
                                                  resourceQuotaTemplateId=t_id)
    assert ns is not None
    assert ns.resourceQuotaTemplateId == t_id

    # namespace quota exceeding project resource quota
    l_id = large_template.id
    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=p.id,
                                                  resourceQuotaTemplateId=l_id)
    assert ns is not None
    assert ns.resourceQuotaTemplateId == l_id
    with pytest.raises(Exception):
        wait_for_applied_template_set(admin_pc.cluster.client, ns, 10)

    # quota within limits
    t_id = small_template.id
    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=p.id,
                                                  resourceQuotaTemplateId=t_id)
    assert ns is not None
    assert ns.resourceQuotaTemplateId == t_id
    wait_for_applied_template_set(admin_pc.cluster.client, ns, 10)

    # update namespace with exceeding quota
    ns = admin_pc.cluster.client.update(ns,
                                        projectId=p.id,
                                        resourceQuotaTemplateId=l_id)
    assert ns is not None
    assert ns.resourceQuotaTemplateId == l_id
    applied = wait_for_applied_template_set(admin_pc.cluster.client, ns, 5)
    assert applied != l_id
    assert applied == t_id


def test_project_used_quota(admin_cc, admin_pc, default_template):
    p = admin_cc.management.client.update(admin_pc.project,
                                          resourceQuota=default_quota())
    p = admin_cc.management.client.wait_success(p)
    assert p.resourceQuota is not None

    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=p.id)
    applied = wait_for_applied_template_set(admin_pc.cluster.client,
                                            ns)
    assert applied == default_template.id

    used = wait_for_used_limit_set(admin_cc.management.client, p)
    assert used.pods == "4"


def wait_for_used_limit_set(admin_cc_client, project, timeout=30):
    start = time.time()
    project = admin_cc_client.reload(project)
    while project.resourceQuota.usedLimit is None:
        time.sleep(.5)
        project = admin_cc_client.reload(project)
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for'
                            ' project.usedLimit to be set')
    return project.resourceQuota.usedLimit


def wait_for_template_reset(admin_cc_client, ns, timeout=30):
    start = time.time()
    ns = admin_cc_client.reload(ns)
    while ns.resourceQuotaTemplateId is not None:
        time.sleep(.5)
        ns = admin_cc_client.reload(ns)
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for'
                            ' resourceQuotaTemplateId to be reset')
    return ns
