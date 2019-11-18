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
