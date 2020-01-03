from .common import *  # NOQA
import pytest
from .test_rbac import create_user

project_detail = {"cluster1": {"project1": None, "namespace1": None,
                               "project2": None, "namespace2": None,
                               "cluster": None},
                  "cluster2": {"project1": None, "namespace1": None,
                               "cluster": None}}
user_token = {"user_c1_p1_owner": {"user": None, "token": None},
              "user_c1_p1_member": {"user": None, "token": None},
              "user_c1_p2_owner": {"user": None, "token": None},
              "user_c2_p1_owner": {"user": None, "token": None},
              "user_c1_owner": {"user": None, "token": None},
              "user_c1_member": {"user": None, "token": None},
              "user_c2_owner": {"user": None, "token": None},
              "user_standard": {"user": None, "token": None}}
CATALOG_NAME = random_test_name("test-catalog")
PROJECT_CATALOG = random_test_name("test-pj")
CLUSTER_CATALOG = random_test_name("test-cl")
CATALOG_URL = "https://github.com/rancher/integration-test-charts.git"
BRANCH = "validation-tests"
MYSQL_EXTERNALID_131 = create_catalog_external_id(CATALOG_NAME,
                                                  "mysql", "1.3.1")
MYSQL_EXTERNALID_132 = create_catalog_external_id(CATALOG_NAME,
                                                  "mysql",
                                                  "1.3.2")
WORDPRESS_EXTID = create_catalog_external_id(CATALOG_NAME,
                                             "wordpress",
                                             "7.3.8")


def cluster_and_client(cluster_id, mgmt_client):
    cluster = mgmt_client.by_id_cluster(cluster_id)
    url = cluster.links.self + '/schemas'
    client = rancher.Client(url=url,
                            verify=False,
                            token=mgmt_client.token)
    return cluster, client


def wait_for_template_to_be_created(client, name, timeout=45):
    found = False
    start = time.time()
    interval = 0.5
    while not found:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for templates")
        templates = client.list_template(catalogId=name)
        if len(templates) > 0:
            found = True
        time.sleep(interval)
        interval *= 2


def check_condition(condition_type, status):
    def _find_condition(resource):
        if not hasattr(resource, "conditions"):
            return False

        if resource.conditions is None:
            return False

        for condition in resource.conditions:
            if condition.type == condition_type and condition.status == status:
                return True
        return False

    return _find_condition


@if_test_rbac
def test_tiller():
    name = random_test_name()
    admin_client = get_user_client()

    clusters = admin_client.list_cluster(name=CLUSTER_NAME).data
    assert len(clusters) > 0
    cluster_id = clusters[0].id

    p = admin_client. \
        create_project(name="test-" + random_str(),
                       clusterId=cluster_id,
                       resourceQuota={
                           "limit": {
                               "secrets": "1"}},
                       namespaceDefaultResourceQuota={
                           "limit": {
                               "secrets": "1"}}
                       )

    p = admin_client.reload(p)
    proj_client = rancher.Client(url=p.links.self +
                                 '/schemas', verify=False,
                                 token=USER_TOKEN)
    # need a cluster scoped client to create a namespace
    _cluster, cluster_client = cluster_and_client(cluster_id, admin_client)
    ns = cluster_client.create_namespace(name=random_str(),
                                         projectId=p.id,
                                         resourceQuota={
                                             "limit": {
                                                 "secrets": "1"
                                             }}
                                         )
    wait_for_template_to_be_created(admin_client, "library")
    app = proj_client.create_app(
        name=name,
        externalId=WORDPRESS_EXTID,
        targetNamespace=ns.name,
        projectId=p.id,
        answers=get_defaut_question_answers(admin_client, WORDPRESS_EXTID)
    )

    app = proj_client.reload(app)
    # test for tiller to be stuck on bad installs
    wait_for_condition(proj_client, app, check_condition('Installed', 'False'))
    # cleanup by deleting project
    admin_client.delete(p)


