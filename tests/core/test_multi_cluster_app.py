from .common import random_str
from rancher import ApiError
from .conftest import wait_until
import time


def test_multiclusterapp_create_no_roles(admin_mc, admin_pc, remove_resource,
                                         user_factory):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-2.1.10"
    targets = [{"projectId": admin_pc.project.id}]
    # should not be able to create without passing roles
    try:
        client.create_multi_cluster_app(name=mcapp_name,
                                        templateVersionId=temp_ver,
                                        targets=targets)
    except ApiError as e:
        assert e.error.status == 422


def test_mutliclusterapp_invalid_project(admin_mc):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-2.1.10"
    targets = [{"projectId": "abc:def"}]
    try:
        client.create_multi_cluster_app(name=mcapp_name,
                                        templateVersionId=temp_ver,
                                        targets=targets)
    except ApiError as e:
        assert e.error.status == 422


def test_multiclusterapp_create_with_members(admin_mc, admin_pc,
                                             user_factory, remove_resource):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-2.1.10"

    targets = [{"projectId": admin_pc.project.id}]

    user_member = user_factory()
    remove_resource(user_member)
    user_not_member = user_factory()
    remove_resource(user_not_member)
    members = [{"userPrincipalId": "local://"+user_member.user.id,
                "accessType": "read-only"}]
    roles = ["cluster-owner", "project-member"]

    mcapp1 = client.create_multi_cluster_app(name=mcapp_name,
                                             templateVersionId=temp_ver,
                                             targets=targets,
                                             members=members,
                                             roles=roles)
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_name, 60)

    # check who has access to the multiclusterapp
    # admin and user_member should be able to list it
    id = "cattle-global-data:" + mcapp_name
    mcapp = client.by_id_multi_cluster_app(id)
    assert mcapp is not None
    um_client = user_member.client
    mcapp = um_client.by_id_multi_cluster_app(id)
    assert mcapp is not None

    unm_client = user_not_member.client
    try:
        unm_client.by_id_multi_cluster_app(id)
    except ApiError as e:
        assert e.error.status == 403


def test_multiclusterapp_admin_create(admin_mc, admin_pc, remove_resource):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-2.1.10"
    targets = [{"projectId": admin_pc.project.id}]
    roles = ["cluster-owner", "project-member"]
    # roles check should be relaxed for admin
    mcapp1 = client.create_multi_cluster_app(name=mcapp_name,
                                             templateVersionId=temp_ver,
                                             targets=targets,
                                             roles=roles)
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_name, 60)


def test_multiclusterapp_cluster_owner_create(admin_mc, admin_pc,
                                              remove_resource, user_factory):
    client = admin_mc.client
    mcapp_name = random_str()
    cowner = user_factory()
    crtb_owner = client.create_cluster_role_template_binding(
        clusterId="local",
        roleTemplateId="cluster-owner",
        userId=cowner.user.id)
    remove_resource(crtb_owner)
    wait_until(rtb_cb(client, crtb_owner))
    temp_ver = "cattle-global-data:library-wordpress-2.1.10"
    targets = [{"projectId": admin_pc.project.id}]
    roles = ["cluster-owner", "project-member"]
    # user isn't explicitly added as project-member, but this check should be
    # relaxed since user is added as cluster-owner
    mcapp1 = cowner.client.\
        create_multi_cluster_app(name=mcapp_name,
                                 templateVersionId=temp_ver,
                                 targets=targets,
                                 roles=roles)
    remove_resource(mcapp1)


def test_multiclusterapp_project_owner_create(admin_mc, admin_pc,
                                              remove_resource, user_factory):
    client = admin_mc.client
    mcapp_name = random_str()
    powner = user_factory()
    prtb_owner = client.create_project_role_template_binding(
        projectId=admin_pc.project.id,
        roleTemplateId="project-owner",
        userId=powner.user.id)
    remove_resource(prtb_owner)
    wait_until(rtb_cb(client, prtb_owner))
    temp_ver = "cattle-global-data:library-wordpress-2.1.10"
    targets = [{"projectId": admin_pc.project.id}]
    roles = ["project-member"]
    # user isn't explicitly added as project-member, but this check should be
    # relaxed since user is added as project-owner
    mcapp1 = powner.client.\
        create_multi_cluster_app(name=mcapp_name,
                                 templateVersionId=temp_ver,
                                 targets=targets,
                                 roles=roles)
    remove_resource(mcapp1)


