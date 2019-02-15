from .common import random_str
from rancher import ApiError
from .conftest import wait_until
import time
import pytest


def test_multiclusterapp_create(admin_mc, admin_pc, remove_resource):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-2.1.10"

    targets = [{"projectId": admin_pc.project.id}]
    mcapp1 = client.create_multi_cluster_app(name=mcapp_name,
                                             templateVersionId=temp_ver,
                                             targets=targets)
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_name, 60)
    roles = mcapp1["roles"]
    # created this as admin, admin has cluster-owner and project-owner
    # roles in each cluster and project, so mcapp roles should get these
    # by default, since no roles were provided in the request
    expected_roles = ["project-owner", "cluster-owner"]
    for r in expected_roles:
        assert r in roles


def test_multiclusterapp_create_with_members(admin_mc, admin_pc,
                                             user_factory, remove_resource):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-2.1.10"

    targets = [{"projectId": admin_pc.project.id}]

    user_member = user_factory()
    remove_resource(user_member)
    user_not_member = user_factory()
    remove_resource(user_not_member)
    members = [{"userPrincipalId": "local://"+user_member.user.id,
                "accessType": "read-only"}]

    mcapp1 = client.create_multi_cluster_app(name=mcapp_name,
                                             templateVersionId=temp_ver,
                                             targets=targets,
                                             members=members)
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_name, 60)

    # check who has access to the multiclusterapp
    # admin and user_member should be able to list it
    id = "cattle-global-data:" + mcapp_name
    mcapp = client.by_id_multi_cluster_app(id)
    assert mcapp is not None
    um_client = user_member.client
    mcapp = um_client.by_id_multi_cluster_app(id)
    assert mcapp is not None

    unm_client = user_not_member.client
    try:
        unm_client.by_id_multi_cluster_app(id)
    except ApiError as e:
        assert e.error.status == 403


@pytest.mark.skip()
def test_multiclusterapp_create_with_roles(admin_mc, admin_pc,
                                           remove_resource, user_factory):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-2.1.10"
    targets = [{"projectId": admin_pc.project.id}]
    roles = ["cluster-owner", "project-owner"]
    mcapp1 = client.create_multi_cluster_app(name=mcapp_name,
                                             templateVersionId=temp_ver,
                                             targets=targets,
                                             roles=roles)
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_name, 60)

    # try creating as a user who is not cluster-owner,
    # but that is one of the roles listed, must fail
    user1 = user_factory()
    remove_resource(user1)
    # add user to project as owner but not to cluster
    prtb_owner = client.create_project_role_template_binding(
        projectId=admin_pc.project.id,
        roleTemplateId="project-owner",
        userId=user1.user.id)
    remove_resource(prtb_owner)

    wait_until(rtb_cb(client, prtb_owner))
    try:
        user1.client.create_multi_cluster_app(name=random_str(),
                                              templateVersionId=temp_ver,
                                              targets=targets,
                                              roles=roles)
    except ApiError as e:
        assert e.error.status == 500
        assert "does not have all cluster roles" in e.error.message


@pytest.mark.skip()
def test_multiclusterapp_update_roles(admin_mc, admin_pc, remove_resource):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-2.1.10"
    targets = [{"projectId": admin_pc.project.id}]
    # admin always gets cluster-owner and project-owner roles
    roles = ["cluster-owner", "project-owner"]
    mcapp1 = client.create_multi_cluster_app(name=mcapp_name,
                                             templateVersionId=temp_ver,
                                             targets=targets,
                                             roles=roles)
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_name, 60)

    # admin doesn't get cluster-member/project-member roles by default
    # updating the mcapp to add these roles must fail
    new_roles = ["cluster-owner", "project-owner", "project-member"]
    try:
        client.update(mcapp1, roles=new_roles)
    except ApiError as e:
        assert e.error.status == 500
        assert "does not have all project roles" in e.error.message

    new_roles = ["cluster-owner", "project-owner", "cluster-member"]
    try:
        client.update(mcapp1, roles=new_roles)
    except ApiError as e:
        assert e.error.status == 500
        assert "does not have all cluster roles" in e.error.message


def wait_for_app(admin_pc, name, timeout=60):
    start = time.time()
    interval = 0.5
    client = admin_pc.client
    cluster_id, project_id = admin_pc.project.id.split(':')
    app_name = name+"-"+project_id
    found = False
    while not found:
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for app of multiclusterapp')
        apps = client.list_app(name=app_name)
        if len(apps) > 0:
            found = True
        time.sleep(interval)
        interval *= 2


def rtb_cb(client, rtb):
    """Wait for the prtb to have the userId populated"""
    def cb():
        rt = client.reload(rtb)
        return rt.userPrincipalId is not None
    return cb
