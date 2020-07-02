from .test_rke_cluster_provisioning import create_and_validate_custom_host
from .common import *
import pytest
import time


project = {}
project_detail = {"c0_id": None, "c1_id": None, "c2_id": None,
                  "p0_id": None, "p1_id": None, "p2_id": None,
                  "p_client0": None, "namespace0": None,
                  "cluster0": None, "project0": None,
                  "p_client1": None, "namespace1": None,
                  "cluster1": None, "project1": None,
                  "p_client2": None, "namespace2": None,
                  "cluster2": None, "project2": None}

global_client = {"cluster_count": False}
PROJECT_ROLE = ["project-member"]
CATALOG_URL = "https://github.com/rancher/integration-test-charts.git"
BRANCH = "validation-tests"
CATALOG_NAME = random_test_name("test-catalog")
WORDPRESS_TEMPLATE_VID_738 = \
    "cattle-global-data:" + CATALOG_NAME + "-wordpress-7.3.8"
MYSQL_TEMPLATE_VID_131 = "cattle-global-data:" + CATALOG_NAME + "-mysql-1.3.1"
MYSQL_TEMPLATE_VID_132 = "cattle-global-data:" + CATALOG_NAME + "-mysql-1.3.2"
GRAFANA_TEMPLATE_VID = "cattle-global-data:" + CATALOG_NAME + "-grafana-3.8.6"
WORDPRESS_EXTID = create_catalog_external_id(CATALOG_NAME,
                                             "wordpress", "7.3.8")
MYSQL_EXTERNALID_131 = create_catalog_external_id(CATALOG_NAME,
                                                  "mysql", "1.3.1")
MYSQL_EXTERNALID_132 = create_catalog_external_id(CATALOG_NAME,
                                                  "mysql", "1.3.2")
GRAFANA_EXTERNALID = create_catalog_external_id(CATALOG_NAME,
                                                "grafana", "3.8.6")
ROLLING_UPGRADE_STRATEGY = {
        'rollingUpdate': {
            'batchSize': 1,
            'interval': 20,
            'type': '/v3/schemas/rollingUpdate'},
        'type': '/v3/schemas/upgradeStrategy'}

skip_test_rolling_update = pytest.mark.skipif(
    reason="Skipping this test always "
           "as for now its not in scope for automation")


def test_multi_cluster_app_create():
    client = get_user_client()
    assert_if_valid_cluster_count()
    targets = []
    for project_id in project:
        targets.append({"projectId": project_id, "type": "target"})
    answer_values = get_defaut_question_answers(client, WORDPRESS_EXTID)
    mcapp = client.create_multiClusterApp(
        templateVersionId=WORDPRESS_TEMPLATE_VID_738,
        targets=targets,
        roles=PROJECT_ROLE,
        name=random_name(),
        answers=[{"values": answer_values}])
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster_wordpress(mcapp)
    client.delete(mcapp)


def test_multi_cluster_app_edit_template_upgrade():
    client = get_user_client()
    assert_if_valid_cluster_count()
    targets = []
    for project_id in project:
        targets.append({"projectId": project_id, "type": "target"})
    answer_values = \
        get_defaut_question_answers(client, MYSQL_EXTERNALID_131)
    mcapp = client.create_multiClusterApp(
        templateVersionId=MYSQL_TEMPLATE_VID_131,
        targets=targets,
        roles=PROJECT_ROLE,
        name=random_name(),
        answers=[{"values": answer_values}])
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    answer_values_new = get_defaut_question_answers(client,
                                                    MYSQL_EXTERNALID_132)
    mcapp = client.update(mcapp,
                          roles=PROJECT_ROLE,
                          templateVersionId=MYSQL_TEMPLATE_VID_132,
                          answers=[{"values": answer_values_new}])
    mcapp = client.reload(mcapp)
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    client.delete(mcapp)


