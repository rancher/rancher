import pytest
from rancher import ApiError, Client

from .common import (
    ADMIN_TOKEN,
    CATTLE_API_URL,
    assign_members_to_cluster,
    assign_members_to_project,
    change_member_role_in_cluster,
    change_member_role_in_project,
    create_ns,
    create_project,
    create_project_and_ns,
    get_user_client,
    get_admin_client,
    get_client_for_token,
    get_cluster_client_for_token,
    create_user,
    get_user_client_and_cluster
)


def test_rbac_cluster_owner():
    client, cluster = get_user_client_and_cluster()
    user1, user1_token = create_user(get_admin_client())

    # Assert that user1 is not able to list cluster
    user1_client = get_client_for_token(user1_token)
    clusters = user1_client.list_cluster().data  # pylint: disable=no-member
    assert len(clusters) == 0

    # As admin , add user1 as cluster member of this cluster
    client = get_user_client()
    assign_members_to_cluster(client, user1, cluster, "cluster-owner")
    validate_cluster_owner(user1_token, cluster)


def test_rbac_cluster_member():
    client, cluster = get_user_client_and_cluster()
    user1, user1_token = create_user(get_admin_client())

    # Assert that user1 is not able to list cluster
    user1_client = get_client_for_token(user1_token)
    clusters = user1_client.list_cluster().data  # pylint: disable=no-member
    assert len(clusters) == 0

    # Add user1 as cluster member of this cluster
    client = get_user_client()
    assign_members_to_cluster(client, user1, cluster, "cluster-member")
    validate_cluster_member(user1_token, cluster)


def test_rbac_project_owner():
    client, cluster = get_user_client_and_cluster()
    #  As admin user create a project and namespace
    a_p, a_ns = create_project_and_ns(ADMIN_TOKEN, cluster)

    user1, user1_token = create_user(get_admin_client())

    # Assert that user1 is not able to list cluster
    user1_client = get_client_for_token(user1_token)
    clusters = user1_client.list_cluster().data  # pylint: disable=no-member
    assert len(clusters) == 0

    # As admin user, Add user1 as project member of this project
    client = get_user_client()
    assign_members_to_project(client, user1, a_p, "project-owner")
    validate_project_owner(user1_token, cluster, a_p, a_ns)


def test_rbac_project_member():
    client, cluster = get_user_client_and_cluster()

    #  As admin user create a project and namespace
    a_p, a_ns = create_project_and_ns(ADMIN_TOKEN, cluster)

    user1, user1_token = create_user(get_admin_client())
    user2, user2_token = create_user(get_admin_client())

    # Assert that user1 is not able to list cluster
    user1_client = get_client_for_token(user1_token)
    clusters = user1_client.list_cluster().data  # pylint: disable=no-member
    assert len(clusters) == 0

    # As admin user, Add user1 as project member of this project
    client = get_user_client()
    assign_members_to_project(client, user1, a_p, "project-member")
    validate_project_member(user1_token, cluster, a_p, a_ns)


def test_rbac_change_cluster_owner_to_cluster_member():
    client, cluster = get_user_client_and_cluster()

    user1, user1_token = create_user(get_admin_client())

    # Assert that user1 is not able to list cluster
    user1_client = get_client_for_token(user1_token)
    clusters = user1_client.list_cluster().data  # pylint: disable=no-member
    assert len(clusters) == 0

    # As admin , add user1 as cluster member of this cluster
    client = get_user_client()
    crtb = assign_members_to_cluster(
        client, user1, cluster, "cluster-owner")
    validate_cluster_owner(user1_token, cluster)
    change_member_role_in_cluster(
        client, user1, crtb, "cluster-member")
    validate_cluster_member(user1_token, cluster)


def test_rbac_change_cluster_member_to_cluster_owner():
    client, cluster = get_user_client_and_cluster()

    user1, user1_token = create_user(get_admin_client())

    # Assert that user1 is not able to list cluster
    user1_client = get_client_for_token(user1_token)
    clusters = user1_client.list_cluster().data  # pylint: disable=no-member
    assert len(clusters) == 0

    # Add user1 as cluster member of this cluster
    crtb = assign_members_to_cluster(
        get_user_client(), user1, cluster, "cluster-member")
    validate_cluster_member(user1_token, cluster)
    change_member_role_in_cluster(
        get_user_client(), user1, crtb, "cluster-owner")
    validate_cluster_owner(user1_token, cluster)


def test_rbac_change_project_owner_to_project_member():
    client, cluster = get_user_client_and_cluster()

    #  As admin user create a project and namespace
    a_p, a_ns = create_project_and_ns(ADMIN_TOKEN, cluster)

    user1, user1_token = create_user(get_admin_client())

    # Assert that user1 is not able to list cluster
    user1_client = get_client_for_token(user1_token)
    clusters = user1_client.list_cluster().data  # pylint: disable=no-member
    assert len(clusters) == 0

    # As admin user, Add user1 as project member of this project
    prtb = assign_members_to_project(
        get_user_client(), user1, a_p, "project-owner")
    validate_project_owner(user1_token, cluster, a_p, a_ns)
    change_member_role_in_project(
        get_user_client(), user1, prtb, "project-member")
    validate_project_member(user1_token, cluster, a_p, a_ns)


def test_rbac_change_project_member_to_project_cluster():
    client, cluster = get_user_client_and_cluster()

    #  As admin user create a project and namespace
    a_p, a_ns = create_project_and_ns(ADMIN_TOKEN, cluster)

    user1, user1_token = create_user(get_admin_client())
    user2, user2_token = create_user(get_admin_client())

    # Assert that user1 is not able to list cluster
    user1_client = get_client_for_token(user1_token)
    clusters = user1_client.list_cluster().data  # pylint: disable=no-member
    assert len(clusters) == 0

    # As admin user, Add user1 as project member of this project
    prtb = assign_members_to_project(
        get_user_client(), user1, a_p, "project-member")
    validate_project_member(user1_token, cluster, a_p, a_ns)
    change_member_role_in_project(
        get_user_client(), user1, prtb, "project-owner")
    validate_project_owner(user1_token, cluster, a_p, a_ns)


