from .common import *  # NOQA
from rancher import ApiError
import pytest

# values used to create a catalog
BRANCH = "dev"
URL = "https://git.rancher.io/system-charts"


def test_global_role_create_1(remove_resource):
    """ test that admin can create a new global role, assign it
    to a standard user, then the user get the newly-assigned permission

    a global role for managing catalogs is used for this test.
    """

    # create a new global role that permits creating catalogs
    gr = validate_create_global_role(ADMIN_TOKEN, True, True)
    remove_resource(gr)
    # create a new user
    client = get_admin_client()
    user, token = create_user(client)
    remove_resource(user)
    # check that the user can NOT create catalogs
    name = random_name()
    validate_create_catalog(token,
                            catalog_name=name,
                            branch=BRANCH,
                            url=URL,
                            permission=False)
    client.create_global_role_binding(globalRoleId=gr.id, userId=user.id)
    # check that the user has the global role
    target_grb = client.list_global_role_binding(userId=user.id,
                                                 globalRoleId=gr.id).data
    assert len(target_grb) == 1
    # the user can create catalogs
    obj = validate_create_catalog(token,
                                  catalog_name=name,
                                  branch=BRANCH,
                                  url=URL,
                                  permission=True)
    remove_resource(obj)


def test_global_role_create_2(remove_resource):
    """ test that admin can create a new global role, assign it
    to a standard user, then the user get the newly-assigned permission

    a global role for listing clusters is used for this test.
    """

    # create a new global role that permits listing clusters
    gr = validate_create_global_role(ADMIN_TOKEN, True, True,
                                     TEMPLATE_LIST_CLUSTER)
    remove_resource(gr)
    # create a new user
    client = get_admin_client()
    user, token = create_user(client)
    remove_resource(user)
    # check that the new user can not list cluster
    user_client = get_client_for_token(token)
    data = user_client.list_cluster().data
    assert len(data) == 0, "the user should not be able to list any cluster"
    client.create_global_role_binding(globalRoleId=gr.id, userId=user.id)
    # check that the user has the global role
    target_grb = client.list_global_role_binding(userId=user.id,
                                                 globalRoleId=gr.id).data
    assert len(target_grb) == 1
    # check that the new user can list cluster
    data = user_client.list_cluster().data
    assert len(data) > 0


def test_global_role_edit(remove_resource):
    """ test that admin can edit a global role, and permissions of user who
    binds to this role reflect the change"""

    gr = validate_create_global_role(ADMIN_TOKEN, True, True)
    remove_resource(gr)
    client = get_admin_client()
    user, user_token = create_user(client)
    remove_resource(user)
    # check that the user can NOT create catalogs
    name = random_name()
    validate_create_catalog(user_token,
                            catalog_name=name,
                            branch=BRANCH,
                            url=URL,
                            permission=False)
    client.create_global_role_binding(globalRoleId=gr.id, userId=user.id)
    # now he can create catalogs
    catalog = validate_create_catalog(user_token,
                                      catalog_name=name,
                                      branch=BRANCH,
                                      url=URL,
                                      permission=True)
    remove_resource(catalog)
    # edit the global role
    validate_edit_global_role(ADMIN_TOKEN, gr, True)
    # the user can not create new catalog
    validate_create_catalog(user_token,
                            catalog_name=name,
                            branch=BRANCH,
                            url=URL,
                            permission=False)


def test_global_role_delete(remove_resource):
    """ test that admin can edit a global role, and permissions of user who
    binds to this role reflect the change"""

    gr = validate_create_global_role(ADMIN_TOKEN, True, True)
    remove_resource(gr)
    client = get_admin_client()
    user, token = create_user(client)
    remove_resource(user)
    client.create_global_role_binding(globalRoleId=gr.id, userId=user.id)
    name = random_name()
    catalog = validate_create_catalog(token,
                                      catalog_name=name,
                                      branch=BRANCH,
                                      url=URL,
                                      permission=True)
    remove_resource(catalog)
    validate_delete_global_role(ADMIN_TOKEN, gr, True)
    # the user can not create new catalog
    validate_create_catalog(token,
                            catalog_name=name,
                            branch=BRANCH,
                            url=URL,
                            permission=False)


