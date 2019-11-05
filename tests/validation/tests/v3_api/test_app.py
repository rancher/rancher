from .common import *  # NOQA
import pytest

project_detail = {"project": None, "namespace": None, "cluster": None,
                  "project2": None, "namespace2": None, "cluster2": None}
user_token = {"user_c1_p1_owner": {"user": None, "token": None},
              "user_c1_p1_member": {"user": None, "token": None},
              "user_c1_p2_owner": {"user": None, "token": None},
              "user_standard": {"user": None, "token": None}}

CATALOG_URL = "https://git.rancher.io/charts"
MYSQL_EXTERNALID_037 = "catalog://?catalog=library&template=mysql" \
                       "&version=0.3.7"
MYSQL_EXTERNALID_038 = "catalog://?catalog=library&template=mysql" \
                       "&version=0.3.8"
WORDPRESS_EXTID = "catalog://?catalog=library&template=wordpress" \
                  "&version=1.0.5"


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


def test_app_deploy():
    admin_client = get_user_client()
    proj_client = get_project_client_for_token(
        project_detail["project"],
        USER_TOKEN)
    answer = get_defaut_question_answers(
        admin_client,
        MYSQL_EXTERNALID_037)
    wait_for_template_to_be_created(admin_client, "library")
    app = proj_client.create_app(
        name=random_test_name(),
        externalId=MYSQL_EXTERNALID_037,
        targetNamespace=project_detail["namespace"].name,
        projectId=project_detail["project"].id,
        answers=answer)
    print("App is active")
    validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_037)
    proj_client.delete(app)


def test_app_delete():
    admin_client = get_user_client()
    proj_client = get_project_client_for_token(
        project_detail["project"],
        USER_TOKEN)
    wait_for_template_to_be_created(admin_client, "library")
    answer = get_defaut_question_answers(
        admin_client,
        MYSQL_EXTERNALID_037)
    app = proj_client.create_app(
        name=random_test_name(),
        externalId=MYSQL_EXTERNALID_037,
        targetNamespace=project_detail["namespace"].name,
        projectId=project_detail["project"].id,
        answers=answer)
    print("App is active")
    validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_037)
    app = proj_client.delete(app)
    validate_app_deletion(proj_client, app.id)


def test_app_upgrade_version():
    admin_client = get_user_client()
    proj_client = get_project_client_for_token(
        project_detail["project"],
        USER_TOKEN)
    wait_for_template_to_be_created(admin_client, "library")
    answer = get_defaut_question_answers(
        admin_client,
        MYSQL_EXTERNALID_037)
    app = proj_client.create_app(
        name=random_test_name(),
        externalId=MYSQL_EXTERNALID_037,
        targetNamespace=project_detail["namespace"].name,
        projectId=project_detail["project"].id,
        answers=answer)
    print("App is active")
    validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_037)
    new_answer = get_defaut_question_answers(
            admin_client,
            MYSQL_EXTERNALID_038)
    app = proj_client.update(
        obj=app,
        externalId=MYSQL_EXTERNALID_038,
        targetNamespace=project_detail["namespace"].name,
        projectId=project_detail["project"].id,
        answers=new_answer)
    app = proj_client.reload(app)
    validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_038)
    assert app.externalId == MYSQL_EXTERNALID_038, "incorrect template version"
    proj_client.delete(app)


def test_app_rollback():
    admin_client = get_user_client()
    proj_client = get_project_client_for_token(
        project_detail["project"],
        USER_TOKEN)
    wait_for_template_to_be_created(admin_client, "library")
    answer = get_defaut_question_answers(
        admin_client,
        MYSQL_EXTERNALID_037)
    app = proj_client.create_app(
        name=random_test_name(),
        externalId=MYSQL_EXTERNALID_037,
        targetNamespace=project_detail["namespace"].name,
        projectId=project_detail["project"].id,
        answers=answer)
    print("App is active")
    app = validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_037)
    rev_id = app.appRevisionId
    new_answer = get_defaut_question_answers(
            admin_client,
            MYSQL_EXTERNALID_038)
    app = proj_client.update(
        obj=app,
        externalId=MYSQL_EXTERNALID_038,
        targetNamespace=project_detail["namespace"].name,
        projectId=project_detail["project"].id,
        answers=new_answer)
    app = proj_client.reload(app)
    app = validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_038)
    assert app.externalId == MYSQL_EXTERNALID_038, "incorrect template version"
    proj_client.action(obj=app,
                       action_name='rollback',
                       revisionId=rev_id)
    app = proj_client.reload(app)
    validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_037)
    assert app.externalId == MYSQL_EXTERNALID_037, "incorrect template version"
    proj_client.delete(app)


