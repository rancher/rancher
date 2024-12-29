import pytest
import kubernetes
import json
from .common import random_str
from .conftest import (
    wait_for_condition, wait_until, wait_for, kubernetes_api_client)

CREATOR_ANNOTATION = 'authz.management.cattle.io/creator-role-bindings'
systemProjectLabel = "authz.management.cattle.io/system-project"
defaultProjectLabel = "authz.management.cattle.io/default-project"


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


@pytest.fixture(autouse=True)
def add_cluster_roles(admin_mc, remove_resource):
    k8s_client = kubernetes_api_client(admin_mc.client, 'local')
    rbac_api = kubernetes.client.RbacAuthorizationV1Api(k8s_client)

    roles = rbac_api.list_cluster_role()

    cr1 = add_cr_if_not_exist(roles=roles,
                              name="monitoring-ui-view",
                              rbac_api=rbac_api)

    cr2 = add_cr_if_not_exist(roles=roles,
                              name="navlinks-view",
                              rbac_api=rbac_api)

    cr3 = add_cr_if_not_exist(roles=roles,
                              name="navlinks-manage",
                              rbac_api=rbac_api)

    yield

    # if the cluster role didn't exist before, delete it after the test
    if cr1:
        rbac_api.delete_cluster_role("monitoring-ui-view")
    if cr2:
        rbac_api.delete_cluster_role("navlinks-view")
    if cr3:
        rbac_api.delete_cluster_role("navlinks-manage")


@pytest.mark.nonparallel
def test_cluster_create_default_role(admin_mc, cleanup_roles, remove_resource):
    test_roles = ['projects-create', 'storage-manage', 'nodes-view']
    client = admin_mc.client

    set_role_state(client, test_roles, 'cluster')

    cluster = client.create_cluster(name=random_str())
    remove_resource(cluster)

    wait_for_condition('InitialRolesPopulated', 'True', client, cluster)

    cluster = client.reload(cluster)

    data_dict = json.loads(cluster.annotations[CREATOR_ANNOTATION])

    assert len(cluster.clusterRoleTemplateBindings()) == 3
    assert set(data_dict['created']) == set(data_dict['required'])
    assert set(data_dict['created']) == set(test_roles)

    for binding in cluster.clusterRoleTemplateBindings():
        def binding_principal_validate():
            bind = client.by_id_cluster_role_template_binding(binding.id)
            if bind.userPrincipalId is None:
                return False
            return bind

        binding = wait_for(binding_principal_validate)
        assert binding.roleTemplateId in test_roles
        assert binding.userId is not None
        user = client.by_id_user(binding.userId)
        assert binding.userPrincipalId in user.principalIds


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

    data_dict = json.loads(cluster.annotations[CREATOR_ANNOTATION])

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

    data_dict = json.loads(project.annotations[
        CREATOR_ANNOTATION])

    assert len(project.projectRoleTemplateBindings()) == 3
    assert set(data_dict['required']) == set(test_roles)

    for binding in project.projectRoleTemplateBindings():
        def binding_principal_validate():
            bind = client.by_id_project_role_template_binding(binding.id)
            if bind.userPrincipalId is None:
                return False
            return bind

        binding = wait_for(binding_principal_validate)

        assert binding.roleTemplateId in test_roles
        assert binding.userId is not None
        user = client.by_id_user(binding.userId)
        assert binding.userPrincipalId in user.principalIds


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
    # Wait for role to get updated
    wait_for(lambda: client.by_id_role_template(locked_role)['locked'] is True,
             fail_handler=lambda: "Failed to lock role"+locked_role)

    project = client.create_project(name=random_str(), clusterId='local')
    remove_resource(project)

    wait_for_condition('InitialRolesPopulated', 'True', client, project)

    project = client.reload(project)

    data_dict = json.loads(project.annotations[
        CREATOR_ANNOTATION])

    assert len(project.projectRoleTemplateBindings()) == 2
    assert set(data_dict['required']) == set(test_roles)

    for binding in project.projectRoleTemplateBindings():
        assert binding.roleTemplateId in test_roles


@pytest.mark.nonparallel
def test_user_create_default_role(admin_mc, cleanup_roles, remove_resource):
    test_roles = ['user-base', 'settings-manage']
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
    assert len(user.globalRoleBindings()) == 2
    for binding in user.globalRoleBindings():
        assert binding.globalRoleId in test_roles


@pytest.mark.nonparallel
def test_default_system_project_role(admin_mc):
    test_roles = ['project-owner']
    client = admin_mc.client
    projects = client.list_project(clusterId="local")
    required_projects = {}
    required_projects["Default"] = defaultProjectLabel
    required_projects["System"] = systemProjectLabel
    created_projects = []

    for project in projects:
        name = project['name']
        if name == "Default" or name == "System":
            project = client.reload(project)

            projectLabel = required_projects[name]
            assert project['labels'].\
                data_dict()[projectLabel] == 'true'
            created_projects.append(project)

    assert len(required_projects) == len(created_projects)

    for project in created_projects:
        for binding in project.projectRoleTemplateBindings():
            assert binding.roleTemplateId in test_roles


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


def add_cr_if_not_exist(roles, name, rbac_api):
    hasRole = False
    for r in roles.items:
        if r.metadata.name == name:
            hasRole = True

    if not hasRole:
        body = kubernetes.client.V1ClusterRole()
        body.metadata = kubernetes.client.V1ObjectMeta(name=name)
        return rbac_api.create_cluster_role(body=body)
