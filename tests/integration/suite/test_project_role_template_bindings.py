import pytest

from .common import random_str
from .conftest import wait_for
from rancher import ApiError


def test_cannot_target_users_and_group(admin_mc, remove_resource):
    """Asserts that a projectroletemplatebinding cannot target both
    user and group subjects"""
    admin_client = admin_mc.client

    project = admin_client.create_project(
        name="p-" + random_str(),
        clusterId="local")
    remove_resource(project)

    with pytest.raises(ApiError) as e:
        prtb = admin_client.create_project_role_template_binding(
            name="prtb-"+random_str(),
            projectId=project.id,
            userId=admin_mc.user.id,
            groupPrincipalId="someauthprovidergroupid",
            roleTemplateId="projectcatalogs-view")
        remove_resource(prtb)
    assert e.value.error.status == 422
    assert "must target a user [userId]/[userPrincipalId] OR a group " \
        "[groupId]/[groupPrincipalId]" in e.value.error.message


def test_must_have_target(admin_mc, admin_pc, remove_resource):
    """Asserts that a projectroletemplatebinding must have a subject"""
    admin_client = admin_mc.client

    with pytest.raises(ApiError) as e:
        prtb = admin_client.create_project_role_template_binding(
            name="prtb-" + random_str(),
            projectId=admin_pc.project.id,
            roleTemplateId="projectcatalogs-view")
        remove_resource(prtb)
    assert e.value.error.status == 422
    assert "must target a user [userId]/[userPrincipalId] OR a group " \
           "[groupId]/[groupPrincipalId]" in e.value.error.message


def test_cannot_update_subject_or_proj(admin_mc, admin_pc, remove_resource):
    """Asserts non-metadata fields cannot be updated"""
    admin_client = admin_mc.client

    old_prtb = admin_client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        projectId=admin_pc.project.id,
        userId=admin_mc.user.id,
        roleTemplateId="projectcatalogs-view")
    remove_resource(old_prtb)

    wait_for(lambda: admin_client.reload(old_prtb).userPrincipalId is not None)
    old_prtb = admin_client.reload(old_prtb)

    prtb = admin_client.update_by_id_project_role_template_binding(
        id=old_prtb.id,
        clusterId="fakeproject",
        userId="",
        userPrincipalId="asdf",
        groupPrincipalId="asdf",
        group="asdf"
    )

    assert prtb.get("projectId") == old_prtb.get("projectId")
    assert prtb.get("userId") == old_prtb.get("userId")
    assert prtb.get("userPrincipalId") == old_prtb.get("userPrincipalId")
    assert prtb.get("groupPrincipalId") == old_prtb.get("groupPrincipalId")
    assert prtb.get("group") == old_prtb.get("group")
