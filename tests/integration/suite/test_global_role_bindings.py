import pytest

from rancher import ApiError

from kubernetes.client.rest import ApiException
from kubernetes.client import RbacAuthorizationV1Api
from .conftest import wait_for
from .common import random_str, string_to_encoding


def test_cannot_update_global_role(admin_mc, remove_resource):
    """Asserts that globalRoleId field cannot be changed"""
    admin_client = admin_mc.client

    grb = admin_client.create_global_role_binding(
        name="gr-" + random_str(),
        userId=admin_mc.user.id,
        globalRoleId="nodedrivers-manage")
    remove_resource(grb)

    grb = admin_client.update_by_id_global_role_binding(
        id=grb.id,
        globalRoleId="settings-manage")
    assert grb.globalRoleId == "nodedrivers-manage"


def test_globalrole_must_exist(admin_mc, remove_resource):
    """Asserts that globalRoleId must reference an existing role"""
    admin_client = admin_mc.client

    with pytest.raises(ApiError) as e:
        grb = admin_client.create_global_role_binding(
            name="gr-" + random_str(),
            globalRoleId="somefakerole",
            userId=admin_mc.user.id
        )
        remove_resource(grb)
    assert e.value.error.status == 404
    assert "globalRole.management.cattle.io \"somefakerole\" not found" in \
           e.value.error.message


def test_cannot_update_subject(admin_mc, user_mc, remove_resource):
    """Asserts that userId and groupPrincipalId fields cannot be
    changed"""
    admin_client = admin_mc.client

    grb = admin_client.create_global_role_binding(
        name="gr-" + random_str(),
        userId=admin_mc.user.id,
        globalRoleId="nodedrivers-manage")
    remove_resource(grb)

    grb = admin_client.update_by_id_global_role_binding(
        id=grb.id,
        userId=user_mc.user.id)
    assert grb.userId == admin_mc.user.id

    grb = admin_client.update_by_id_global_role_binding(
        id=grb.id,
        groupPrincipalId="groupa")
    assert grb.userId == admin_mc.user.id
    assert grb.groupPrincipalId is None


def test_grb_crb_lifecycle(admin_mc, remove_resource):
    """Asserts that global role binding creation and deletion
    properly creates and deletes underlying cluster role binding"""
    admin_client = admin_mc.client

    # admin role is used because it requires an
    # additional cluster role bindig to be managed
    grb = admin_client.create_global_role_binding(
        groupPrincipalId="asd", globalRoleId="admin"
    )
    remove_resource

    cattle_grb = "cattle-globalrolebinding-" + grb.id
    admin_grb = "globaladmin-u-" + string_to_encoding("asd").lower()

    api_instance = RbacAuthorizationV1Api(
        admin_mc.k8s_client)

    def get_crb_by_id(id):
        def get_crb_from_k8s():
            try:
                return api_instance.read_cluster_role_binding(id)
            except ApiException as e:
                assert e.status == 404
        return get_crb_from_k8s

    k8s_grb = wait_for(get_crb_by_id(cattle_grb))
    assert k8s_grb.subjects[0].kind == "Group"
    assert k8s_grb.subjects[0].name == "asd"

    k8s_grb = wait_for(get_crb_by_id(admin_grb))
    assert k8s_grb.subjects[0].kind == "Group"
    assert k8s_grb.subjects[0].name == "asd"

    grb = admin_client.reload(grb)
    admin_client.delete(grb)

    def crb_deleted_by_id(id):
        def is_crb_deleted():
            try:
                api_instance.read_cluster_role_binding(id)
            except ApiException as e:
                return e.status == 404
            return False
        return is_crb_deleted

    wait_for(crb_deleted_by_id(cattle_grb))
    wait_for(crb_deleted_by_id(admin_grb))


def test_grb_targets_user_or_group(admin_mc, remove_resource):
    """Asserts that a globalrolebinding must exclusively target
    a userId or groupPrincipalId"""
    admin_client = admin_mc.client

    with pytest.raises(ApiError) as e:
        grb = admin_client.create_global_role_binding(
            userId="asd",
            groupPrincipalId="asd",
            globalRoleId="admin"
        )
        remove_resource(grb)

    assert e.value.error.status == 422
    assert "must contain field [groupPrincipalId] OR field [userId]" in\
        e.value.error.message

    with pytest.raises(ApiError) as e:
        grb = admin_client.create_global_role_binding(
            globalRoleId="admin"
        )
        remove_resource(grb)

    assert e.value.error.status == 422
    assert "must contain field [groupPrincipalId] OR field [userId]" in \
           e.value.error.message
