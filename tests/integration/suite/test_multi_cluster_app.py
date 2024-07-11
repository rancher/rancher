from .common import random_str, check_subject_in_rb

from rancher import ApiError

from .conftest import (
    wait_until, wait_for, set_server_version, wait_until_available,
    user_project_client
)
import time
import pytest
import kubernetes

roles_resource = 'roles'
projects_resource = 'projects'
members_resource = 'members'


def test_multiclusterapp_create_no_roles(admin_mc, admin_pc, remove_resource):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-1.0.5"
    targets = [{"projectId": admin_pc.project.id}]
    # should not be able to create without passing roles
    try:
        mcapp = client.create_multi_cluster_app(name=mcapp_name,
                                                templateVersionId=temp_ver,
                                                targets=targets)
        remove_resource(mcapp)
    except ApiError as e:
        assert e.error.status == 422


def test_mutliclusterapp_invalid_project(admin_mc, remove_resource):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-1.0.5"
    targets = [{"projectId": "abc:def"}]
    try:
        mcapp = client.create_multi_cluster_app(name=mcapp_name,
                                                templateVersionId=temp_ver,
                                                targets=targets)
        remove_resource(mcapp)
    except ApiError as e:
        assert e.error.status == 422


@pytest.mark.nonparallel
@pytest.mark.skip
def test_multiclusterapp_create_with_members(admin_mc, admin_pc,
                                             user_factory, remove_resource,
                                             ):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-1.0.5"

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

    # member should also get access to the mcapp revision
    if mcapp['status']['revisionId'] != '':
        mcapp_revision_id = "cattle-global-data:" + \
                            mcapp['status']['revisionId']
        mcr = um_client.\
            by_id_multi_cluster_app_revision(mcapp_revision_id)
        assert mcr is not None

    # user who's not a member shouldn't get access
    unm_client = user_not_member.client
    try:
        unm_client.by_id_multi_cluster_app(id)
    except ApiError as e:
        assert e.error.status == 403

    # add the special char * to indicate sharing of resource with all
    # authenticated users
    new_members = [{"userPrincipalId": "local://"+user_member.user.id,
                   "accessType": "read-only"}, {"groupPrincipalId": "*"}]
    client.update(mcapp, members=new_members, roles=roles)

    # now user_not_member should be able to access this mcapp without
    # being explicitly added
    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)
    split = mcapp.id.split(":")
    name = split[1]
    rb_name = name + "-m-r"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         'system:authenticated', rb_name),
             timeout=60, fail_handler=lambda:
             'failed to check updated rolebinding')

    mcapp = user_not_member.client.by_id_multi_cluster_app(id)
    assert mcapp is not None

    # even newly created users should be able to access this mcapp
    new_user = user_factory()
    remove_resource(new_user)
    mcapp = new_user.client.by_id_multi_cluster_app(id)
    assert mcapp is not None


@pytest.mark.skip
def test_multiclusterapp_admin_create(admin_mc, admin_pc, remove_resource):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-1.0.5"
    targets = [{"projectId": admin_pc.project.id}]
    roles = ["cluster-owner", "project-member"]
    # roles check should be relaxed for admin
    mcapp1 = client.create_multi_cluster_app(name=mcapp_name,
                                             templateVersionId=temp_ver,
                                             targets=targets,
                                             roles=roles)
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_name, 60)


@pytest.mark.skip
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
    temp_ver = "cattle-global-data:library-wordpress-1.0.5"
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


@pytest.mark.skip
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
    temp_ver = "cattle-global-data:library-wordpress-1.0.5"
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


@pytest.mark.skip
def test_multiclusterapp_user_create(admin_mc, admin_pc, remove_resource,
                                     user_factory):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-1.0.5"
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


@pytest.mark.skip
def test_multiclusterapp_admin_update_roles(admin_mc, admin_pc,
                                            remove_resource):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-1.0.5"
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
    client.update(mcapp1, roles=new_roles)
    wait_for(lambda: check_updated_roles(admin_mc, mcapp_name, new_roles),
             timeout=60, fail_handler=fail_handler(roles_resource))


