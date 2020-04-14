from .test_auth import enable_ad, load_setup_data
from .common import *  # NOQA
from rancher import ApiError
import pytest
import requests

'''
Prerequisite:
    - Enable Auth
    - Optional: searching nested group enabled
    - All users used for testing global group role binding should not be used
      to create the cluster when preparing the Rancher setup
'''

# values used to create a catalog
BRANCH = "dev"
URL = "https://git.rancher.io/system-charts"
# the link to search principals in the auth provider

AUTH_ADMIN_USER = load_setup_data()["admin_user"]


@if_test_group_rbac
def test_ggrb_1(remove_resource):
    """ test that when a global role is assigned to a group,
    all users in the group will get the permission;
    when the ggrb is removed, all users in the group will lose
    the permission.

    the default global role "catalogs-manage" is used
    """

    target_group_name = get_group(NESTED_GROUP_ENABLED)
    users = get_user_by_group(target_group_name, NESTED_GROUP_ENABLED)
    # check that users can not create catalogs
    for user in users:
        validate_permission_create_catalog(user, False)
    auth_admin = login_as_auth_user(AUTH_ADMIN_USER,
                                    AUTH_USER_PASSWORD)
    g_id = get_group_principal_id(target_group_name, token=auth_admin['token'])
    ggrb = get_admin_client().create_global_role_binding(
        globalRoleId="catalogs-manage", groupPrincipalId=g_id)
    # check that users can create catalogs now
    for user in users:
        catalog = validate_permission_create_catalog(user, True)
        remove_resource(catalog)
    # delete the ggrb
    get_admin_client().delete(ggrb)
    # check that users can not create catalogs
    for user in users:
        validate_permission_create_catalog(user, False)


@if_test_group_rbac
def test_ggrb_2(remove_resource):
    """ test that after editing the global role, users'
    permissions reflect the changes

    Steps:
    - users in group1 cannot list clusters or create catalogs
    - create a custom global role gr1 that permits creating catalogs
    - create the global group role binding ggrb1 to bind gr1 to group1
    - users in group1 can create catalogs now
    - edit gr1 to remove the permission of creating catalogs, add the
    permission of listing clusters
    - users in group1 cannot create catalogs, but can list clusters
    """

    target_group_name = get_group(NESTED_GROUP_ENABLED)
    users = get_user_by_group(target_group_name, NESTED_GROUP_ENABLED)
    # check that users can not create catalogs or list clusters
    for user in users:
        validate_permission_create_catalog(user, False)
        validate_permission_list_cluster(user, 0)
    # create a custom global role that permits create catalogs
    admin_c = get_admin_client()
    template = generate_template_global_role(name=random_name(),
                                             template=TEMPLATE_MANAGE_CATALOG)
    gr = admin_c.create_global_role(template)
    remove_resource(gr)
    auth_admin = login_as_auth_user(AUTH_ADMIN_USER,
                                    AUTH_USER_PASSWORD)
    g_id = get_group_principal_id(target_group_name, token=auth_admin['token'])
    ggrb = get_admin_client().create_global_role_binding(
        globalRoleId=gr["id"], groupPrincipalId=g_id)
    # check that users can create catalogs now, but not list clusters
    for user in users:
        validate_permission_list_cluster(user, 0)
        catalog = validate_permission_create_catalog(user, True)
        remove_resource(catalog)
    # edit the global role
    rules = [
        {
            "type": "/v3/schemas/policyRule",
            "apiGroups": [
                "management.cattle.io"
            ],
            "verbs": [
                "get",
                "list",
                "watch"
            ],
            "resources": [
                "clusters"
            ]
        }
    ]
    admin_c.update(gr, rules=rules)
    target_num = len(admin_c.list_cluster().data)
    # check that users can list clusters, but not create catalogs
    for user in users:
        validate_permission_create_catalog(user, False)
        validate_permission_list_cluster(user, target_num)
    # delete the ggrb
    admin_c.delete(ggrb)
    for user in users:
        validate_permission_list_cluster(user, 0)


@if_test_group_rbac
def test_ggrb_3(remove_resource):
    """ test that when a global role is assigned to a group,
    all users in the group get the permission from the role,
    users not in the group do not get the permission

    Steps:
    - users in the group cannot list clusters
    - users not in the group cannot list clusters
    - create a custom global role gr1 that permits listing clusters
    - create the global group role binding ggrb1 to bind gr1 to the group
    - users in the group can list clusters now
    - users not in group still cannot list clusters
    - delete the ggrb1
    - users in the group can not list clusters
    """

    target_g, user1 = get_a_group_and_a_user_not_in_it(NESTED_GROUP_ENABLED)
    users = get_user_by_group(target_g, NESTED_GROUP_ENABLED)
    # check that users in the group can not list clusters
    for user in users:
        validate_permission_list_cluster(user, 0)
    # check that user not in the group can not list clusters
    validate_permission_list_cluster(user1, 0)

    auth_admin = login_as_auth_user(AUTH_ADMIN_USER,
                                    AUTH_USER_PASSWORD)
    g_id = get_group_principal_id(target_g, token=auth_admin['token'])
    # create a custom global role that permits listing clusters
    admin_c = get_admin_client()
    template = generate_template_global_role(name=random_name(),
                                             template=TEMPLATE_LIST_CLUSTER)
    gr = admin_c.create_global_role(template)
    remove_resource(gr)
    ggrb = admin_c.create_global_role_binding(globalRoleId=gr["id"],
                                              groupPrincipalId=g_id)
    remove_resource(ggrb)
    target_num = len(admin_c.list_cluster().data)
    # check that users in the group can list clusters
    for user in users:
        validate_permission_list_cluster(user, target_num)
    # check that user not in the group can not list clusters
    validate_permission_list_cluster(user1, 0)
    # delete the ggrb
    admin_c.delete(ggrb)
    # check that users in the group can not list clusters
    for user in users:
        validate_permission_list_cluster(user, 0)


