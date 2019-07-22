from .common import random_str
from .conftest import wait_until_available
from rancher import ApiError
import pytest


def test_context_prtb(admin_mc, admin_pc, remove_resource,
                      user_mc):
    """Tests that any user cannot edit a cluster role binding form the project
    context
    """
    project = admin_pc.project
    user_client = user_mc.client

    prtb = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user_mc.user.id,
        projectId=project.id,
        roleTemplateId="project-owner"
    )
    remove_resource(prtb)

    wait_until_available(user_client, project)

    prtb.roleTemplateId = "cluster-owner"
    with pytest.raises(ApiError) as e:
        prtb = user_client.update_by_id_projectRoleTemplateBinding(prtb.id,
                                                                   prtb)
    assert e.value.error.status == 422
    assert "Cannot edit" in e.value.error.message

    prtb = admin_mc.client.reload(prtb)
    prtb.roleTemplateId = "cluster-owner"
    with pytest.raises(ApiError) as e:
        prtb = admin_mc.client.update_by_id_projectRoleTemplateBinding(prtb.id,
                                                                       prtb)
    assert e.value.error.status == 422
    assert "Cannot edit" in e.value.error.message


def test_context_crtb(admin_mc, admin_cc, remove_resource,
                      user_mc):
    """Tests that any user cannot edit a project role binding from the cluster
    context
    """
    user_client = user_mc.client

    crtb = admin_mc.client.create_cluster_role_template_binding(
        userId=user_mc.user.id,
        roleTemplateId="cluster-owner",
        clusterId=admin_cc.cluster.id,
    )
    remove_resource(crtb)

    wait_until_available(user_client, admin_cc.cluster)

    crtb.roleTemplateId = "project-owner"
    with pytest.raises(ApiError) as e:
        crtb = user_client.update_by_id_clusterRoleTemplateBinding(crtb.id,
                                                                   crtb)
    assert e.value.error.status == 422
    assert "Cannot edit" in e.value.error.message

    crtb = admin_mc.client.reload(crtb)
    crtb.roleTemplateId = "project-owner"
    with pytest.raises(ApiError) as e:
        crtb = admin_mc.client.update_by_id_clusterRoleTemplateBinding(crtb.id,
                                                                       crtb)
    assert e.value.error.status == 422
    assert "Cannot edit" in e.value.error.message