@pytest.mark.skip
def test_multiclusterapp_user_update_roles(admin_mc, admin_pc, remove_resource,
                                           user_factory):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-1.0.5"
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
    user.client.update(mcapp1, roles=new_roles)
    wait_for(lambda: check_updated_roles(admin_mc, mcapp_name, new_roles),
             timeout=60, fail_handler=fail_handler(roles_resource))


@pytest.mark.skip
def test_admin_access(admin_mc, admin_pc, user_factory, remove_resource):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-1.0.5"
    targets = [{"projectId": admin_pc.project.id}]
    user = user_factory()
    remove_resource(user)
    prtb_member = client.create_project_role_template_binding(
        projectId=admin_pc.project.id,
        roleTemplateId="project-member",
        userId=user.user.id)
    remove_resource(prtb_member)
    wait_until(rtb_cb(client, prtb_member))
    mcapp1 = user.client.\
        create_multi_cluster_app(name=mcapp_name,
                                 templateVersionId=temp_ver,
                                 targets=targets,
                                 roles=["project-member"])
    wait_for_app(admin_pc, mcapp_name, 60)
    client.update(mcapp1, roles=["cluster-owner"])
    wait_for(lambda: check_updated_roles(admin_mc, mcapp_name,
                                         ["cluster-owner"]), timeout=60,
             fail_handler=fail_handler(roles_resource))


@pytest.mark.skip
def test_add_projects(admin_mc, admin_pc, admin_cc, remove_resource):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-1.0.5"
    targets = [{"projectId": admin_pc.project.id}]
    mcapp1 = client.\
        create_multi_cluster_app(name=mcapp_name,
                                 templateVersionId=temp_ver,
                                 targets=targets,
                                 roles=["project-member"])
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_name, 60)
    p = client.create_project(name='test-' + random_str(),
                              clusterId=admin_cc.cluster.id)
    remove_resource(p)
    p = admin_cc.management.client.wait_success(p)
    client.action(obj=mcapp1, action_name="addProjects",
                  projects=[p.id])
    new_projects = [admin_pc.project.id, p.id]
    wait_for(lambda: check_updated_projects(admin_mc, mcapp_name,
                                            new_projects), timeout=60,
             fail_handler=fail_handler(projects_resource))


@pytest.mark.skip
def test_remove_projects(admin_mc, admin_pc, admin_cc, remove_resource):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-wordpress-1.0.5"
    p = client.create_project(name='test-' + random_str(),
                              clusterId=admin_cc.cluster.id)
    remove_resource(p)
    p = admin_cc.management.client.wait_success(p)
    targets = [{"projectId": admin_pc.project.id}, {"projectId": p.id}]
    mcapp1 = client. \
        create_multi_cluster_app(name=mcapp_name,
                                 templateVersionId=temp_ver,
                                 targets=targets,
                                 roles=["project-member"])
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_name, 60)
    client.action(obj=mcapp1, action_name="removeProjects", projects=[p.id])
    new_projects = [admin_pc.project.id]
    wait_for(lambda: check_updated_projects(admin_mc, mcapp_name,
                                            new_projects), timeout=60,
             fail_handler=fail_handler(projects_resource))


@pytest.mark.skip
def test_multiclusterapp_revision_access(admin_mc, admin_pc, remove_resource,
                                         user_factory):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-mysql-0.3.8"
    targets = [{"projectId": admin_pc.project.id}]
    user = user_factory()
    remove_resource(user)
    user_client = user.client
    # assign user to local cluster as project-member
    prtb_member = client.create_project_role_template_binding(
        projectId=admin_pc.project.id,
        roleTemplateId="project-member",
        userId=user.user.id)

    remove_resource(prtb_member)
    wait_until(rtb_cb(client, prtb_member))
    roles = ["project-member"]
    mcapp1 = user_client.create_multi_cluster_app(name=mcapp_name,
                                                  templateVersionId=temp_ver,
                                                  targets=targets,
                                                  roles=roles)
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_name, 60)

    mcapp_revisions = user_client.list_multi_cluster_app_revision()
    assert len(mcapp_revisions) == 1