@if_test_rbac
def test_app_deploy():
    admin_client = get_user_client()
    proj_client = get_project_client_for_token(
        project_detail["cluster1"]["project1"],
        USER_TOKEN)
    answer = get_defaut_question_answers(
            admin_client,
            MYSQL_EXTERNALID_131)
    wait_for_template_to_be_created(admin_client, "library")
    app = proj_client.create_app(
        name=random_test_name(),
        externalId=MYSQL_EXTERNALID_131,
        targetNamespace=project_detail["cluster1"]["namespace1"].name,
        projectId=project_detail["cluster1"]["project1"].id,
        answers=answer)
    print("App is active")
    validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_131)
    proj_client.delete(app)


@if_test_rbac
def test_app_delete():
    admin_client = get_user_client()
    proj_client = get_project_client_for_token(
        project_detail["cluster1"]["project1"],
        USER_TOKEN)
    wait_for_template_to_be_created(admin_client, "library")
    answer = get_defaut_question_answers(
            admin_client,
            MYSQL_EXTERNALID_131)
    app = proj_client.create_app(
        name=random_test_name(),
        externalId=MYSQL_EXTERNALID_131,
        targetNamespace=project_detail["cluster1"]["namespace1"].name,
        projectId=project_detail["cluster1"]["project1"].id,
        answers=answer)
    print("App is active")
    validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_131)
    app = proj_client.delete(app)
    validate_app_deletion(proj_client, app.id)


@if_test_rbac
def test_app_upgrade_version():
    admin_client = get_user_client()
    proj_client = get_project_client_for_token(
        project_detail["cluster1"]["project1"],
        USER_TOKEN)
    wait_for_template_to_be_created(admin_client, "library")
    answer = get_defaut_question_answers(
            admin_client,
            MYSQL_EXTERNALID_131)
    app = proj_client.create_app(
        name=random_test_name(),
        externalId=MYSQL_EXTERNALID_131,
        targetNamespace=project_detail["cluster1"]["namespace1"].name,
        projectId=project_detail["cluster1"]["project1"].id,
        answers=answer)
    print("App is active")
    validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_131)
    new_answer = get_defaut_question_answers(
            admin_client,
            MYSQL_EXTERNALID_132)
    app = proj_client.update(
        obj=app,
        externalId=MYSQL_EXTERNALID_132,
        targetNamespace=project_detail["cluster1"]["namespace1"].name,
        projectId=project_detail["cluster1"]["project1"].id,
        answers=new_answer)
    app = proj_client.reload(app)
    validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_132)
    assert app.externalId == MYSQL_EXTERNALID_132, "incorrect template version"
    proj_client.delete(app)


@if_test_rbac
def test_app_rollback():
    admin_client = get_user_client()
    proj_client = get_project_client_for_token(
        project_detail["cluster1"]["project1"],
        USER_TOKEN)
    wait_for_template_to_be_created(admin_client, "library")
    answer = get_defaut_question_answers(
            admin_client,
            MYSQL_EXTERNALID_131)
    app = proj_client.create_app(
        name=random_test_name(),
        externalId=MYSQL_EXTERNALID_131,
        targetNamespace=project_detail["cluster1"]["namespace1"].name,
        projectId=project_detail["cluster1"]["project1"].id,
        answers=answer)
    print("App is active")
    app = validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_131)
    rev_id = app.appRevisionId
    new_answer = get_defaut_question_answers(
            admin_client,
            MYSQL_EXTERNALID_132)
    app = proj_client.update(
        obj=app,
        externalId=MYSQL_EXTERNALID_132,
        targetNamespace=project_detail["cluster1"]["namespace1"].name,
        projectId=project_detail["cluster1"]["project1"].id,
        answers=new_answer)
    app = proj_client.reload(app)
    app = validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_132)
    assert app.externalId == MYSQL_EXTERNALID_132, "incorrect template version"
    proj_client.action(obj=app,
                       action_name='rollback',
                       revisionId=rev_id)
    app = proj_client.reload(app)
    validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_131)
    assert app.externalId == MYSQL_EXTERNALID_131, "incorrect template version"
    proj_client.delete(app)


