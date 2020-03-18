import pytest
from rancher import ApiError
from .common import random_str


def test_logging_test_action(admin_mc, admin_pc, user_mc, remove_resource):
    """Tests that a user with read-only access is not
    able to perform a logging test.
    """
    prtb = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user_mc.user.id,
        projectId=admin_pc.project.id,
        roleTemplateId="read-only")
    remove_resource(prtb)

    # use logEndpoint from admin client to get action not available to user
    logEndpoint = admin_mc.client.list_clusterLogging()
    with pytest.raises(ApiError) as e:
        user_mc.client.action(
            obj=logEndpoint,
            action_name="test",
            syslog={"config": {"endpoint": "0.0.0.0:8080"}}
        )
    assert e.value.error.status == 404