def test_multiclusterapp_user_create(admin_mc, admin_pc, remove_resource,
                                     user_factory):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-2.1.10"
    targets = [{"projectId": admin_pc.project.id}]
    # make regular user cluster-owner and project-owner in the cluster and
    # it's project
    user = user_factory()
    remove_resource(user)
    user_client = user.client
    crtb_owner = client.create_cluster_role_template_binding(
        clusterId="local",
        roleTemplateId="cluster-owner",
        userId=user.user.id)
    remove_resource(crtb_owner)
    wait_until(rtb_cb(client, crtb_owner))
    prtb_member = client.create_project_role_template_binding(
        projectId=admin_pc.project.id,
        roleTemplateId="project-member",
        userId=user.user.id)
    remove_resource(prtb_member)
    wait_until(rtb_cb(client, prtb_member))
    roles = ["cluster-owner", "project-member"]
    mcapp1 = user_client.create_multi_cluster_app(name=mcapp_name,
                                                  templateVersionId=temp_ver,
                                                  targets=targets,
                                                  roles=roles)
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_name, 60)

    # try creating as a user who is not cluster-owner,
    # but that is one of the roles listed, must fail
    user_no_roles = user_factory()
    remove_resource(user_no_roles)
    # add user to project as member but not to cluster as owner
    prtb_member = client.create_project_role_template_binding(
        projectId=admin_pc.project.id,
        roleTemplateId="project-member",
        userId=user_no_roles.user.id)
    remove_resource(prtb_member)

    wait_until(rtb_cb(client, prtb_member))
    try:
        user_no_roles.client.\
            create_multi_cluster_app(name=random_str(),
                                     templateVersionId=temp_ver,
                                     targets=targets,
                                     roles=roles)
    except ApiError as e:
        assert e.error.status == 403
        assert "does not have roles cluster-owner in cluster"\
               in e.error.message
        assert "cluster-owner" in e.error.message


def test_multiclusterapp_admin_update_roles(admin_mc, admin_pc,
                                            remove_resource):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-2.1.10"
    targets = [{"projectId": admin_pc.project.id}]
    roles = ["project-member"]
    mcapp1 = client.create_multi_cluster_app(name=mcapp_name,
                                             templateVersionId=temp_ver,
                                             targets=targets,
                                             roles=roles)
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_name, 60)

    # admin doesn't get cluster/project roles (crtb/prtb) by default
    # but updating the mcapp to add these roles must pass, since global admin
    # should have access to everything and must be excused
    new_roles = ["cluster-owner", "project-member"]
    updated_mcapp = client.update(mcapp1, roles=new_roles)
    wait_for_roles_to_be_updated(admin_mc, updated_mcapp, new_roles)


def test_multiclusterapp_user_update_roles(admin_mc, admin_pc, remove_resource,
                                           user_factory):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-2.1.10"
    targets = [{"projectId": admin_pc.project.id}]
    # create mcapp as admin, passing "cluster-owner" role
    roles = ["cluster-owner"]
    # add a user as a member with access-type owner
    user = user_factory()
    remove_resource(user)
    members = [{"userPrincipalId": "local://" + user.user.id,
                "accessType": "owner"}]
    mcapp1 = client.create_multi_cluster_app(name=mcapp_name,
                                             templateVersionId=temp_ver,
                                             targets=targets,
                                             roles=roles,
                                             members=members)
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_name, 60)

    # user wants to update roles to add project-member role
    # but user is not a part of target project, so this must fail
    new_roles = ["cluster-owner", "project-member"]
    try:
        user.client.update(mcapp1, roles=new_roles)
    except ApiError as e:
        assert e.error.status == 403
        assert "does not have roles project-member in project" \
               in e.error.message
        assert "of cluster local" in e.error.message

    # now admin adds this user to project as project-member
    prtb_member = client.create_project_role_template_binding(
        projectId=admin_pc.project.id,
        roleTemplateId="project-member",
        userId=user.user.id)
    remove_resource(prtb_member)
    wait_until(rtb_cb(client, prtb_member))

    # now user should be able to add project-member role
    updated_mcapp = user.client.update(mcapp1, roles=new_roles)
    wait_for_roles_to_be_updated(admin_mc, updated_mcapp, new_roles)


def wait_for_app(admin_pc, name, timeout=60):
    start = time.time()
    interval = 0.5
    client = admin_pc.client
    cluster_id, project_id = admin_pc.project.id.split(':')
    app_name = name+"-"+project_id
    found = False
    while not found:
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for app of multiclusterapp')
        apps = client.list_app(name=app_name)
        if len(apps) > 0:
            found = True
        time.sleep(interval)
        interval *= 2


def wait_for_roles_to_be_updated(admin_mc, mcapp, roles, timeout=60):
    start = time.time()
    interval = 0.5
    found = False
    id = "cattle-global-data:" + mcapp.name
    while not found:
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for update to '
                            'multiclusterapp roles')

        mcapp = admin_mc.client.by_id_multi_cluster_app(id)
        if mcapp is not None and mcapp.roles == roles:
            found = True
        time.sleep(interval)
        interval *= 2


def rtb_cb(client, rtb):
    """Wait for the prtb to have the userId populated"""
    def cb():
        rt = client.reload(rtb)
        return rt.userPrincipalId is not None
    return cb
