import pytest

from rancher import ApiError
from .common import random_str


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
