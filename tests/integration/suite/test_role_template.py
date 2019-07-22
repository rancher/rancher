from .common import random_str
from .conftest import wait_until_available
from rancher import ApiError
import time
import pytest


def test_role_template_creation(admin_mc, remove_resource):
    rt_name = random_str()
    rt = admin_mc.client.create_role_template(name=rt_name)
    remove_resource(rt)
    assert rt is not None
    assert rt.name == rt_name


def test_administrative_role_template_creation(admin_mc, remove_resource):
    client = admin_mc.client
    crt_name = random_str()
    crt = client.create_role_template(name=crt_name,
                                      context="cluster",
                                      administrative=True)
    remove_resource(crt)
    assert crt is not None
    assert crt.name == crt_name

    prt_name = random_str()
    try:
        client.create_role_template(name=prt_name,
                                    context="project",
                                    administrative=True)
    except ApiError as e:
        assert e.error.status == 500
        assert e.error.message == "Only cluster roles can be administrative"


def test_edit_builtin_role_template(admin_mc, remove_resource):
    client = admin_mc.client
    # edit non builtin role, any field is updatable
    org_rt_name = random_str()
    rt = client.create_role_template(name=org_rt_name)
    remove_resource(rt)
    wait_for_role_template_creation(admin_mc, org_rt_name)
    new_rt_name = random_str()
    rt = client.update(rt, name=new_rt_name)
    assert rt.name == new_rt_name

    # edit builtin role, only locked,cluster/projectcreatordefault
    # are updatable
    new_rt_name = "Cluster Member-Updated"
    cm_rt = client.by_id_role_template("cluster-member")
    rt = client.update(cm_rt, name=new_rt_name)
    assert rt.name == "Cluster Member"


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


def wait_for_role_template_creation(admin_mc, rt_name, timeout=60):
    start = time.time()
    interval = 0.5
    client = admin_mc.client
    found = False
    while not found:
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for roletemplate creation')
        rt = client.list_role_template(name=rt_name)
        if len(rt) > 0:
            found = True
        time.sleep(interval)
        interval *= 2