@if_test_rbac
def test_app_answer_override():
    admin_client = get_user_client()
    proj_client = get_project_client_for_token(
        project_detail["cluster1"]["project1"],
        USER_TOKEN)
    wait_for_template_to_be_created(admin_client, "library")
    answers = get_defaut_question_answers(
        admin_client,
        MYSQL_EXTERNALID_131)
    app = proj_client.create_app(
        name=random_test_name(),
        externalId=MYSQL_EXTERNALID_131,
        targetNamespace=project_detail["cluster1"]["namespace1"].name,
        projectId=project_detail["cluster1"]["project1"].id,
        answers=answers)
    print("App is active")
    app = validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_131)
    answers["mysqlUser"] = "admin1234"
    app = proj_client.update(
        obj=app,
        externalId=MYSQL_EXTERNALID_131,
        targetNamespace=project_detail["cluster1"]["namespace1"].name,
        projectId=project_detail["cluster1"]["project1"].id,
        answers=answers)
    app = proj_client.reload(app)
    app = validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_131, answers)
    assert app["answers"].mysqlUser == "admin1234", \
        "incorrect answer upgrade"
    proj_client.delete(app)


@if_test_rbac
def test_rbac_app_project_catalog_list_1():
    catalog, project_catalog_external_id = create_project_catalog()
    # Verify user_c1_p1_owner CAN list the added catalog
    validate_user_list_catalog("user_c1_p1_owner", clustercatalog=False)
    # deploy an app
    proj_client_user = get_project_client_for_token(
        project_detail["cluster1"]["project1"],
        user_token["user_c1_p1_owner"]["token"])
    validate_catalog_app_deploy(proj_client_user,
                                project_detail["cluster1"]["namespace1"].name,
                                project_detail["cluster1"]["project1"].id,
                                project_catalog_external_id)


@if_test_rbac
def test_rbac_app_project_catalog_list_2():
    catalog, project_catalog_external_id = create_project_catalog()
    # Verify user_c1_p1_member CAN list the added catalog
    validate_user_list_catalog("user_c1_p1_member", clustercatalog=False)

    proj_client_user2 = get_project_client_for_token(
        project_detail["cluster1"]["project1"],
        user_token["user_c1_p1_member"]["token"])
    validate_catalog_app_deploy(proj_client_user2,
                                project_detail["cluster1"]["namespace1"].name,
                                project_detail["cluster1"]["project1"].id,
                                project_catalog_external_id)


@if_test_rbac
def test_rbac_app_project_catalog_list_3():
    catalog, project_catalog_external_id = create_project_catalog()
    # Verify user_c1_p2_owner CANNOT list the added catalog
    validate_user_list_catalog("user_c1_p2_owner", False, False)


@if_test_rbac
def test_rbac_app_project_catalog_list_4():
    catalog, project_catalog_external_id = create_project_catalog()
    # Verify user_standard CANNOT list the added catalog
    validate_user_list_catalog("user_standard", False, False)


@if_test_rbac
def test_rbac_app_project_catalog_list_5():
    catalog, project_catalog_external_id = create_project_catalog()
    # Verify user_c2_p1_owner CANNOT list the added catalog
    validate_user_list_catalog("user_c2_p1_owner", False, False)


@if_test_rbac
def test_rbac_app_project_catalog_list_6():
    catalog, project_catalog_external_id = create_project_catalog()
    # Verify user_c1_owner CAN list the added catalog
    validate_user_list_catalog("user_c1_owner", clustercatalog=False)
    proj_client_user3 = get_project_client_for_token(
        project_detail["cluster1"]["project1"],
        user_token["user_c1_owner"]["token"])
    validate_catalog_app_deploy(proj_client_user3,
                                project_detail["cluster1"]["namespace1"].name,
                                project_detail["cluster1"]["project1"].id,
                                project_catalog_external_id)


@if_test_rbac
def test_rbac_app_project_catalog_list_7():
    catalog, project_catalog_external_id = create_project_catalog()
    # Verify user_c1_member CANNOT list the added catalog
    validate_user_list_catalog("user_c1_member", False, False)



@if_test_rbac
def test_rbac_app_project_catalog_list_8():
    catalog, project_catalog_external_id = create_project_catalog()
    # Verify user_c2_owner CANNOT list the added catalog
    validate_user_list_catalog("user_c2_owner", False, False)


@if_test_rbac
def test_rbac_app_cluster_catalog_list_1():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # verify user_c1_p1_owner CAN list the catalog
    validate_user_list_catalog("user_c1_p1_owner")

    proj_client_user = get_project_client_for_token(
        project_detail["cluster1"]["project1"],
        user_token["user_c1_p1_owner"]["token"])
    validate_catalog_app_deploy(proj_client_user,
                                project_detail["cluster1"]["namespace1"].name,
                                project_detail["cluster1"]["project1"].id,
                                cluster_catalog_external_id)