def validate_cluster_owner(user_token, cluster):
    #  As admin user create a project and namespace and a user
    user2, user2_token = create_user(get_admin_client())
    a_p, a_ns = create_project_and_ns(ADMIN_TOKEN, cluster)

    # Assert that user1 is able to see cluster
    user_client = get_client_for_token(user_token)
    clusters = user_client.list_cluster().data  # pylint: disable=no-member
    assert len(clusters) == 1

    # Assert that user1 is allowed to assign member to the cluster
    assign_members_to_cluster(user_client, user2, cluster, "cluster-member")

    # Assert that user1 is able to see projects he does not own
    project = user_client.list_project(  # pylint: disable=no-member
        name=a_p.name).data
    assert len(project) == 1

    # Assert that user1 is able to see namespaces that are in projects
    # that he does not own
    user_c_client = get_cluster_client_for_token(cluster, user_token)
    ns = user_c_client.list_namespace(  # pylint: disable=no-member
        uuid=a_ns.uuid).data
    assert len(ns) == 1

    # Assert that user1 is able to create namespaces in the projects
    # that he does not own
    create_ns(user_c_client, cluster, a_p)

    # Assert that user1 is able create projects and namespace in that project
    create_project_and_ns(user_token, cluster)


def validate_cluster_member(user_token, cluster):
    #  As admin user create a project and namespace and a user
    a_p, a_ns = create_project_and_ns(ADMIN_TOKEN, cluster)
    user2, user2_token = create_user(get_admin_client())

    # Assert that user1 is able to see cluster
    user_client = get_client_for_token(user_token)
    clusters = user_client.list_cluster(  # pylint: disable=no-member
        name=cluster.name).data
    assert len(clusters) == 1
    assert clusters[0].name == cluster.name

    # Assert that user1 is NOT able to assign member to the cluster
    with pytest.raises(ApiError) as e:
        assign_members_to_cluster(
            user_client, user2, cluster, "cluster-member")
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'

    # Assert that user1 is NOT able to see projects he does not own
    project = user_client.list_project(  # pylint: disable=no-member
        name=a_p.name).data
    assert len(project) == 0

    """
    # Assert that user1 is NOT able to access projects that he does not own
    with pytest.raises(ApiError) as e:
        get_project_client_for_token(a_p, user_token)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'
    """
    # Assert that user1 is able create projects and namespace in that project
    create_project_and_ns(user_token, cluster)


def validate_project_owner(user_token, cluster, project, namespace):
    user2, user2_token = create_user(get_admin_client())

    # Assert that user1 is now able to see cluster
    user_client = get_client_for_token(user_token)
    clusters = user_client.list_cluster(  # pylint: disable=no-member
        name=cluster.name).data
    assert len(clusters) == 1
    assert clusters[0].name == cluster.name

    # Assert that user1 is NOT able to assign member to the cluster
    with pytest.raises(ApiError) as e:
        assign_members_to_cluster(user_client, user2,
                                  cluster, "cluster-member")
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'

    # Assert that user1 is able to see projects he is made the owner of
    projects = user_client.list_project(  # pylint: disable=no-member
        name=project.name).data
    assert len(projects) == 1

    # Assert that user1 is able to access this project
    p_user_client = get_cluster_client_for_token(cluster, user_token)

    # Assert that user1 is able to see the existing namespace in this project
    nss = p_user_client.list_namespace(  # pylint: disable=no-member
        uuid=namespace.uuid).data
    assert len(nss) == 1

    # Assert that user1 is able to access this project
    create_ns(p_user_client, cluster, project)

    # Assert that user1 is able to assign member to the project
    assign_members_to_project(user_client, user2, project, "project-member")

    # Assert that user1 is NOT able to create project
    with pytest.raises(ApiError) as e:
        create_project(user_client, cluster)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'


def validate_project_member(user_token, cluster, project, namespace):
    user2, user2_token = create_user(get_admin_client())
    # Assert that user1 is able to see cluster
    user_client = Client(url=CATTLE_API_URL, token=user_token,
                         verify=False)
    clusters = user_client.list_cluster().data  # pylint: disable=no-member
    assert len(clusters) == 1
    assert clusters[0].name == cluster.name

    # Assert that user1 is NOT able to assign member to the cluster
    with pytest.raises(ApiError) as e:
        assign_members_to_cluster(user_client, user2,
                                  cluster, "cluster-member")
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'

    # Assert that user1 is able to see projects he is made member of
    projects = user_client.list_project(  # pylint: disable=no-member
        name=project.name).data
    assert len(projects) == 1

    # Assert that user1 is able to access this project
    p_user_client = get_cluster_client_for_token(cluster, user_token)

    # Assert that user1 is able to see the existing namespace in this project
    nss = p_user_client.list_namespace(  # pylint: disable=no-member
        uuid=namespace.uuid).data
    assert len(nss) == 1

    # Assert that user1 is able create namespace in this project
    create_ns(p_user_client, cluster, project)

    # Assert that user1 is NOT able to assign member to the project
    with pytest.raises(ApiError) as e:
        assign_members_to_project(user_client, user2,
                                  project, "project-member")
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'

    # Assert that user1 is NOT able to create project
    with pytest.raises(ApiError) as e:
        create_project(user_client, cluster)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'