def test_multi_cluster_app_delete():
    assert_if_valid_cluster_count()
    targets = []
    for project_id in project:
        targets.append({"projectId": project_id, "type": "target"})
    client = get_user_client()
    answer_values = get_defaut_question_answers(client, MYSQL_EXTERNALID_131)
    mcapp = client.create_multiClusterApp(
        templateVersionId=MYSQL_TEMPLATE_VID_131,
        targets=targets,
        roles=PROJECT_ROLE,
        name=random_name(),
        answers=[{"values": answer_values}])
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    delete_multi_cluster_app(mcapp, True)


def test_multi_cluster_app_template_rollback():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = get_user_client()
    answer_values = get_defaut_question_answers(client, MYSQL_EXTERNALID_131)
    mcapp = client.create_multiClusterApp(
        templateVersionId=MYSQL_TEMPLATE_VID_131,
        targets=targets,
        roles=PROJECT_ROLE,
        name=random_name(),
        answers=[{"values": answer_values}])
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    first_id = mcapp["status"]["revisionId"]
    assert mcapp.templateVersionId == MYSQL_TEMPLATE_VID_131
    answer_values_new = get_defaut_question_answers(
        client, MYSQL_EXTERNALID_132)
    mcapp = client.update(mcapp,
                          roles=PROJECT_ROLE,
                          templateVersionId=MYSQL_TEMPLATE_VID_132,
                          answers=[{"values": answer_values_new}])
    mcapp = client.reload(mcapp)
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    assert mcapp.templateVersionId == MYSQL_TEMPLATE_VID_132
    mcapp.rollback(revisionId=first_id)
    mcapp = client.reload(mcapp)
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    assert mcapp.templateVersionId == MYSQL_TEMPLATE_VID_131
    client.delete(mcapp)


def test_multi_cluster_upgrade_and_add_target():
    assert_if_valid_cluster_count()
    project_id = project_detail["p0_id"]
    targets = [{"projectId": project_id, "type": "target"}]
    project_id_2 = project_detail["p1_id"]
    client = get_user_client()
    answer_values = get_defaut_question_answers(client, MYSQL_EXTERNALID_131)
    mcapp = client.create_multiClusterApp(
        templateVersionId=MYSQL_TEMPLATE_VID_131,
        targets=targets,
        roles=PROJECT_ROLE,
        name=random_name(),
        answers=[{"values": answer_values}])
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    uuid = mcapp.uuid
    name = mcapp.name
    assert len(client.list_multiClusterApp(
        uuid=uuid, name=name).data[0]["targets"]) == 1, \
        "did not start with 1 target"
    mcapp.addProjects(projects=[project_id_2])
    mcapp = client.reload(mcapp)
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    assert len(client.list_multiClusterApp(
        uuid=uuid, name=name).data[0]["targets"]) == 2, "did not add target"
    validate_multi_cluster_app_cluster(mcapp)
    client.delete(mcapp)


def test_multi_cluster_upgrade_and_delete_target():
    assert_if_valid_cluster_count()
    project_id = project_detail["p0_id"]
    targets = []
    for project_id in project:
        targets.append({"projectId": project_id, "type": "target"})
    client = get_user_client()
    answer_values = get_defaut_question_answers(client, MYSQL_EXTERNALID_131)
    mcapp = client.create_multiClusterApp(
        templateVersionId=MYSQL_TEMPLATE_VID_131,
        targets=targets,
        roles=PROJECT_ROLE,
        name=random_name(),
        answers=[{"values": answer_values}])
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    uuid = mcapp.uuid
    name = mcapp.name
    assert len(client.list_multiClusterApp(
        uuid=uuid, name=name).data[0]["targets"]) == 2, \
        "did not start with 2 targets"
    project_client = project_detail["p_client0"]
    app = mcapp.targets[0].projectId.split(":")
    app1id = app[1] + ":" + mcapp.targets[0].appId
    client.action(obj=mcapp, action_name="removeProjects",
                  projects=[project_id])
    mcapp = client.reload(mcapp)
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    assert len(mcapp["targets"]) == 1, "did not delete target"
    validate_app_deletion(project_client, app1id)
    client.delete(mcapp)


