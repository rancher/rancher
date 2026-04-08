import kubernetes
import pytest
from rancher import ApiError
import time

from .common import random_str
from .conftest import wait_until_available, wait_until, \
    user_project_client, \
    wait_for, user_cluster_client


def test_appropriate_users_can_see_kontainer_drivers(user_factory):
    kds = user_factory().client.list_kontainer_driver()
    assert len(kds) == 3

    kds = user_factory('clusters-create').client.list_kontainer_driver()
    assert len(kds) == 3

    kds = user_factory('kontainerdrivers-manage').client. \
        list_kontainer_driver()
    assert len(kds) == 3

    kds = user_factory('settings-manage').client.list_kontainer_driver()
    assert len(kds) == 0


def test_readonly_cannot_edit_secret(admin_mc, user_mc, admin_pc,
                                     remove_resource):
    """Tests that a user with readonly access is not able to create/update
     a secret or ns secret
    """
    project = admin_pc.project
    user_client = user_mc.client

    prtb = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user_mc.user.id,
        projectId=project.id,
        roleTemplateId="read-only"
    )
    remove_resource(prtb)

    wait_until_available(user_client, project)

    proj_user_client = user_project_client(user_mc, project)

    # readonly should failed to create a regular secret
    with pytest.raises(ApiError) as e:
        proj_user_client.create_secret(
            name="test-" + random_str(),
            stringData={
                'abc': '123'
            }
        )
    assert e.value.error.status == 403

    secret = admin_pc.client.create_secret(
        name="test-" + random_str(),
        stringData={
            'abc': '123'
        }
    )
    remove_resource(secret)

    wait_until_available(admin_pc.client, secret)

    # readonly should failed to update a regular secret
    with pytest.raises(ApiError) as e:
        proj_user_client.update_by_id_secret(
            id=secret.id,
            stringData={
                'asd': 'fgh'
            }
        )
    assert e.value.error.status == 404

    ns = admin_pc.cluster.client.create_namespace(
        name='test-' + random_str(),
        projectId=project.id
    )
    remove_resource(ns)

    # readonly should fail to create ns secret
    with pytest.raises(ApiError) as e:
        proj_user_client.create_namespaced_secret(
            namespaceId=ns.id,
            name="test-" + random_str(),
            stringData={
                'abc': '123'
            }
        )
    assert e.value.error.status == 403

    ns_secret = admin_pc.client.create_namespaced_secret(
        namespaceId=ns.id,
        name="test-" + random_str(),
        stringData={
            'abc': '123'
        }
    )
    remove_resource(ns_secret)

    wait_until_available(admin_pc.client, ns_secret)

    # readonly should fail to update ns secret
    with pytest.raises(ApiError) as e:
        proj_user_client.update_by_id_namespaced_secret(
            namespaceId=ns.id,
            id=ns_secret.id,
            stringData={
                'asd': 'fgh'
            }
        )
    assert e.value.error.status == 404


@pytest.mark.skip
def test_member_can_edit_secret(admin_mc, admin_pc, remove_resource,
                                user_mc):
    """Tests that a user with project-member role is able to create/update
    secrets and namespaced secrets
    """
    project = admin_pc.project
    user_client = user_mc.client

    ns = admin_pc.cluster.client.create_namespace(
        name='test-' + random_str(),
        projectId=project.id
    )
    remove_resource(ns)

    prtb = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user_mc.user.id,
        projectId=project.id,
        roleTemplateId="project-member"
    )

    remove_resource(prtb)

    wait_until_available(user_client, project)

    proj_user_client = user_project_client(user_mc, project)

    def try_create_secret():
        try:
            return proj_user_client.create_secret(
                name="secret-" + random_str(),
                stringData={
                    'abc': '123'
                }
            )
        except ApiError as e:
            assert e.error.status == 403
        return False

    # Permission to create secret may not have been granted yet,
    # so it will be retried for 45 seconds
    secret = wait_for(try_create_secret, fail_handler=lambda:
                      "do not have permission to create secret")
    remove_resource(secret)

    wait_until_available(proj_user_client, secret)

    proj_user_client.update_by_id_secret(id=secret.id, stringData={
        'asd': 'fgh'
    })

    def try_create_ns_secret():
        try:
            return proj_user_client.create_namespaced_secret(
                name="secret-" + random_str(),
                namespaceId=ns.id,
                stringData={
                    "abc": "123"
                }
            )

        except ApiError as e:
            assert e.error.status == 403
        return False

    ns_secret = wait_for(try_create_ns_secret, fail_handler=lambda:
                         "do not have permission to create ns secret")
    remove_resource(ns_secret)

    wait_until_available(proj_user_client, ns_secret)

    proj_user_client.update_by_id_namespaced_secret(
        namespaceId=ns.id,
        id=ns_secret.id,
        stringData={
            "asd": "fgh"
        }
    )


def test_readonly_cannot_move_namespace(
        admin_cc, admin_mc, user_mc, remove_resource):
    """Tests that a user with readonly access is not able to
    move namespace across projects. Makes 2 projects and one
    namespace and then moves NS across.
    """
    p1 = admin_mc.client.create_project(
        name='test-' + random_str(),
        clusterId=admin_cc.cluster.id
    )
    remove_resource(p1)
    p1 = admin_cc.management.client.wait_success(p1)

    p2 = admin_mc.client.create_project(
        name='test-' + random_str(),
        clusterId=admin_cc.cluster.id
    )
    remove_resource(p2)
    p2 = admin_mc.client.wait_success(p2)

    # Use k8s client to see if project namespace exists
    k8s_client = kubernetes.client.CoreV1Api(admin_mc.k8s_client)
    wait_until(cluster_has_namespace(k8s_client, p1.id.split(":")[1]))
    wait_until(cluster_has_namespace(k8s_client, p2.id.split(":")[1]))

    prtb = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user_mc.user.id,
        projectId=p1.id,
        roleTemplateId="read-only")
    remove_resource(prtb)

    prtb2 = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user_mc.user.id,
        projectId=p2.id,
        roleTemplateId="read-only")
    remove_resource(prtb2)

    wait_until_available(user_mc.client, p1)
    wait_until_available(user_mc.client, p2)

    ns = admin_cc.client.create_namespace(
        name=random_str(),
        projectId=p1.id
    )
    wait_until_available(admin_cc.client, ns)
    remove_resource(ns)

    cluster_user_client = user_cluster_client(user_mc, admin_cc.cluster)
    wait_until_available(cluster_user_client, ns)

    with pytest.raises(ApiError) as e:
        user_mc.client.action(obj=ns, action_name="move", projectId=p2.id)
    assert e.value.error.status == 404


def wait_for_workload(client, ns, timeout=60, count=0):
    start = time.time()
    interval = 0.5
    workloads = client.list_workload(namespaceId=ns)
    while len(workloads.data) != count:
        if time.time() - start > timeout:
            print(workloads)
            raise Exception('Timeout waiting for workload service')
        time.sleep(interval)
        interval *= 2
        workloads = client.list_workload(namespaceId=ns)
    return workloads


def cluster_has_namespace(client, ns_name):
    """Wait for the give namespace to exist, useful for project namespaces"""
    def cb():
        return ns_name in \
               [ns.metadata.name for ns in client.list_namespace().items]
    return cb
