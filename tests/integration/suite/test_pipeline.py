import pytest
from rancher import ApiError
from .conftest import wait_until_available, user_project_client
from .common import random_str


def test_pipeline_run_access(admin_mc, admin_pc, user_mc, remove_resource):
    """Tests that a user with read-only access is not
    able to run a pipeline.
    """
    prtb = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user_mc.user.id,
        projectId=admin_pc.project.id,
        roleTemplateId="read-only")
    remove_resource(prtb)

    pipeline = admin_pc.client.create_pipeline(
        projectId=admin_pc.project.id,
        repositoryUrl="https://github.com/rancher/pipeline-example-go.git",
        name=random_str(),
    )
    remove_resource(pipeline)
    wait_until_available(admin_pc.client, pipeline)

    # ensure user can get pipeline
    proj_user_client = user_project_client(user_mc, admin_pc.project)
    wait_until_available(proj_user_client, pipeline)
    with pytest.raises(ApiError) as e:
        # Doing run action with pipeline obj from admin_client should fail
        user_mc.client.action(obj=pipeline, action_name="run", branch="master")
    assert e.value.error.status == 404
