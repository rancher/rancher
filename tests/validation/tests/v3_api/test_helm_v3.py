from .common import *  # NOQA
import pytest

CATALOG_NAME = random_test_name("test-v3")
CATALOG_URL = "https://github.com/rancher/integration-test-charts.git"
BRANCH = "validation-tests"
MYSQL_EXTERNALID_131 = \
    create_catalog_external_id(CATALOG_NAME, "mysql", "1.3.1")
MYSQL_EXTERNALID_132 = \
    create_catalog_external_id(CATALOG_NAME, "mysql", "1.3.2")
cluster_detail = {"cluster1": {"project1": None, "namespace1": None,
                               "cluster": None}}


def test_helm_v3_app_deploy():
    client = get_user_client()
    answer = get_defaut_question_answers(client, MYSQL_EXTERNALID_131)
    proj_client, ns, project = get_project_details("cluster1")
    app = create_and_validate_app(
        proj_client, MYSQL_EXTERNALID_131, ns, project, answer)
    proj_client.delete(app)


def test_helm_v3_app_delete():
    client = get_user_client()
    answer = get_defaut_question_answers(client, MYSQL_EXTERNALID_131)
    proj_client, ns, project = get_project_details("cluster1")
    app = create_and_validate_app(
        proj_client, MYSQL_EXTERNALID_131, ns, project, answer)
    app = proj_client.delete(app)
    validate_app_deletion(proj_client, app.id)


def test_helm_v3_app_upgrade_version():
    client = get_user_client()
    answer = get_defaut_question_answers(client, MYSQL_EXTERNALID_131)
    proj_client, ns, project = get_project_details("cluster1")
    # deploy app
    app = create_and_validate_app(
        proj_client, MYSQL_EXTERNALID_131, ns, project, answer)

    new_answers = get_defaut_question_answers(client, MYSQL_EXTERNALID_132)
    # update app
    app = update_and_validate_app(
        app, proj_client, MYSQL_EXTERNALID_132, ns, project, new_answers)
    proj_client.delete(app)


def test_helm_v3_app_rollback():
    client = get_user_client()
    answer = get_defaut_question_answers(client, MYSQL_EXTERNALID_131)
    proj_client, ns, project = get_project_details("cluster1")
    # deploy app
    app = create_and_validate_app(
        proj_client, MYSQL_EXTERNALID_131, ns, project, answer)
    rev_id = app.appRevisionId
    new_answer = get_defaut_question_answers(client, MYSQL_EXTERNALID_132)
    # update app
    app = update_and_validate_app(
        app, proj_client, MYSQL_EXTERNALID_132, ns, project, new_answer)
    proj_client.action(obj=app,
                       action_name='rollback',
                       revisionId=rev_id)
    app = proj_client.reload(app)
    app = validate_catalog_app(proj_client, app, MYSQL_EXTERNALID_131)
    proj_client.delete(app)


def test_helm_v3_app_answer_override():
    client = get_user_client()
    answer = get_defaut_question_answers(client, MYSQL_EXTERNALID_131)
    proj_client, ns, project = get_project_details("cluster1")
    # deploy app
    app = create_and_validate_app(
        proj_client, MYSQL_EXTERNALID_131, ns, project, answer
    )
    answer["mysqlUser"] = "admin1234"
    # update app
    app = update_and_validate_app(
        app, proj_client, MYSQL_EXTERNALID_132, ns, project, answer
    )
    assert app["answers"].mysqlUser == "admin1234", \
        "incorrect answer upgrade"
    proj_client.delete(app)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster_existing = get_user_client_and_cluster()
    cluster_detail["cluster1"]["cluster"] = cluster_existing
    cluster_detail["cluster1"]["project1"], \
        cluster_detail["cluster1"]["namespace1"] =\
        create_project_and_ns(
            USER_TOKEN,
            cluster_existing,
            random_test_name("test-helmv3")
        )
    # add catalog
    admin_client = get_admin_client()
    v3_catalog = admin_client.create_catalog(
        name=CATALOG_NAME,
        baseType="catalog",
        branch=BRANCH,
        kind="helm",
        url=CATALOG_URL,
        helmVersion="helm_v3")
    assert v3_catalog["helmVersion"] == \
           "helm_v3", "Helm version is not helm_v3"
    time.sleep(5)

    def fin():
        admin_client.delete(v3_catalog)
        admin_client.delete(cluster_detail["cluster1"]["namespace1"])
        admin_client.delete(cluster_detail["cluster1"]["project1"])

    request.addfinalizer(fin)


def create_and_validate_app(proj_client, externalid, ns, project, answer):
    """
    :param proj_client: Project client of the project
    where the app will be deployed
    :param externalid: App's external ID
    :param ns: namespace
    :param project: project
    :param answer: answers for the app with external_id: externalid
    :return: app
    """
    app = proj_client.create_app(
        name=random_test_name(),
        externalId=externalid,
        targetNamespace=ns,
        projectId=project,
        answers=answer)
    app = validate_catalog_app(proj_client, app, externalid)
    assert app["helmVersion"] == "helm_v3", "Helm version is not helm_v3"
    return app


def update_and_validate_app(app, proj_client, externalid, ns, project, answer):
    """
    :param app: app object to be updated
    :param proj_client: Project client of the project
    where the app will be deployed
    :param externalid: App's external ID
    :param ns: namespace
    :param project: project
    :param answer: answers for the app with external_id: externalid
    :return: app
    """
    app = proj_client.update(
        obj=app,
        externalId=externalid,
        targetNamespace=ns,
        projectId=project,
        answers=answer)
    app = validate_catalog_app(proj_client, app, externalid, answer)
    assert app["helmVersion"] == "helm_v3", "Helm version is not helm_v3"
    return app


def get_project_details(cluster):
    """
    :param cluster: cluster is a "key" in the
    cluster_detail pointing to the cluster
    :return: proj_client, ns, project
    """
    proj_client = get_project_client_for_token(
        cluster_detail[cluster]["project1"],
        USER_TOKEN
    )
    ns = cluster_detail["cluster1"]["namespace1"].name
    project = cluster_detail["cluster1"]["project1"].id
    return proj_client, ns, project