@if_test_rbac
def test_rbac_app_cluster_catalog_list_2():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # verify user_c1_p1_member CAN list the catalog
    validate_user_list_catalog("user_c1_p1_member")
    proj_client_user = get_project_client_for_token(
        project_detail["cluster1"]["project1"],
        user_token["user_c1_p1_member"]["token"])
    validate_catalog_app_deploy(proj_client_user,
                                project_detail["cluster1"]["namespace1"].name,
                                project_detail["cluster1"]["project1"].id,
                                cluster_catalog_external_id)


@if_test_rbac
def test_rbac_app_cluster_catalog_list_3():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # verify user_c1_p2_owner CAN list the catalog
    validate_user_list_catalog("user_c1_p2_owner")
    proj_client_user = get_project_client_for_token(
        project_detail["cluster1"]["project2"],
        user_token["user_c1_p2_owner"]["token"])
    validate_catalog_app_deploy(proj_client_user,
                                project_detail["cluster1"]["namespace2"].name,
                                project_detail["cluster1"]["project2"].id,
                                cluster_catalog_external_id)


@if_test_rbac
def test_rbac_app_cluster_catalog_list_4():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # verify user_c2_p1_owner CANNOT list the catalog
    validate_user_list_catalog("user_c2_p1_owner", False)


@if_test_rbac
def test_rbac_app_cluster_catalog_list_5():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # verify user_standard CANNOT list the catalog
    validate_user_list_catalog("user_standard", False)


@if_test_rbac
def test_rbac_app_cluster_catalog_list_6():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # user_c1_owner CAN list the catalog
    validate_user_list_catalog("user_c1_owner")
    proj_client_user = get_project_client_for_token(
        project_detail["cluster1"]["project1"],
        user_token["user_c1_owner"]["token"])
    validate_catalog_app_deploy(proj_client_user,
                                project_detail["cluster1"]["namespace1"].name,
                                project_detail["cluster1"]["project1"].id,
                                cluster_catalog_external_id)


@if_test_rbac
def test_rbac_app_cluster_catalog_list_7():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # user_c1_member CAN list the catalog
    validate_user_list_catalog("user_c1_member")
    p3, n3 = create_project_and_ns(
        user_token["user_c1_member"]["token"],
        project_detail["cluster1"]["cluster"],
        random_test_name("testapp"))
    proj_client_user = get_project_client_for_token(
        p3, user_token["user_c1_member"]["token"])
    validate_catalog_app_deploy(proj_client_user,
                                n3.name,
                                p3.id,
                                cluster_catalog_external_id)
    user_client = get_user_client()
    user_client.delete(p3)


@if_test_rbac
def test_rbac_app_cluster_catalog_list_8():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # user_c2_owner CANNOT list the catalog
    validate_user_list_catalog("user_c2_owner", False)


@if_test_rbac
def test_rbac_app_project_scope_delete_1():
    catalog, cluster_catalog_external_id = create_project_catalog()
    # Verify user_c1_p1_owner CAN delete the added catalog
    validate_catalog_deletion(catalog, "user_c1_p1_owner", True, False)


@if_test_rbac
def test_rbac_app_project_scope_delete_2():
    catalog, cluster_catalog_external_id = create_project_catalog()
    # Verify user_c1_p1_member CANNOT delete the added catalog
    validate_catalog_deletion(catalog, "user_c1_p1_member", False, False)


@if_test_rbac
def test_rbac_app_project_scope_delete_3():
    catalog, cluster_catalog_external_id = create_project_catalog()
    # Verify user_c1_p1_member CANNOT delete the added catalog
    validate_catalog_deletion(catalog, "user_c1_p1_member", False, False)


@if_test_rbac
def test_rbac_app_project_scope_delete_4():
    catalog, cluster_catalog_external_id = create_project_catalog()
    # Verify user_c1_p2_owner CANNOT delete the added catalog
    validate_catalog_deletion(catalog, "user_c1_p2_owner", False, False)