def test_multi_cluster_role_change():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = get_user_client()
    original_role = ["project-member"]
    answer_values = get_defaut_question_answers(client, GRAFANA_EXTERNALID)
    mcapp = client.create_multiClusterApp(
        templateVersionId=GRAFANA_TEMPLATE_VID,
        targets=targets,
        roles=original_role,
        name=random_name(),
        answers=[{"values": answer_values}])
    try:
        mcapp = wait_for_mcapp_to_active(client, mcapp, 10)
    except AssertionError:
        print("expected failure as project member")
        pass  # expected fail
    mcapp = client.update(mcapp, roles=["cluster-owner"])
    client.reload(mcapp)
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    client.delete(mcapp)


def test_multi_cluster_project_answer_override():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = get_user_client()
    answer_values = get_defaut_question_answers(client, MYSQL_EXTERNALID_131)
    mcapp = client.create_multiClusterApp(
        templateVersionId=MYSQL_TEMPLATE_VID_131,
        targets=targets,
        roles=PROJECT_ROLE,
        name=random_name(),
        answers=[{"values": answer_values}])
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    answers_override = {
            "clusterId": None,
            "projectId": project_detail["p0_id"],
            "type": "/v3/schemas/answer",
            "values": {
                "mysqlUser": "test_override"}
    }
    mysql_override = []
    mysql_override.extend([{"values": answer_values}, answers_override])
    mcapp = client.update(mcapp,
                          roles=PROJECT_ROLE,
                          answers=mysql_override)
    mcapp = client.reload(mcapp)
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    projectId_answer_override = project_detail["p0_id"]
    validate_answer_override(mcapp,
                             projectId_answer_override,
                             answers_override,
                             False)
    client.delete(mcapp)


def test_multi_cluster_cluster_answer_override():
    assert_if_valid_cluster_count()
    client = get_user_client()
    cluster1 = project_detail["cluster1"]
    p3, ns3 = create_project_and_ns(
        USER_TOKEN, cluster1, random_test_name("mcapp-3"))
    p_client2 = get_project_client_for_token(p3, USER_TOKEN)
    project_detail["c2_id"] = cluster1.id
    project_detail["namespace2"] = ns3
    project_detail["p2_id"] = p3.id
    project_detail["p_client2"] = p_client2
    project_detail["cluster2"] = cluster1
    project_detail["project2"] = p3
    project[p3.id] = project_detail
    client = global_client["client"]
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    answer_values = get_defaut_question_answers(client, MYSQL_EXTERNALID_131)
    mcapp = client.create_multiClusterApp(
        templateVersionId=MYSQL_TEMPLATE_VID_131,
        targets=targets,
        roles=PROJECT_ROLE,
        name=random_name(),
        answers=[{"values": answer_values}])
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    answers_override_cluster = {
        "clusterId": project_detail["c0_id"],
        "projectId": None,
        "type": "/v3/schemas/answer",
        "values": {
            "mysqlUser": "test_override"}
    }
    mysql_override_cluster = []
    mysql_override_cluster.extend([{"values": answer_values},
                                   answers_override_cluster])
    clusterId_answer_override = project_detail["c0_id"]
    mcapp = client.update(mcapp,
                          roles=PROJECT_ROLE,
                          answers=mysql_override_cluster)
    mcapp = client.reload(mcapp)
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    validate_answer_override(mcapp,
                             clusterId_answer_override,
                             answers_override_cluster)
    client.delete(mcapp)