def test_builtin_global_role():
    # builtin global role can not be deleted
    client = get_admin_client()
    gr_list = client.list_global_role(name="Manage Users").data
    assert len(gr_list) == 1
    gr = gr_list[0]
    try:
        client.delete(gr)
    except ApiError as e:
        assert e.error.status == 403
        assert e.error.code == 'PermissionDenied'
    # builtin global role can be set as new user default
    gr = client.update(gr, {"newUserDefault": "true"})
    gr = client.reload(gr)
    assert gr.newUserDefault is True
    gr = client.update(gr, {"newUserDefault": "false"})
    gr = client.reload(gr)
    assert gr.newUserDefault is False


def validate_list_global_role(token, permission=False):
    client = get_client_for_token(token)
    res = client.list_global_role().data
    if not permission:
        assert len(res) == 0
    else:
        assert len(res) > 0


@if_test_rbac
def test_rbac_global_role_list_cluster_owner():
    # cluster owner can not list global roles
    token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    validate_list_global_role(token, False)


@if_test_rbac
def test_rbac_global_role_list_cluster_member():
    # cluster member can not list global roles
    token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    validate_list_global_role(token, False)


@if_test_rbac
def test_rbac_global_role_list_project_owner():
    # project owner can not list global roles
    token = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_list_global_role(token, False)


@if_test_rbac
def test_rbac_global_role_list_project_member():
    # project member can not list global roles
    token = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_list_global_role(token, False)


@if_test_rbac
def test_rbac_global_role_list_project_read_only():
    # project read-only can not list global roles
    token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    validate_list_global_role(token, False)


@if_test_rbac
def test_rbac_global_role_create_cluster_owner():
    # cluster owner can not create global roles
    token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    validate_create_global_role(token, permission=False)


@if_test_rbac
def test_rbac_global_role_create_cluster_member():
    # cluster member can not create global roles
    token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    validate_create_global_role(token, permission=False)


@if_test_rbac
def test_rbac_global_role_create_project_owner():
    # project owner can not create global roles
    token = rbac_get_user_token_by_role(PROJECT_OWNER)
    validate_create_global_role(token, permission=False)


@if_test_rbac
def test_rbac_global_role_create_project_member():
    # project member can not create global roles
    token = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_create_global_role(token, permission=False)


@if_test_rbac
def test_rbac_global_role_create_project_read_only():
    # project read-only can not create global roles
    token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    validate_create_global_role(token, permission=False)


@if_test_rbac
def test_rbac_global_role_delete_cluster_owner(remove_resource):
    # cluster owner can not delete global roles
    gr = create_gr()
    remove_resource(gr)
    token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    validate_delete_global_role(token, gr, False)


@if_test_rbac
def test_rbac_global_role_delete_cluster_member(remove_resource):
    # cluster member can not delete global roles
    gr = create_gr()
    remove_resource(gr)
    token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    validate_delete_global_role(token, gr, False)


@if_test_rbac
def test_rbac_global_role_delete_project_owner(remove_resource):
    # project owner can not delete global roles
    gr = create_gr()
    remove_resource(gr)
    token = rbac_get_user_token_by_role(PROJECT_OWNER)
    validate_delete_global_role(token, gr, False)


@if_test_rbac
def test_rbac_global_role_delete_project_member(remove_resource):
    # project member can not delete global roles
    gr = create_gr()
    remove_resource(gr)
    token = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_delete_global_role(token, gr, False)


@if_test_rbac
def test_rbac_global_role_delete_project_read_only(remove_resource):
    # project read-only can not delete global roles
    gr = create_gr()
    remove_resource(gr)
    token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    validate_delete_global_role(token, gr, False)


@if_test_rbac
def test_rbac_global_role_edit_cluster_owner(remove_resource):
    # cluster owner can not edit global roles
    gr = create_gr()
    remove_resource(gr)
    token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    validate_edit_global_role(token, gr, False)