def test_app_answer_override():
    admin_client = get_user_client()
    proj_client = get_project_client_for_token(
        project_detail["project"],
        USER_TOKEN)
    wait_for_template_to_be_created(admin_client, "library")
    answers = get_defaut_question_answers(
        admin_client,
        MYSQL_EXTERNALID_037)
    app = proj_client.create_app(
        name=random_test_name(),
        externalId=MYSQL_EXTERNALID_037,
        targetNamespace=project_detail["namespace"].name,
        projectId=project_detail["project"].id,
        answers=answers)
    print("App is active")
    app = validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_037)
    answers["mysqlUser"] = "admin1234"
    app = proj_client.update(
        obj=app,
        externalId=MYSQL_EXTERNALID_037,
        targetNamespace=project_detail["namespace"].name,
        projectId=project_detail["project"].id,
        answers=answers)
    app = proj_client.reload(app)
    app = validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_037, answers)
    assert app["answers"].mysqlUser == "admin1234", \
        "incorrect answer upgrade"
    proj_client.delete(app)


def test_rbac_app_project_scope_deploy():
    admin_client = get_user_client()
    proj_client = get_project_client_for_token(
        project_detail["project"],
        USER_TOKEN)
    catalog = admin_client.create_projectCatalog(
        name="projectcatalog",
        baseType="projectCatalog",
        branch="master",
        url=CATALOG_URL,
        projectId=project_detail["project"].id)
    time.sleep(5)
    pId = project_detail["project"].id.split(":")[1]
    catalog_proj_scoped_ext_id = "catalog://?catalog=" + pId + \
                                 "/projectcatalog&type=" \
                                 "projectCatalog&template=" \
                                 "mysql&version=0.3.8"
    answers = get_defaut_question_answers(
        admin_client,
        catalog_proj_scoped_ext_id)
    app = proj_client.create_app(
        name=random_test_name(),
        externalId=catalog_proj_scoped_ext_id,
        answers=answers,
        targetNamespace=project_detail["namespace"].name,
        projectId=project_detail["project"].id)
    validate_catalog_app(proj_client, app, catalog_proj_scoped_ext_id)
    p2, ns2 = create_project_and_ns(
        USER_TOKEN,
        project_detail["cluster"],
        random_test_name("testapp"))
    #Assign role
    assign_members_to_project(admin_client,
                              user_token["user_c1_p2_owner"]["user"],
                              p2,
                              "project-owner")
    #Verify "project-owner" of p1 can list the added catalog
    user1_client = get_client_for_token(
        user_token["user_c1_p1_owner"]["token"])
    catalogs_list = user1_client.list_projectCatalog()
    assert len(catalogs_list) == 1, \
        "Project catalog not found for the user"
    assert catalogs_list["data"][0]["name"] == \
           "projectcatalog", "Incorrect project catalog found"

    # Verify "project-member" of p1 can list the added catalog
    user2_client = get_client_for_token(
        user_token["user_c1_p1_member"]["token"])
    catalogs_list_2 = user2_client.list_projectCatalog()
    assert len(catalogs_list_2) == 1, \
        "Project catalog not found for the user"

    # Verify "project-owner" of p2 CANNOT list the added catalog
    user3_client = get_client_for_token(
        user_token["user_c1_p2_owner"]["token"])
    catalogs_list_3 = user3_client.list_projectCatalog()
    assert len(catalogs_list_3) == 0, \
        "Project catalog found for the user"

    # Verify A standard user CANNOT list the added catalog
    user4_client = get_client_for_token(
        user_token["user_standard"]["token"])
    catalogs_list_4 = user4_client.list_projectCatalog()
    assert len(catalogs_list_4) == 0, \
        "Project catalog found for the user"
    admin_client.delete(p2)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client = get_admin_client()
    clusters = client.list_cluster(name=CLUSTER_NAME).data
    assert len(clusters) > 0
    project_detail["project"], project_detail["namespace"] = \
        create_project_and_ns(USER_TOKEN, clusters[0],
                              random_test_name("testapp"))
    project_detail["cluster"] = clusters[0]
    #create users
    user_token["user_c1_p1_owner"]["user"], \
        user_token["user_c1_p1_owner"]["token"] = create_user(client)
    user_token["user_c1_p1_member"]["user"], \
        user_token["user_c1_p1_member"]["token"] = create_user(client)
    user_token["user_c1_p2_owner"]["user"], \
        user_token["user_c1_p2_owner"]["token"] = create_user(client)
    user_token["user_standard"]["user"], \
        user_token["user_standard"]["token"] = create_user(client)
    #Assign roles to the users
    assign_members_to_project(client,
                              user_token["user_c1_p1_owner"]["user"],
                              project_detail["project"],
                              "project-owner")
    assign_members_to_project(client,
                              user_token["user_c1_p1_member"]["user"],
                              project_detail["project"],
                              "project-member")

    def fin():
        client = get_user_client()
        client.delete(project_detail["project"])
    request.addfinalizer(fin)