@if_test_rbac
def test_rbac_app_project_scope_delete_5():
    catalog, cluster_catalog_external_id = create_project_catalog()
    # Verify user_c2_p1_owner CANNOT delete the added catalog
    validate_catalog_deletion(catalog, "user_c2_p1_owner", False, False)


@if_test_rbac
def test_rbac_app_project_scope_delete_6():
    catalog, cluster_catalog_external_id = create_project_catalog()
    # Verify user_c1_owner CAN delete the added catalog
    validate_catalog_deletion(catalog, "user_c1_owner", True, False)


@if_test_rbac
def test_rbac_app_project_scope_delete_7():
    catalog, cluster_catalog_external_id = create_project_catalog()
    # Verify user_c1_member CANNOT delete the added catalog
    validate_catalog_deletion(catalog, "user_c1_member", False, False)


@if_test_rbac
def test_rbac_app_project_scope_delete_8():
    catalog, cluster_catalog_external_id = create_project_catalog()
    # Verify user_c2_owner CANNOT delete the added catalog
    validate_catalog_deletion(catalog, "user_c2_owner", False, False)


@if_test_rbac
def test_rbac_app_project_scope_delete_9():
    catalog, cluster_catalog_external_id = create_project_catalog()
    # Verify user_standard CANNOT delete the added catalog
    validate_catalog_deletion(catalog, "user_standard", False, False)


@if_test_rbac
def test_rbac_app_cluster_scope_delete_1():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # verify user_c1_p1_owner CANNOT delete the catalog
    validate_catalog_deletion(catalog, "user_c1_p1_owner", False)


@if_test_rbac
def test_rbac_app_cluster_scope_delete_2():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # verify user_c1_p1_member CANNOT delete the catalog
    validate_catalog_deletion(catalog, "user_c1_p1_member", False)


@if_test_rbac
def test_rbac_app_cluster_scope_delete_3():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # verify user_c1_p2_owner CANNOT delete the catalog
    validate_catalog_deletion(catalog, "user_c1_p2_owner", False)


@if_test_rbac
def test_rbac_app_cluster_scope_delete_4():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # verify user_c2_p1_owner CANNOT delete the catalog
    validate_catalog_deletion(catalog, "user_c2_p1_owner", False)


@if_test_rbac
def test_rbac_app_cluster_scope_delete_5():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # verify user_c1_owner CAN delete the catalog
    validate_catalog_deletion(catalog, "user_c1_owner", True)


@if_test_rbac
def test_rbac_app_cluster_scope_delete_6():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # verify user_c1_member CANNOT delete the catalog
    validate_catalog_deletion(catalog, "user_c1_member", False)


@if_test_rbac
def test_rbac_app_cluster_scope_delete_7():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # verify user_c2_owner CANNOT delete the catalog
    validate_catalog_deletion(catalog, "user_c2_owner", False)


