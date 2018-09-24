import kubernetes
from rancher import ApiError
from .common import random_str
from .conftest import wait_until_available,\
    cluster_and_client, kubernetes_api_client, wait_for

from pprint import pprint

def test_multi_user(admin_mc, user_mc):
    """Tests a bug in the python client where multiple clients would not
    work properly. All clients would get the auth header of the last  client"""
    # Original admin client should be able to get auth configs
    ac = admin_mc.client.list_auth_config()
    assert len(ac) > 0

    # User client should not. We currently dont 404 on this, which would be
    # more correct. Instead, list gets filtered to zero
    ac = user_mc.client.list_auth_config()
    assert len(ac) == 0


def test_global_privilege_escalation(admin_mc, user_mc):
    admin_client = admin_mc.client
    user_client = user_mc.client

    admin_client.create_global_role_binding(
        userId=user_mc.user.id,
        globalRoleId="users-manage",
    )

    def roles_bound():
        roles = user_client.list_global_role_binding()
        if not roles.data:
            return False

        roleIds = []
        for role in roles.data:
            if role.userId == user_mc.user.id:
                roleIds.append(role.globalRoleId)

        if set(roleIds) != set(["user", "users-manage"]):
            pprint(roleIds)
            assert False, "Roles ids should contain 'user' and 'users-manage'"

        return True

    wait_for(roles_bound)

    try:
        user_client.create_global_role_binding(
            userId=user_mc.user.id,
            globalRoleId="admin",
        )
        assert False, "create_global_role_binding should cause an exception"
    except ApiError as e:
        pprint(vars(e))
        assert e.error.code == 'InvalidState', "self escalation to project owner should be invalid state"
        assert e.error.status == 422, "self escalation to cluster owner should return status code 422"

    return


def test_cluster_privilege_escalation(admin_cc, admin_mc, user_mc):
    admin_client = admin_mc.client
    user_client = user_mc.client

    admin_client.create_cluster_role_template_binding(
        userId=user_mc.user.id,
        roleTemplateId="clusterroletemplatebindings-manage",
        clusterId=admin_cc.cluster.id,
    )

    wait_until_available(user_client, admin_cc.cluster)
    try:
        user_client.create_cluster_role_template_binding(
            userId=user_mc.user.id,
            roleTemplateId="cluster-owner",
            clusterId=admin_cc.cluster.id,
        )
        assert False, "create_cluster_role_template_binding should cause an exception"
    except ApiError as e:
        pprint(vars(e))
        assert e.error.code == 'InvalidState', "self escalation to project owner should be invalid state"
        assert e.error.status == 422, "self escalation to cluster owner should return status code 422"

    return


def test_project_privilege_escalation(admin_cc, admin_pc, admin_mc, user_mc, request):
    admin_client = admin_mc.client
    user_client = user_mc.client

    p = admin_client.create_project(name='test-' + random_str(),
                                    clusterId=admin_cc.cluster.id)

    request.addfinalizer(lambda: admin_client.delete(p))

    p = wait_until_available(admin_client, p)
    p = admin_client.wait_success(p)
    assert p.state == 'active'

    admin_client.create_project_role_template_binding(
        userId=user_mc.user.id,
        roleTemplateId="projectroletemplatebindings-manage",
        projectId=admin_pc.project.id,
    )
    wait_until_available(admin_client, admin_pc.project)

    try:
        user_client.create_project_role_template_binding(
            userId=user_mc.user.id,
            roleTemplateId="project-owner",
            projectId=admin_pc.project.id,
        )
        assert False, "create_project_role_template_binding should cause an exception"
    except ApiError as e:
        pprint(vars(e))
        assert e.error.code == 'InvalidState', "self escalation to project owner should be invalid state"
        assert e.error.status == 422, "self escalation to project owner should return status code 422"

    return