# cluster owner, cluster member, project owner, project member
# and project read-only can not list ggrb
rbac_list_ggrb = [
    (CLUSTER_OWNER, 0),
    (CLUSTER_MEMBER, 0),
    (PROJECT_OWNER, 0),
    (PROJECT_MEMBER, 0),
    (PROJECT_READ_ONLY, 0),
]

# cluster owner, cluster member, project owner, project member
# and project read-only can not create or delete ggrb
rbac_create_delete_ggrb = [
    (CLUSTER_OWNER, False),
    (CLUSTER_MEMBER, False),
    (PROJECT_OWNER, False),
    (PROJECT_MEMBER, False),
    (PROJECT_READ_ONLY, False),
]


@if_test_rbac
@pytest.mark.parametrize(["role", "expected_count"], rbac_list_ggrb)
def test_rbac_ggrb_list(role, expected_count):
    token = rbac_get_user_token_by_role(role)
    validate_permission_list_ggrb(token, expected_count)


@if_test_rbac
@pytest.mark.parametrize(["role", "permission"], rbac_create_delete_ggrb)
def test_rbac_ggrb_create(role, permission):
    token = rbac_get_user_token_by_role(role)
    validate_permission_create_ggrb(token, permission)


@if_test_rbac
@pytest.mark.parametrize(["role", "permission"], rbac_create_delete_ggrb)
def test_rbac_ggrb_delete(role, permission):
    token = rbac_get_user_token_by_role(role)
    validate_permission_delete_ggrb(token, permission)


def validate_permission_list_cluster(username, num=0):
    """ check if the user from auth provider has the permission to
    list clusters

    :param username: username from the auth provider
    :param num: expected number of clusters
    """
    token = login_as_auth_user(username, AUTH_USER_PASSWORD)['token']
    user_client = get_client_for_token(token)
    clusters = user_client.list_cluster().data
    assert len(clusters) == num


def validate_permission_create_catalog(username, permission=False):
    """ check if the user from auth provider has the permission to
    create new catalog
    """
    name = random_name()
    token = login_as_auth_user(username, AUTH_USER_PASSWORD)['token']
    return validate_create_catalog(token, catalog_name=name, branch=BRANCH,
                                   url=URL, permission=permission)


def validate_permission_list_ggrb(token, num=0):
    """ check if the user from auth provider has the permission to
    list global role bindings
    """
    user_client = get_client_for_token(token)
    clusters = user_client.list_global_role_binding().data
    assert len(clusters) == num


def validate_permission_create_ggrb(token, permission=False):
    """ check if the user from auth provider has the permission to
    create group global role bindings
    """
    target_group_name = get_group()
    auth_admin = login_as_auth_user(AUTH_ADMIN_USER,
                                    AUTH_USER_PASSWORD)
    g_id = get_group_principal_id(target_group_name, token=auth_admin['token'])
    role = generate_a_global_role()
    client = get_client_for_token(token)
    if not permission:
        with pytest.raises(ApiError) as e:
            client.create_global_role_binding(globalRoleId=role["id"],
                                              groupPrincipalId=g_id)
        assert e.value.error.status == 403 and \
            e.value.error.code == 'Forbidden', \
            "user with no permission should receive 403: Forbidden"
        return None
    else:
        try:
            rtn = \
                client.create_global_role_binding(globalRoleId=role["id"],
                                                  groupPrincipalId=g_id)
            return rtn
        except ApiError as e:
            assert False, "user with permission should receive no exception:" \
                          + str(e.error.status) + " " + e.error.code


def validate_permission_delete_ggrb(token, permission=False):
    """ check if the user from auth provider has the permission to
    deleting group global role bindings
    """
    ggrb = validate_permission_create_ggrb(ADMIN_TOKEN, True)
    client = get_client_for_token(token)
    if not permission:
        with pytest.raises(ApiError) as e:
            client.delete(ggrb)
        assert e.value.error.status == 403 and \
            e.value.error.code == 'Forbidden', \
            "user with no permission should receive 403: Forbidden"
        get_admin_client().delete(ggrb)
    else:
        try:
            client.delete(ggrb)
        except ApiError as e:
            get_admin_client().delete(ggrb)
            assert False, "user with permission should receive no exception:" \
                          + str(e.error.status) + " " + e.error.code


def generate_a_global_role():
    """ return a global role with the permission of listing cluster"""
    admin_c = get_admin_client()
    template = generate_template_global_role(name=random_name(),
                                             template=TEMPLATE_LIST_CLUSTER)
    return admin_c.create_global_role(template)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)

    admin_client = get_admin_client()
    ad_enabled = admin_client.by_id_auth_config("activedirectory").enabled
    if AUTH_PROVIDER == "activeDirectory" and not ad_enabled:
        enable_ad(AUTH_ADMIN_USER, ADMIN_TOKEN,
                  password=AUTH_USER_PASSWORD, nested=NESTED_GROUP_ENABLED)

    if NESTED_GROUP_ENABLED:
        assert is_nested(), "no nested group is found"