@pytest.mark.skip(reason='flaky test maybe, skipping for now')
def test_app_upgrade_mcapp_roles_change(admin_mc, admin_pc,
                                        remove_resource):
    client = admin_mc.client
    mcapp_name = random_str()
    temp_ver = "cattle-global-data:library-grafana-0.0.31"
    targets = [{"projectId": admin_pc.project.id}]
    roles = ["project-member"]
    mcapp1 = client.create_multi_cluster_app(name=mcapp_name,
                                             templateVersionId=temp_ver,
                                             targets=targets,
                                             roles=roles)
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_name, 60)
    # changing roles should trigger app upgrade
    roles = ["cluster-owner"]
    client.update(mcapp1, roles=roles)
    wait_for_app_condition(admin_pc, mcapp_name, 'UserTriggeredAction', 60)


def wait_for_app_condition(admin_pc, name, condition, timeout=60):
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
            conditions = apps['data'][0]['conditions']
            for c in conditions:
                if c['type'] == condition and\
                        c['status'] == 'True':
                    found = True
        time.sleep(interval)
        interval *= 2


@pytest.mark.nonparallel
@pytest.mark.skip
def test_mcapp_create_validation(admin_mc, admin_pc, custom_catalog,
                                 remove_resource, restore_rancher_version):
    """Test create validation of multi cluster apps. This test will set the
    rancher version explicitly and attempt to create apps with rancher version
    requirements
    """
    # 1.6.0 uses 2.0.0-2.2.0
    # 1.6.2 uses 2.1.0-2.3.0
    c_name = random_str()
    custom_catalog(name=c_name)

    client = admin_mc.client
    server_version = "2.0.0"
    set_server_version(client, server_version)

    cat_ns_name = "cattle-global-data:"+c_name

    mcapp_data = {
        'name': random_str(),
        'templateVersionId': cat_ns_name+"-chartmuseum-1.6.2",
        'targets': [{"projectId": admin_pc.project.id}],
        'roles': ["cluster-owner", "project-member"],
    }

    # First app requires a min rancher version of 2.1 so we expect an error
    with pytest.raises(ApiError) as e:
        mcapp1 = client.create_multi_cluster_app(mcapp_data)
        remove_resource(mcapp1)
    assert e.value.error.status == 422
    assert 'incompatible rancher version [%s] for template' % server_version \
        in e.value.error.message

    # Second app requires a min of 2.0 so no error should be returned
    mcapp_data['name'] = random_str()
    mcapp_data['templateVersionId'] = cat_ns_name+"-chartmuseum-1.6.0",
    mcapp2 = client.create_multi_cluster_app(mcapp_data)
    remove_resource(mcapp2)
    wait_for_app(admin_pc, mcapp_data['name'])

    server_version = "2.2.1"
    set_server_version(client, server_version)
    # Third app requires a max of version 2.2.0 so expect error
    with pytest.raises(ApiError) as e:
        mcapp_data['name'] = random_str()
        mcapp3 = client.create_multi_cluster_app(mcapp_data)
        remove_resource(mcapp3)
    assert e.value.error.status == 422
    assert 'incompatible rancher version [%s] for template' % server_version \
        in e.value.error.message


@pytest.mark.nonparallel
@pytest.mark.skip
def test_mcapp_update_validation(admin_mc, admin_pc, custom_catalog,
                                 remove_resource, restore_rancher_version):
    """Test update validation of multi cluster apps. This test will set the
    rancher version explicitly and attempt to update an app with rancher
    version requirements
    """
    # 1.6.0 uses 2.0.0-2.2.0
    # 1.6.2 uses 2.1.0-2.3.0
    c_name = random_str()
    custom_catalog(name=c_name)

    client = admin_mc.client
    server_version = "2.0.0"
    set_server_version(client, server_version)

    cat_ns_name = "cattle-global-data:"+c_name

    mcapp_data = {
        'name': random_str(),
        'templateVersionId': cat_ns_name+"-chartmuseum-1.6.0",
        'targets': [{"projectId": admin_pc.project.id}],
        'roles': ["cluster-owner", "project-member"],
    }

    # First app requires a min rancher version of 2.0 so no error
    mcapp1 = client.create_multi_cluster_app(mcapp_data)
    remove_resource(mcapp1)
    wait_for_app(admin_pc, mcapp_data['name'])

    # App upgrade requires a min of 2.1 so expect error
    with pytest.raises(ApiError) as e:
        mcapp1 = client.update_by_id_multi_cluster_app(
            id=mcapp1.id, templateVersionId=cat_ns_name+"-chartmuseum-1.6.2")
    assert e.value.error.status == 422
    assert 'incompatible rancher version [%s] for template' % server_version \
        in e.value.error.message

    server_version = "2.3.1"
    set_server_version(client, server_version)
    # App upgrade requires a max of 2.3 so expect error
    with pytest.raises(ApiError) as e:
        mcapp1 = client.update_by_id_multi_cluster_app(
            id=mcapp1.id, templateVersionId=cat_ns_name+"-chartmuseum-1.6.2")
    assert e.value.error.status == 422
    assert 'incompatible rancher version [%s] for template' % server_version \
        in e.value.error.message