@if_test_rbac
def test_rbac_global_role_edit_cluster_member(remove_resource):
    # cluster member can not edit global roles
    gr = create_gr()
    remove_resource(gr)
    token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    validate_edit_global_role(token, gr, False)


@if_test_rbac
def test_rbac_global_role_edit_project_owner(remove_resource):
    # project owner can not edit global roles
    gr = create_gr()
    remove_resource(gr)
    token = rbac_get_user_token_by_role(PROJECT_OWNER)
    validate_edit_global_role(token, gr, False)


@if_test_rbac
def test_rbac_global_role_edit_project_member(remove_resource):
    # project member can not edit global roles
    gr = create_gr()
    remove_resource(gr)
    token = rbac_get_user_token_by_role(PROJECT_MEMBER)
    validate_edit_global_role(token, gr, False)


@if_test_rbac
def test_rbac_global_role_edit_project_read_only(remove_resource):
    # project read-only can not edit global roles
    gr = create_gr()
    remove_resource(gr)
    token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    validate_edit_global_role(token, gr, False)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_global_admin_client_and_cluster()
    create_kubeconfig(cluster)


def create_gr():
    return validate_create_global_role(ADMIN_TOKEN, False, True)


def validate_create_global_role(token, new_user_default=False,
                                permission=False, template=None):
    """ validate if the user has the permission to create global role."""

    if template is None:
        template = TEMPLATE_MANAGE_CATALOG
    client = get_client_for_token(token)
    t_name = random_name()
    template = generate_template_global_role(t_name, new_user_default,
                                             template)
    # catch the expected error if the user has no permission to create
    if not permission:
        with pytest.raises(ApiError) as e:
            client.create_global_role(template)
        assert e.value.error.status == 403 and \
            e.value.error.code == 'Forbidden', \
            "user with no permission should receive 403: Forbidden"
        return None
    else:
        try:
            client.create_global_role(template)
        except ApiError as e:
            assert False, "user with permission should receive no exception:" \
                          + e.error.status + " " + e.error.code

    # check that the global role is created
    gr_list = client.list_global_role(name=t_name).data
    assert len(gr_list) == 1
    gr = gr_list[0]
    # check that the global role is set to be the default
    assert gr.newUserDefault == new_user_default
    return gr


def validate_delete_global_role(token, global_role, permission=False):
    """ validate if the user has the permission to delete global role."""

    client = get_client_for_token(token)
    # catch the expected error if the user has no permission to delete
    if not permission:
        with pytest.raises(ApiError) as e:
            client.delete(global_role)
        assert e.value.error.status == 403 and \
            e.value.error.code == 'Forbidden', \
            "user with no permission should receive 403: Forbidden"
        return
    else:
        try:
            client.delete(global_role)
        except ApiError as e:
            assert False, "user with permission should receive no exception:" \
                          + e.error.status + " " + e.error.code
    # check that the global role is removed
    client = get_client_for_token(ADMIN_TOKEN)
    assert client.reload(global_role) is None


def validate_edit_global_role(token, global_role, permission=False):
    """ for the testing purpose, this method removes all permissions in
    the global role and unset it as new user default."""

    client = get_client_for_token(token)
    # edit the global role
    template = deepcopy(TEMPLATE_MANAGE_CATALOG)
    template["newUserDefault"] = "false"
    template["rules"] = []
    # catch the expected error if the user has no permission to edit
    if not permission:
        with pytest.raises(ApiError) as e:
            client.update(global_role, template)
        assert e.value.error.status == 403 and \
            e.value.error.code == 'Forbidden', \
            "user with no permission should receive 403: Forbidden"
        return None
    else:
        try:
            client.update(global_role, template)
        except ApiError as e:
            assert False, "user with permission should receive no exception:" \
                          + e.error.status + " " + e.error.code

    # check that the global role is not the new user default
    gr = client.reload(global_role)
    assert gr.newUserDefault is False
    # check that there is no rule left
    assert gr.rules is None
    return gr
