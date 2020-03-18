import pytest
from rancher import ApiError
from .common import random_str
from .conftest import wait_until


def test_alert_access(admin_mc, admin_pc, admin_cc, user_mc, remove_resource):
    """Tests that a user with read-only access is not
    able to deactivate an alert.
    """
    prtb = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user_mc.user.id,
        projectId=admin_pc.project.id,
        roleTemplateId="read-only")
    remove_resource(prtb)

    # we get some project defaults, wait for them to come up
    wait_until(projectAlertRules(user_mc.client), timeout=20)
    # list with admin_mc to get action not available to user
    alerts = admin_mc.client.list_projectAlertRule(
        projectId=admin_pc.project.id
    )
    with pytest.raises(ApiError) as e:
        user_mc.client.action(obj=alerts.data[0], action_name="deactivate")
    assert e.value.error.status == 404


def projectAlertRules(client):
    """Wait for the crtb to have the userId populated"""
    def cb():
        return len(client.list_projectAlertRule().data) > 0
    return cb