def test_multi_cluster_all_answer_override():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = get_user_client()
    answer_values = get_defaut_question_answers(client, MYSQL_EXTERNALID_131)
    mcapp = client.create_multiClusterApp(
        templateVersionId=MYSQL_TEMPLATE_VID_131,
        targets=targets,
        roles=PROJECT_ROLE,
        name=random_name(),
        answers=[{"values": answer_values}])
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    new_answers = {"values": answer_values}
    new_answers["values"]["mysqlUser"] = "root"
    mcapp = client.update(mcapp,
                          roles=PROJECT_ROLE,
                          answers=[new_answers])
    mcapp = client.reload(mcapp)
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    validate_multi_cluster_app_cluster(mcapp)
    validate_all_answer_override_mca(mcapp)
    client.delete(mcapp)


@if_test_rbac
@pytest.mark.parametrize("role", ["owner", "member", "read-only"])
def test_rbac_multi_cluster_access_update(role):
    admin_client = get_admin_client()
    # adding targets
    targets = list()
    targets.append({"projectId": rbac_data["project"].id, "type": "target"})
    user = get_user_by_role(role)
    member = get_member_list(role, user)
    print("member:", member)

    answer_values = get_defaut_question_answers(
        admin_client,
        MYSQL_EXTERNALID_131
    )
    mcapp = create_mcapp_with_member(targets, answer_values, member)
    admin_client.reload(mcapp)
    time.sleep(5)
    admin_client.reload(mcapp)
    # verfiy rbac CLUSTER_OWNER can edit/upgrade the mcapp
    new_user, token = create_user(admin_client)
    mcapp = verify_rbac_multiclusterapp_update(
        new_user,
        rbac_get_user_token_by_role(CLUSTER_OWNER),
        role,
        mcapp,
        answer_values
    )
    admin_client.delete(new_user)
    admin_client.delete(mcapp)


@if_test_rbac
@pytest.mark.parametrize("role", ["owner", "member", "read-only"])
def test_rbac_multi_cluster_access_add_member(role):
    admin_client = get_admin_client()
    # adding targets
    targets = list()
    targets.append({"projectId": rbac_data["project"].id, "type": "target"})
    user = get_user_by_role(role)
    member = get_member_list(role, user)
    print("member:", member)

    answer_values = get_defaut_question_answers(
        admin_client,
        MYSQL_EXTERNALID_131
    )
    mcapp = create_mcapp_with_member(targets, answer_values, member)
    admin_client.reload(mcapp)
    time.sleep(5)
    admin_client.reload(mcapp)
    # verfiy rbac CLUSTER_OWNER can edit/upgrade the mcapp
    new_user, token = create_user(admin_client)
    mcapp = verify_rbac_multiclusterapp_add_member(
        new_user,
        rbac_get_user_token_by_role(CLUSTER_OWNER),
        role,
        mcapp,
        answer_values
    )
    admin_client.delete(new_user)
    admin_client.delete(mcapp)