@if_test_rbac
def test_rbac_app_cluster_scope_delete_8():
    catalog, cluster_catalog_external_id = create_cluster_catalog()
    # verify user_standard CANNOT delete the catalog
    validate_catalog_deletion(catalog, "user_standard", False)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    project_detail["cluster1"]["project1"] = rbac_data["project"]
    project_detail["cluster1"]["namespace1"] = rbac_data["namespace"]
    user_token["user_c1_p1_owner"]["user"] = \
        rbac_data["users"][PROJECT_OWNER]["user"]
    user_token["user_c1_p1_owner"]["token"] = \
        rbac_data["users"][PROJECT_OWNER]["token"]
    user_token["user_c1_p1_member"]["user"] = \
        rbac_data["users"][PROJECT_MEMBER]["user"]
    user_token["user_c1_p1_member"]["token"] = \
        rbac_data["users"][PROJECT_MEMBER]["token"]
    user_token["user_c1_owner"]["user"] = \
        rbac_data["users"][CLUSTER_OWNER]["user"]
    user_token["user_c1_owner"]["token"] = \
        rbac_data["users"][CLUSTER_OWNER]["token"]
    user_token["user_c1_member"]["user"] = \
        rbac_data["users"][CLUSTER_MEMBER]["user"]
    user_token["user_c1_member"]["token"] = \
        rbac_data["users"][CLUSTER_MEMBER]["token"]
    client, clusters = get_user_client_and_cluster_app()
    admin_client = get_admin_client()
    cluster1 = clusters[0]
    cluster2 = clusters[1]
    assert len(clusters) > 0
    project_detail["cluster1"]["project2"], \
    project_detail["cluster1"]["namespace2"] = \
        create_project_and_ns(USER_TOKEN,
                              cluster1,
                              random_test_name("testapp"))
    project_detail["cluster2"]["project1"], \
        project_detail["cluster2"]["namespace1"] = \
        create_project_and_ns(USER_TOKEN,
                              cluster2,
                              random_test_name("testapp"))
    project_detail["cluster1"]["cluster"] = cluster1
    project_detail["cluster2"]["cluster"] = cluster2

    catalog = admin_client.create_catalog(
        name=CATALOG_NAME,
        baseType="catalog",
        branch=BRANCH,
        kind="helm",
        url=CATALOG_URL)
    time.sleep(5)

    # create users
    user_token["user_c1_p2_owner"]["user"], \
        user_token["user_c1_p2_owner"]["token"] = create_user(admin_client)
    user_token["user_c2_p1_owner"]["user"], \
        user_token["user_c2_p1_owner"]["token"] = create_user(admin_client)
    user_token["user_c2_owner"]["user"], \
        user_token["user_c2_owner"]["token"] = create_user(admin_client)
    user_token["user_standard"]["user"], \
        user_token["user_standard"]["token"] = create_user(admin_client)

    # Assign roles to the users
    assign_members_to_project(admin_client,
                              user_token["user_c1_p2_owner"]["user"],
                              project_detail["cluster1"]["project2"],
                              "project-owner")
    assign_members_to_project(admin_client,
                              user_token["user_c2_p1_owner"]["user"],
                              project_detail["cluster2"]["project1"],
                              "project-owner")
    assign_members_to_cluster(admin_client,
                              user_token["user_c2_owner"]["user"],
                              project_detail["cluster2"]["cluster"],
                              "cluster-owner")

    def fin():
        admin_client.delete(project_detail["cluster1"]["project2"])
        admin_client.delete(project_detail["cluster2"]["project1"])
        admin_client.delete(catalog)
        admin_client.delete(user_token["user_c1_p2_owner"]["user"])
        admin_client.delete(user_token["user_c2_p1_owner"]["user"])
        admin_client.delete(user_token["user_c2_owner"]["user"])
        admin_client.delete(user_token["user_standard"]["user"])

    request.addfinalizer(fin)


def validate_user_list_catalog(user, listcatalog=True, clustercatalog=True):
    user_client = get_client_for_token(
        user_token[user]["token"])
    if clustercatalog:
        catalogs_list = user_client.list_clusterCatalog(name=CLUSTER_CATALOG)
        catalogName = CLUSTER_CATALOG
    else:
        catalogs_list = user_client.list_projectCatalog(name=PROJECT_CATALOG)
        catalogName = PROJECT_CATALOG

    if listcatalog:
        print("Length of catalog list:", len(catalogs_list))
        assert len(catalogs_list) == 1, \
            "Catalog not found for the user"
        assert catalogs_list["data"][0]["name"] == catalogName, \
            "Incorrect catalog found"
    else:
        assert len(catalogs_list) == 0, \
            "Catalog found for the user"


def validate_catalog_app_deploy(proj_client_user, namespace,
                                projectid, catalog_ext_id):
    try:
        app = proj_client_user.create_app(name=random_test_name(),
                                          externalId=catalog_ext_id,
                                          answers=get_defaut_question_answers(
                                                  get_user_client(),
                                                  catalog_ext_id),
                                          targetNamespace=namespace,
                                          projectId=projectid)
        pass
    except:
        assert False, "User is not able to deploy app from catalog"
    validate_catalog_app(proj_client_user, app, catalog_ext_id)
    proj_client_user.delete(app)


def validate_catalog_deletion(catalog,
                              user, candelete=True, clustercatalog=True):
    user_client = get_client_for_token(user_token[user]["token"])
    catalog_name = catalog.name
    if not candelete:
        with pytest.raises(ApiError) as e:
            user_client.delete(catalog)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
    else:
        user_client.delete(catalog)
        if clustercatalog:
            catalogs_list = user_client.list_clusterCatalog(name=catalog_name)
        else:
            catalogs_list = user_client.list_projectCatalog(name=catalog_name)
        assert len(catalogs_list) == 0, \
            "Catalog has not been deleted for the user"


