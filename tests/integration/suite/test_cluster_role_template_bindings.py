import pytest

from .common import random_str
from .conftest import wait_for
from rancher import ApiError


def test_cannot_target_users_and_group(admin_mc, remove_resource):
    """Asserts that a clusterroletemplatebinding cannot target both
    user and group subjects"""
    admin_client = admin_mc.client

    with pytest.raises(ApiError) as e:
        crtb = admin_client.create_cluster_role_template_binding(
            name="crtb-"+random_str(),
            clusterId="local",
            userId=admin_mc.user.id,
            groupPrincipalId="someauthprovidergroupid",
            roleTemplateId="clustercatalogs-view")
        remove_resource(crtb)
    assert e.value.error.status == 422
    assert "must target a user [userId]/[userPrincipalId] OR a group " \
        "[groupId]/[groupPrincipalId]" in e.value.error.message


def test_must_have_target(admin_mc, remove_resource):
    """Asserts that a clusterroletemplatebinding must have a subject"""
    admin_client = admin_mc.client

    with pytest.raises(ApiError) as e:
        crtb = admin_client.create_cluster_role_template_binding(
            name="crtb-" + random_str(),
            clusterId="local",
            roleTemplateId="clustercatalogs-view")
        remove_resource(crtb)
    assert e.value.error.status == 422
    assert "must target a user [userId]/[userPrincipalId] OR a group " \
           "[groupId]/[groupPrincipalId]" in e.value.error.message


def test_cannot_update_subjects_or_cluster(admin_mc, remove_resource):
    """Asserts non-metadata fields cannot be updated"""
    admin_client = admin_mc.client
    old_crtb = admin_client.create_cluster_role_template_binding(
        name="crtb-" + random_str(),
        clusterId="local",
        userId=admin_mc.user.id,
        roleTemplateId="clustercatalogs-view")
    remove_resource(old_crtb)

    wait_for(lambda: admin_client.reload(old_crtb).userPrincipalId is not None)
    old_crtb = admin_client.reload(old_crtb)

    crtb = admin_client.update_by_id_cluster_role_template_binding(
        id=old_crtb.id,
        clusterId="fakecluster",
        userId="",
        userPrincipalId="asdf",
        groupPrincipalId="asdf",
        group="asdf"
    )

    assert crtb.get("clusterId") == old_crtb.get("clusterId")
    assert crtb.get("userId") == old_crtb.get("userId")
    assert crtb.get("userPrincipalId") == old_crtb.get("userPrincipalId")
    assert crtb.get("groupPrincipalId") == old_crtb.get("groupPrincipalId")
    assert crtb.get("group") == old_crtb.get("group")