@skip_test_rolling_update
def test_multi_cluster_rolling_upgrade():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = get_user_client()
    answer_values = get_defaut_question_answers(client, MYSQL_EXTERNALID_131)
    mcapp = client.create_multiClusterApp(
        templateVersionId=MYSQL_TEMPLATE_VID_131,
        targets=targets,
        roles=PROJECT_ROLE,
        name=random_name(),
        answers=[{"values": answer_values}],
        upgradeStrategy=ROLLING_UPGRADE_STRATEGY)
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    new_answers = {"values": answer_values}
    new_answers["values"]["mysqlUser"] = "admin1234"
    mcapp = client.update(mcapp,
                          roles=["cluster-owner"],
                          answers=[new_answers])
    mcapp = client.reload(mcapp)
    app_info = {"p_client": None, "app_id": None}
    app_info_2 = {"p_client": None, "app_id": None}
    start = time.time()
    end = time.time()
    time.sleep(5)
    app_state = []
    for i in range(0, len(mcapp.targets)):
        app_id = mcapp.targets[i].appId
        assert app_id is not None, "app_id is None"
        project_client = project_detail["p_client" + str(i)]
        app_detail = project_client.list_app(id=app_id).data[0]
        app_state.append(app_detail.state)
        if app_detail.state == "active":
            app_info["p_client"] = project_client
            app_info["app_id"] = app_id
        else:
            app_info_2["p_client"] = project_client
            app_info_2["app_id"] = app_id
    assert app_state.count("active") == 1, "Only one app should be upgrading"
    print("app_state: ", app_state)
    # check interval time is 20 seconds
    while True:
        app = app_info["p_client"].list_app(id=app_info["app_id"]).data[0]
        app2 = app_info_2["p_client"].list_app(id=app_info_2["app_id"]).data[0]
        if app2.state == "active":
            start_1 = time.time()
        if app.state != "active":
            end = time.time()
            break
    print("Start: ", start)
    print("Start_1: ", start_1)
    print("End: ", end)
    print(end - start)
    print(end - start_1)
    validate_multi_cluster_app_cluster(mcapp)
    client.delete(mcapp)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    node_roles = [["controlplane", "etcd", "worker"],
                  ["worker"], ["worker"]]
    cluster_list = []
    client, cluster_existing = get_user_client_and_cluster()
    cluster, aws_nodes = create_and_validate_custom_host(node_roles, True)
    admin_client = get_admin_client()
    client = get_user_client()
    # add clusters to cluster_list
    cluster_list.append(cluster_existing)
    cluster_list.append(cluster)
    if len(cluster_list) > 1:
        global_client["cluster_count"] = True
    assert_if_valid_cluster_count()
    p1, ns1 = create_project_and_ns(
        USER_TOKEN, cluster_list[0], random_test_name("mcapp-1"))
    p_client1 = get_project_client_for_token(p1, USER_TOKEN)
    p2, ns2 = create_project_and_ns(
        USER_TOKEN, cluster_list[1], random_test_name("mcapp-2"))
    p_client2 = get_project_client_for_token(p2, USER_TOKEN)
    project_detail["c0_id"] = cluster_list[0].id
    project_detail["p0_id"] = p1.id
    project_detail["namespace0"] = ns1
    project_detail["p_client0"] = p_client1
    project_detail["cluster0"] = cluster_list[0]
    project_detail["project0"] = p1
    project[p1.id] = project_detail
    project_detail["c1_id"] = cluster_list[1].id
    project_detail["namespace1"] = ns2
    project_detail["p1_id"] = p2.id
    project_detail["p_client1"] = p_client2
    project_detail["cluster1"] = cluster_list[1]
    project_detail["project1"] = p2
    project[p2.id] = project_detail
    global_client["client"] = client
    catalog = admin_client.create_catalog(
        name=CATALOG_NAME,
        baseType="catalog",
        branch=BRANCH,
        kind="helm",
        url=CATALOG_URL)
    catalog = wait_for_catalog_active(admin_client, catalog)

    def fin():
        admin_client.delete(catalog)
        admin_client.delete(p1)
        admin_client.delete(p2)
        admin_client.delete(project_detail["project2"])
        admin_client.delete(cluster)
        if aws_nodes is not None:
            delete_node(aws_nodes)

    request.addfinalizer(fin)


def assert_if_valid_cluster_count():
    assert global_client["cluster_count"], \
        "Setup Failure. Tests require at least 2 clusters"


def validate_multi_cluster_app_cluster_wordpress(multiclusterapp):
    for i in range(0, len(multiclusterapp.targets)):
        app_id = multiclusterapp.targets[i].appId
        assert app_id is not None, "app_id is None"
        project_client = project_detail["p_client"+str(i)]
        wait_for_app_to_active(project_client, app_id)
        validate_app_version(project_client, multiclusterapp, app_id)
        validate_response_app_endpoint(project_client, app_id)


def validate_multi_cluster_app_cluster(multiclusterapp):
    for i in range(0, len(multiclusterapp.targets)):
        app_id = multiclusterapp.targets[i].appId
        assert app_id is not None, "app_id is None"
        project_client = project_detail["p_client"+str(i)]
        wait_for_app_to_active(project_client, app_id)
        validate_app_version(project_client, multiclusterapp, app_id)