def create_project_catalog():
    """create a catalog by user1 at the project level
    and allow other users to access it"""
    added_catalog = None
    catalog_external_id = None
    user_client = get_user_client()
    catalogs_list = user_client.list_projectCatalog(name=PROJECT_CATALOG)
    pId = project_detail["cluster1"]["project1"].id.split(":")[1]
    if len(catalogs_list["data"]) != 0:
        catalog_proj_scoped_ext_id = \
            create_catalog_external_id(catalogs_list["data"][0]["name"],
                                       "mysql",
                                       "1.3.2",
                                       pId,
                                       "project")
        added_catalog = catalogs_list["data"][0]
        catalog_external_id = catalog_proj_scoped_ext_id
    else:
        catalog = user_client.create_projectCatalog(
            name=PROJECT_CATALOG,
            baseType="projectCatalog",
            branch=BRANCH,
            url=CATALOG_URL,
            projectId=project_detail["cluster1"]["project1"].id)
        time.sleep(10)
        assert catalog.state == "active", "Catalog is not in Active state."

        catalog_proj_scoped_ext_id = \
            create_catalog_external_id(catalog.name,
                                       "mysql",
                                       "1.3.2",
                                       pId,
                                       "project")
        print(catalog_proj_scoped_ext_id)
        answers = get_defaut_question_answers(
            user_client,
            catalog_proj_scoped_ext_id)
        proj_client = get_project_client_for_token(
            project_detail["cluster1"]["project1"],
            USER_TOKEN)
        app = proj_client.create_app(
            name=random_test_name(),
            externalId=catalog_proj_scoped_ext_id,
            answers=answers,
            targetNamespace=project_detail["cluster1"]["namespace1"].name,
            projectId=project_detail["cluster1"]["project1"].id)
        validate_catalog_app(proj_client, app, catalog_proj_scoped_ext_id)
        proj_client.delete(app)
        added_catalog = catalog
        catalog_external_id = catalog_proj_scoped_ext_id

    return added_catalog, catalog_external_id


def create_cluster_catalog():
    """create a catalog by user1 at the cluster level
    and allow other users to access it"""
    added_catalog = None
    catalog_external_id = None
    user_client = get_user_client()
    catalogs_list = user_client.list_clusterCatalog(name=CLUSTER_CATALOG)
    pId = project_detail["cluster1"]["cluster"].id
    # catalog = catalogs_list[0]
    if len(catalogs_list["data"]) != 0:
        catalog_cluster_scoped_ext_id = \
            create_catalog_external_id(catalogs_list["data"][0]["name"],
                                       "mysql",
                                       "1.3.2",
                                       pId,
                                       "cluster")
        added_catalog = catalogs_list["data"][0]
        catalog_external_id = catalog_cluster_scoped_ext_id
    else:
        proj_client = get_project_client_for_token(
            project_detail["cluster1"]["project1"],
            USER_TOKEN)
        print(project_detail["cluster1"]["cluster"].id)
        print(project_detail["cluster1"]["cluster"])
        catalog = user_client.create_clusterCatalog(
            name=CLUSTER_CATALOG,
            baseType="clustercatalog",
            branch=BRANCH,
            url=CATALOG_URL,
            clusterId=project_detail["cluster1"]["cluster"].id)
        time.sleep(10)
        assert catalog.state == "active", "Catalog is not in Active state."
        catalog_cluster_scoped_ext_id = \
            create_catalog_external_id(catalog.name,
                                       "mysql",
                                       "1.3.2",
                                       pId,
                                       "cluster")
        answers = get_defaut_question_answers(
            user_client,
            catalog_cluster_scoped_ext_id)
        app = proj_client.create_app(
            name=random_test_name(),
            externalId=catalog_cluster_scoped_ext_id,
            answers=answers,
            targetNamespace=project_detail["cluster1"]["namespace1"].name,
            projectId=project_detail["cluster1"]["project1"].id)
        validate_catalog_app(proj_client, app, catalog_cluster_scoped_ext_id)
        proj_client.delete(app)
        added_catalog = catalog
        catalog_external_id = catalog_cluster_scoped_ext_id

    return added_catalog, catalog_external_id