@pytest.mark.skip
def test_perform_mca_action_read_only(admin_mc, admin_pc, remove_resource,
                                      user_mc, user_factory):
    """Tests MCA actions with a read-only user and a member user."""
    client = admin_mc.client
    project = admin_pc.project
    user = user_mc
    user_member = user_factory()

    ns = admin_pc.cluster.client.create_namespace(
        name=random_str(),
        projectId=project.id)
    remove_resource(ns)

    # Create a read-only user binding.
    prtb1 = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user.user.id,
        projectId=project.id,
        roleTemplateId="read-only")
    remove_resource(prtb1)
    wait_until_available(user.client, project)

    # Then, create a member user binding.
    prtb2 = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user_member.user.id,
        projectId=project.id,
        roleTemplateId="project-member")
    remove_resource(prtb2)
    wait_until_available(user_member.client, project)
    user_pc = user_project_client(user, project)
    user_member_pc = user_project_client(user_member, project)

    # Admin user creates the MCA and specifically adds both users. The
    # project-member user should have permissions by default since their role
    # is specified in the MCA creation.
    mcapp_name = random_str()
    mcapp_user_read_only = "local://" + user.user.id
    mcapp_user_member = "local://" + user_member.user.id
    mcapp = client.create_multi_cluster_app(
        name=mcapp_name,
        templateVersionId="cattle-global-data:library-docker-registry-1.9.2",
        targets=[{"projectId": admin_pc.project.id}],
        members=[{"userPrincipalId": mcapp_user_read_only,
                  "accessType": "read-only"},
                 {"userPrincipalId": mcapp_user_member,
                  "accessType": "member"}],
        roles=["cluster-owner", "project-member"])
    remove_resource(mcapp)
    wait_for_app(admin_pc, mcapp_name)

    # Admin user updates the MCA to yield a rollback option. We change the
    # image version below.
    mcapp = client.reload(mcapp)
    original_rev = mcapp.revisions().data[0].name
    mcapp.templateVersionId = (
        "cattle-global-data:library-docker-registry-1.8.1")
    mcapp = client.update_by_id_multi_cluster_app(mcapp.id, mcapp)
    wait_for_app(admin_pc, mcapp_name)
    mcapp = client.reload(mcapp)

    # Read-only users should receive a 404 error.
    with pytest.raises(ApiError) as e:
        user_pc.action(obj=mcapp, action_name="rollback",
                       revisionId=original_rev)
    assert e.value.error.status == 404

    # Member users will be able to perform the rollback.
    user_member_pc.action(obj=mcapp, action_name="rollback",
                          revisionId=original_rev)


def wait_for_app(admin_pc, name, timeout=60):
    start = time.time()
    interval = 0.5
    client = admin_pc.client
    project_id = admin_pc.project.id.split(':')[1]
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


def rtb_cb(client, rtb):
    """Wait for the prtb to have the userId populated"""
    def cb():
        rt = client.reload(rtb)
        return rt.userPrincipalId is not None
    return cb


def check_updated_projects(admin_mc, mcapp_name, projects):
    mcapp_projects = []
    id = "cattle-global-data:" + mcapp_name
    mcapp = admin_mc.client.by_id_multi_cluster_app(id)
    for t in mcapp.targets:
        mcapp_projects.append(t.projectId)
    if mcapp_projects == projects:
        return True
    return False


def check_updated_roles(admin_mc, mcapp_name, roles):
    id = "cattle-global-data:" + mcapp_name
    mcapp = admin_mc.client.by_id_multi_cluster_app(id)
    if mcapp is not None and mcapp.roles == roles:
        return True
    return False


def fail_handler(resource):
    return "failed waiting for multiclusterapp " + resource + " to get updated"