def delete_multi_cluster_app(multiclusterapp, validation=False):
    client = global_client["client"]
    uuid = multiclusterapp.uuid
    name = multiclusterapp.name
    client.delete(multiclusterapp)
    if validation:
        mcapps = client.list_multiClusterApp(uuid=uuid, name=name).data
        assert len(mcapps) == 0, "Multi Cluster App is not deleted"
        for i in range(1, len(multiclusterapp.targets)):
            app_id = multiclusterapp.targets[i].appId
            assert app_id is not None, "app_id is None"
            project_client = project_detail["p_client" + str(i)]
            validate_app_deletion(project_client, app_id)


def validate_app_version(project_client, multiclusterapp, app_id):
    temp_version = multiclusterapp.templateVersionId
    app = temp_version.split(":")[1].split("-")
    catalog_name = app[0] + "-" + app[1] + "-" + app[2]
    mcapp_template_version = "catalog://?catalog=" + catalog_name + \
                             "&template=" + app[3] + "&version=" + app[4]
    app_template_version = \
        project_client.list_app(name=app_id).data[0].externalId
    assert mcapp_template_version == app_template_version, \
        "App Id is different from the Multi cluster app id"


def return_application_status_and_upgrade(client1, app_id1, client2, app_id2):
    app_data1 = client1.list_app(id=app_id1).data
    application1 = app_data1[0]
    app_data2 = client2.list_app(id=app_id2).data
    application2 = app_data2[0]
    a = application1.state == "active" \
        and application1.answers["mysqlUser"] == "admin1234"
    b = application2.state == "active" \
        and application2.answers["mysqlUser"] == "admin1234"
    return a is True and b is not True


def validate_app_upgrade_mca(multiclusterapp):
    for i in range(0, len(multiclusterapp.targets)):
        project_client = project_detail["p_client" + str(i)]
        app = multiclusterapp.targets[0].projectId.split(":")
        appid = app[1] + ":" + multiclusterapp.targets[i].appId
        temp_version = multiclusterapp.templateVersionId
        app = temp_version.split(":")[1].split("-")
        mcapp_template_version = "catalog://?catalog=" + app[0] + \
                                 "&template=" + app[1] + "&version=" \
                                 + app[2]
        app_template_version = \
            project_client.list_app(id=appid).data[0].externalId
        assert mcapp_template_version == app_template_version, \
            "App Id is different from the Multi cluster app id"


def validate_deletion_mca(multiclusterapp):
    for i in range(0, len(multiclusterapp.targets)):
        app_id = multiclusterapp.targets[i].appId
        assert app_id is not None, "app_id is None"
        project_client = project_detail["p_client"+str(i)]
        app = multiclusterapp.targets[i].projectId.split(":")
        app1id = app[1] + ":" + multiclusterapp.targets[i].appId
        validate_app_deletion(project_client, app1id)


def validate_all_answer_override_mca(multiclusterapp):
    for i in range(0, len(multiclusterapp.targets)):
        project_client = project_detail["p_client" + str(i)]
        app = multiclusterapp.targets[0].projectId.split(":")
        appid = app[1] + ":" + multiclusterapp.targets[i].appId
        hold = multiclusterapp['answers'][0]
        val = hold["values"]
        app_answers = \
            project_client.list_app(id=appid).data[0].answers
        assert str(val) == str(app_answers), \
            "App answers are different than the Multi cluster answers"


def validate_answer_override(multiclusterapp, id,
                             answers_override, cluster=True):
    for i in range(0, len(multiclusterapp.targets)):
        project_client = project_detail["p_client"+str(i)]
        app_id = multiclusterapp.targets[i].appId
        target_project_id = multiclusterapp.targets[i].projectId
        target_clusterId = target_project_id.split(":")[0]
        app_answers = project_client.list_app(id=app_id).data[0].answers
        if not cluster:
            if target_project_id == id:
                assert answers_override["values"]["mysqlUser"] == \
                       app_answers.get("mysqlUser"), \
                       "Answers are not available on the expected project"
            else:
                assert app_answers.get("mysqlUser") == "admin", \
                    "answers should not have changed"
        else:
            if target_clusterId == id:
                assert answers_override["values"]["mysqlUser"] == \
                       app_answers.get("mysqlUser"), \
                       "Answers are not available on the expected project"
            else:
                assert app_answers.get("mysqlUser") == "admin", \
                    "answers should not have changed"


