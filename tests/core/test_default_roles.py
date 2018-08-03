import pytest
import json
from .common import random_str
from .conftest import wait_for_condition, wait_until

CREATOR_ANNOTATION = 'authz.management.cattle.io/creator-role-bindings'


@pytest.fixture
def cleanup_roles(request, admin_mc):
    """Resets global roles and role remplates back to the server default:
    global role == 'user'
    cluster create == 'cluster-owner'
    project create == 'project-owner'
    """
    client = admin_mc.client

    def _cleanup():
        for role in client.list_role_template():
            if role.id == 'cluster-owner':
                client.update(role, clusterCreatorDefault=True,
                              projectCreatorDefault=False, locked=False)
            elif role.id == 'project-owner':
                client.update(role, clusterCreatorDefault=False,
                              projectCreatorDefault=True, locked=False)
            elif (role.clusterCreatorDefault or role.projectCreatorDefault or
                  role.locked):
                client.update(role, clusterCreatorDefault=False,
                              projectCreatorDefault=False, locked=False)

        for role in client.list_global_role():
            if role.id == 'user':
                client.update(role, newUserDefault=True)
            elif role.newUserDefault:
                client.update(role, newUserDefault=False)

    request.addfinalizer(_cleanup)


@pytest.mark.nonparallel
def test_cluster_create_default_role(admin_mc, cleanup_roles, remove_resource):
    test_roles = ['projects-create', 'storage-manage', 'nodes-view']
    client = admin_mc.client

    set_role_state(client, test_roles, 'cluster')

    cluster = client.create_cluster(name=random_str())
    remove_resource(cluster)

    wait_for_condition('InitialRolesPopulated', 'True', client, cluster)

    cluster = client.reload(cluster)

    data_dict = json.loads(cluster.annotations.data_dict()[CREATOR_ANNOTATION])

    assert len(cluster.clusterRoleTemplateBindings()) == 3
    assert set(data_dict['created']) == set(data_dict['required'])
    assert set(data_dict['created']) == set(test_roles)

    for binding in cluster.clusterRoleTemplateBindings():
        assert binding.roleTemplateId in test_roles


@pytest.mark.nonparallel
def test_cluster_create_role_locked(admin_mc, cleanup_roles, remove_resource):
    test_roles = ['projects-create', 'storage-manage', 'nodes-view']
    client = admin_mc.client

    set_role_state(client, test_roles, 'cluster')

    # Grab a role to lock
    locked_role = test_roles.pop()

    # Lock the role
    client.update(client.by_id_role_template(locked_role), locked=True)

    cluster = client.create_cluster(name=random_str())
    remove_resource(cluster)

    wait_for_condition('InitialRolesPopulated', 'True', client, cluster)

    cluster = client.reload(cluster)

    data_dict = json.loads(cluster.annotations.data_dict()[CREATOR_ANNOTATION])

    assert len(cluster.clusterRoleTemplateBindings()) == 2
    assert set(data_dict['created']) == set(data_dict['required'])
    assert set(data_dict['created']) == set(test_roles)

    for binding in cluster.clusterRoleTemplateBindings():
        assert binding.roleTemplateId in test_roles


@pytest.mark.nonparallel
def test_project_create_default_role(admin_mc, cleanup_roles, remove_resource):
    test_roles = ['project-member', 'workloads-view', 'secrets-view']
    client = admin_mc.client

    set_role_state(client, test_roles, 'project')

    project = client.create_project(name=random_str(), clusterId='local')
    remove_resource(project)

    wait_for_condition('InitialRolesPopulated', 'True', client, project)

    project = client.reload(project)

    data_dict = json.loads(project.annotations.data_dict()[
        CREATOR_ANNOTATION])

    assert len(project.projectRoleTemplateBindings()) == 3
    assert set(data_dict['required']) == set(test_roles)

    for binding in project.projectRoleTemplateBindings():
        assert binding.roleTemplateId in test_roles


@pytest.mark.nonparallel
def test_project_create_role_locked(admin_mc, cleanup_roles, remove_resource):
    """Test a locked role that is set to default is not applied
    """
    test_roles = ['project-member', 'workloads-view', 'secrets-view']
    client = admin_mc.client

    set_role_state(client, test_roles, 'project')

    # Grab a role to lock
    locked_role = test_roles.pop()

    # Lock the role
    client.update(client.by_id_role_template(locked_role), locked=True)

    project = client.create_project(name=random_str(), clusterId='local')
    remove_resource(project)

    wait_for_condition('InitialRolesPopulated', 'True', client, project)

    project = client.reload(project)

    data_dict = json.loads(project.annotations.data_dict()[
        CREATOR_ANNOTATION])

    assert len(project.projectRoleTemplateBindings()) == 2
    assert set(data_dict['required']) == set(test_roles)

    for binding in project.projectRoleTemplateBindings():
        assert binding.roleTemplateId in test_roles


@pytest.mark.nonparallel
def test_user_create_default_role(admin_mc, cleanup_roles, remove_resource):
    test_roles = ['user-base', 'settings-manage', 'catalogs-use']
    principal = "local://fakeuser"
    client = admin_mc.client

    set_role_state(client, test_roles, 'global')

    # Creating a crtb with a fake principal causes the user to be created
    # through usermanager.EnsureUser. This triggers the creation of default
    # globalRoleBinding
    crtb = client.create_cluster_role_template_binding(
        clusterId="local",
        roleTemplateId="cluster-owner",
        userPrincipalId=principal)
    remove_resource(crtb)

    wait_until(crtb_cb(client, crtb))

    crtb = client.reload(crtb)

    user = client.by_id_user(crtb.userId)
    remove_resource(user)

    wait_for_condition('InitialRolesPopulated',
                       'True', client, user, timeout=5)

    user = client.reload(user)
    assert len(user.globalRoleBindings()) == 3
    for binding in user.globalRoleBindings():
        assert binding.globalRoleId in test_roles


def set_role_state(client, roles, context):
    """Set the default templates for globalRole or roleTemplates"""
    if context == 'cluster' or context == 'project':
        existing_roles = client.list_role_template()

        for role in existing_roles:
            client.update(role, clusterCreatorDefault=False,
                          projectCreatorDefault=False)

        for role in roles:
            if context == 'cluster':
                client.update(client.by_id_role_template(
                    role), clusterCreatorDefault=True)
            elif context == 'project':
                client.update(client.by_id_role_template(
                    role), projectCreatorDefault=True)

    elif context == 'global':
        existing_roles = client.list_global_role()

        for role in existing_roles:
            client.update(role, newUserDefault=False)

        for role in roles:
            client.update(client.by_id_global_role(role), newUserDefault=True)


def crtb_cb(client, crtb):
    """Wait for the crtb to have the userId populated"""
    def cb():
        c = client.reload(crtb)
        return c.userId is not None
    return cb