def verify_rbac_multiclusterapp_update(new_user,
                                       user_token,
                                       user_role,
                                       multiclusterapp,
                                       answer_values):
    client = get_client_for_token(user_token)
    answer_values_new = \
        answer_values["mysqlPassword"] = "test123"
    member_list = multiclusterapp["members"]
    print("Testing can_update")
    if user_role == "owner" or user_role == "member":
        update_mcapp_with_member(client, multiclusterapp, answer_values_new, member_list)
    else:
        try:
            mcapp = client.update(multiclusterapp,
                                  targets=multiclusterapp.targets,
                                  roles=PROJECT_ROLE,
                                  templateVersionId=MYSQL_TEMPLATE_VID_131,
                                  answers=[{"values": answer_values_new}],
                                  members=member_list)
        except ApiError as e:
            print("Error here: ", e)
            assert e.error.status == 500
            assert e.error.message == \
                "read-only members cannot update multiclusterapp"
    return mcapp


def verify_rbac_multiclusterapp_add_member(new_user,
                                           user_token,
                                           user_role,
                                           multiclusterapp,
                                           answer_values):
    client = get_client_for_token(user_token)
    member_list_new = multiclusterapp["members"]
    member = get_member_list("member", new_user)
    member_list_new.append(member)
    print("Testing can_add_user")
    if user_role == "owner":
        update_mcapp_with_member(
            client, multiclusterapp, answer_values, member_list_new
        )
    else:
        try:
            mcapp = client.update(multiclusterapp,
                                  targets=multiclusterapp.targets,
                                  roles=PROJECT_ROLE,
                                  templateVersionId=MYSQL_TEMPLATE_VID_131,
                                  answers=[{"values": answer_values}],
                                  members=member_list_new
                                  )
        except ApiError as e:
            print("Error: ", e)
            assert e.error.status == 500
            if user_role == "member":
                assert e.error.message == \
                    "only members with owner access can update members"
            elif user_role == "read_only":
                assert e.error.message == \
                    "read-only members cannot update multiclusterapp"
    return mcapp


def get_member_list(access, user):
    member = dict()
    member["accessType"] = access
    member["displayName"] = user["username"]
    member["displayType"] = user["type"]
    member["displayType"] = "member"
    member["userPrincipalId"] = user["principalIds"][0]
    return member


def get_user_by_role(role):
    if role == "owner":
        user = rbac_get_user_by_role(CLUSTER_OWNER)
    elif role == "member":
        user = rbac_get_user_by_role(PROJECT_OWNER)
    elif role == "read-only":
        user = rbac_get_user_by_role(PROJECT_MEMBER)
    return user


def create_mcapp_with_member(targets, answer_values, member):
    admin_client = get_admin_client()
    mcapp = admin_client.create_multiClusterApp(
        templateVersionId=MYSQL_TEMPLATE_VID_131,
        targets=targets,
        roles=PROJECT_ROLE,
        name=random_name(),
        answers=[{"values": answer_values}],
        members=[member])
    mcapp = wait_for_mcapp_to_active(admin_client, mcapp)
    return mcapp


def update_mcapp_with_member(client, multiclusterapp, answer_values, member):
    mcapp = client.update(multiclusterapp,
                          roles=PROJECT_ROLE,
                          templateVersionId=MYSQL_TEMPLATE_VID_131,
                          answers=[{"values": answer_values}],
                          members=member)
    mcapp = wait_for_mcapp_to_active(client, mcapp)
    return mcapp
