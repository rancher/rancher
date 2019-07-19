import pytest
from .common import * # NOQA

project = {}
project_detail = {"p1_id": None, "p2_id": None, "p3_id": None, "p_client": None, "namespace": None,
                  "cluster": None, "project": None}
global_client = {"client": None, "cluster_count": False}
answer_105version = {
    "values": {
        "defaultImage": "true",
        "externalDatabase.database": "",
        "externalDatabase.host": "",
        "externalDatabase.password": "",
        "externalDatabase.port": "3306",
        "externalDatabase.user": "",
        "image.repository": "bitnami/wordpress",
        "image.tag": "4.9.4",
        "ingress.enabled": "true",
        "ingress.hosts[0].name": "xip.io",
        "mariadb.enabled": "true",
        "mariadb.image.repository": "bitnami/mariadb",
        "mariadb.image.tag": "10.1.32",
        "mariadb.mariadbDatabase": "wordpress",
        "mariadb.mariadbPassword": "",
        "mariadb.mariadbUser": "wordpress",
        "mariadb.persistence.enabled": "false",
        "mariadb.persistence.existingClaim": "",
        "mariadb.persistence.size": "8Gi",
        "mariadb.persistence.storageClass": "",
        "nodePorts.http": "",
        "nodePorts.https": "",
        "persistence.enabled": "false",
        "persistence.existingClaim": "",
        "persistence.size": "10Gi",
        "persistence.storageClass": "",
        "serviceType": "NodePort",
        "wordpressEmail": "user@example.com",
        "wordpressPassword": "",
        "wordpressUsername": "user"
    }
}

answer = {
    "values": {
        "defaultImage": "true",
        "externalDatabase.database": "",
        "externalDatabase.host": "",
        "externalDatabase.password": "",
        "externalDatabase.port": "3306",
        "externalDatabase.user": "",
        "image.repository": "bitnami/wordpress",
        "image.tag": "4.9.8-debian-9",
        "ingress.enabled": "true",
        "ingress.hosts[0].name": "xip.io",
        "mariadb.db.name": "wordpress",
        "mariadb.db.user": "wordpress",
        "mariadb.enabled": "true",
        "mariadb.image.repository": "bitnami/mariadb",
        "mariadb.image.tag": "10.1.35-debian-9",
        "mariadb.mariadbPassword": "",
        "mariadb.master.persistence.enabled": "false",
        "mariadb.master.persistence.existingClaim": "",
        "mariadb.master.persistence.size": "8Gi",
        "mariadb.master.persistence.storageClass": "",
        "nodePorts.http": "",
        "nodePorts.https": "",
        "persistence.enabled": "false",
        "persistence.size": "10Gi",
        "persistence.storageClass": "",
        "serviceType": "NodePort",
        "wordpressEmail": "user@example.com",
        "wordpressPassword": "",
        "wordpressUsername": "user"
    }
}
new_answers = {
        "values": {
            "defaultImage": "true",
            "externalDatabase.database": "",
            "externalDatabase.host": "",
            "externalDatabase.password": "",
            "externalDatabase.port": "3306",
            "externalDatabase.user": "",
            "image.repository": "bitnami/wordpress",
            "image.tag": "4.9.8-debian-9",
            "ingress.enabled": "true",
            "ingress.hosts[0].name": "xip.io",
            "mariadb.db.name": "wordpress",
            "mariadb.db.user": "wordpress",
            "mariadb.enabled": "true",
            "mariadb.image.repository": "bitnami/mariadb",
            "mariadb.image.tag": "10.1.35-debian-9",
            "mariadb.mariadbPassword": "",
            "mariadb.master.persistence.enabled": "false",
            "mariadb.master.persistence.existingClaim": "",
            "mariadb.master.persistence.size": "8Gi",
            "mariadb.master.persistence.storageClass": "",
            "nodePorts.http": "",
            "nodePorts.https": "",
            "persistence.enabled": "false",
            "persistence.size": "10Gi",
            "persistence.storageClass": "",
            "serviceType": "NodePort",
            "wordpressEmail": "test_answers@example.com",
            "wordpressPassword": "",
            "wordpressUsername": "test_adding_answers"
        }
    }
ROLES = ["project-member"]
TEMP_VER = "cattle-global-data:library-wordpress-2.1.10"
UPGRADE = {'rollingUpdate': {'batchSize': 1, 'interval': 1, 'type': '/v3/schemas/rollingUpdate'}, 'type': '/v3/schemas/upgradeStrategy'}
original_rev = "cattle-global-data:library-wordpress-2.1.10"
new_ver = "cattle-global-data:library-wordpress-2.1.12"


def test_multi_cluster_project_member_create():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = global_client["client"]
    ROLES = ["project-member"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    targets=targets,
                                                    roles=ROLES,
                                                    name=random_name(),
                                                    answers=[answer])
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    # verify if this app is available in the cluster/project
    # validate_multi_cluster_app_cluster(multiclusterapp)
    delete_multi_cluster_app(multiclusterapp)


def test_multi_cluster_deploy_to_single_target():
    assert_if_valid_cluster_count()
    project_id = project_detail["p1_id"]
    targets = [{"projectId": project_id, "type": "target"}]
    client = global_client["client"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    roles=["cluster-owner"],
                                                    targets=targets,
                                                    name=random_name(),
                                                    answers=[answer],
                                                    )
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    uuid = multiclusterapp.uuid
    name = multiclusterapp.name
    assert len(client.list_multiClusterApp(uuid=uuid, name=name).data[0]["targets"]) == 1, "did not start with 1"


def test_multi_cluster_template_upgrade():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = global_client["client"]
    TEMP_VER = "cattle-global-data:library-wordpress-2.1.10"
    new_ver = "cattle-global-data:library-wordpress-2.1.12"
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    targets=targets,
                                                    roles=ROLES,
                                                    name=random_name(),
                                                    answers=[answer])
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    original_temp = multiclusterapp['templateVersionId']
    original_revId = multiclusterapp["status"]["revisionId"]
    multiclusterapp = client.update(multiclusterapp, roles=ROLES, templateVersionId=new_ver)
    new_temp = multiclusterapp['templateVersionId']
    new_revId = multiclusterapp["status"]["revisionId"]
    assert original_temp != new_temp, "template did not change"
    assert original_revId != new_revId, "revisionId did not change"


def test_multi_cluster_template_rollback():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = global_client["client"]
    TEMP_VER = "cattle-global-data:library-wordpress-2.1.10"
    new_ver = "cattle-global-data:library-wordpress-2.1.11"
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    targets=targets,
                                                    roles=ROLES,
                                                    name=random_name(),
                                                    answers=[answer])
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    original_temp = multiclusterapp['templateVersionId']
    original_revId = multiclusterapp["status"]["revisionId"]
    multiclusterapp = client.update(multiclusterapp, roles=ROLES, templateVersionId=new_ver)
    multiclusterapp = client.reload(multiclusterapp)
    new_temp = multiclusterapp['templateVersionId']
    new_revId = multiclusterapp["status"]["revisionId"]
    assert original_temp != new_temp, "template did not change"
    assert original_revId != new_revId, "revisionId did not change"
    client.action(obj=multiclusterapp, action_name='rollback', revisionId=original_revId)
    multiclusterapp = client.reload(multiclusterapp)
    assert multiclusterapp["templateVersionId"] == TEMP_VER, "did not rollback"
    assert original_revId == multiclusterapp["status"]["revisionId"], "revisionId did not rollback"


def test_multi_cluster_multi_targets_one_cluster():
    assert_if_valid_cluster_count()
    targets = []
    first_id = project_detail["p1_id"]
    targets.append({"projectId": first_id, "type": "target"})
    second_id = project_detail["p3_id"]
    targets.append({"projectId": second_id, "type": "target"})
    client = global_client["client"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    targets=targets,
                                                    roles=ROLES,
                                                    name=random_name(),
                                                    answers=[answer])
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    # verify if this app is available in the cluster/project
    assert len(multiclusterapp["targets"]) == 2, "did not add both targets"


def test_multi_cluster_project_member_update():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = global_client["client"]
    ROLES = ["project-member"]
    UPGRADE = {'rollingUpdate': {'batchSize': 1, 'interval': 1, 'type': '/v3/schemas/rollingUpdate'},
               'type': '/v3/schemas/upgradeStrategy'}
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    targets=targets,
                                                    roles=ROLES,
                                                    name=random_name(),
                                                    answers=[answer],
                                                    upgradeStrategy=UPGRADE)
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    expected_UPGRADE = {'rollingUpdate': {'batchSize': 1, 'interval': 1, 'type': '/v3/schemas/rollingUpdate'},
                        'type': '/v3/schemas/upgradeStrategy'}
    # validate_multi_cluster_app_cluster(multiclusterapp)
    assert str(multiclusterapp['upgradeStrategy']) == str(expected_UPGRADE), "incorrect update strategy"


def test_multi_cluster_project_owner_create():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = global_client["client"]
    ROLES = ["project-owner"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    targets=targets,
                                                    roles=ROLES,
                                                    name=random_name(),
                                                    answers=[answer])
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    # verify if this app is available in the cluster/project
    validate_multi_cluster_app_cluster(multiclusterapp)
    delete_multi_cluster_app(multiclusterapp)


def test_multi_cluster_project_owner_update():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = global_client["client"]
    ROLES = ["project-owner"]
    UPGRADE = {'rollingUpdate': {'batchSize': 1, 'interval': 1, 'type': '/v3/schemas/rollingUpdate'},
               'type': '/v3/schemas/upgradeStrategy'}
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    targets=targets,
                                                    roles=ROLES,
                                                    name=random_name(),
                                                    answers=[answer],
                                                    upgradeStrategy=UPGRADE)
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    expected_UPGRADE = {'rollingUpdate': {'batchSize': 1, 'interval': 1, 'type': '/v3/schemas/rollingUpdate'},
                        'type': '/v3/schemas/upgradeStrategy'}
    # validate_multi_cluster_app_cluster(multiclusterapp)
    assert str(multiclusterapp['upgradeStrategy']) == str(expected_UPGRADE), "incorrect update strategy"


def test_multi_cluster_upgrade():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = global_client["client"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    targets=targets,
                                                    roles=ROLES,
                                                    name=random_name(),
                                                    answers=[answer],
                                                    upgradeStrategy=UPGRADE)
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    expected_UPGRADE = {'rollingUpdate': {'batchSize': 1, 'interval': 1, 'type': '/v3/schemas/rollingUpdate'}, 'type': '/v3/schemas/upgradeStrategy'}
    # validate_multi_cluster_app_cluster(multiclusterapp)
    assert str(multiclusterapp['upgradeStrategy']) == str(expected_UPGRADE), "incorrect update strategy"


def test_multi_cluster_upgrade_and_edit_upgrade():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = global_client["client"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    targets=targets,
                                                    roles=ROLES,
                                                    name=random_name(),
                                                    answers=[answer],
                                                    upgradeStrategy=UPGRADE
                                                    )
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    expected_UPGRADE = {'rollingUpdate': {'batchSize': 1, 'interval': 1, 'type': '/v3/schemas/rollingUpdate'},
                        'type': '/v3/schemas/upgradeStrategy'}
    validate_multi_cluster_app_cluster(multiclusterapp)
    assert str(multiclusterapp['upgradeStrategy']) == str(expected_UPGRADE), "incorrect update strategy"
    new_upgrade = {'rollingUpdate': {'batchSize': 2, 'interval': 2, 'type': '/v3/schemas/rollingUpdate'}, 'type': '/v3/schemas/upgradeStrategy'}
    multiclusterapp = client.update(multiclusterapp, roles=ROLES, upgradeStrategy=new_upgrade)
    assert str(multiclusterapp['upgradeStrategy']) == str(new_upgrade)


def test_multi_cluster_upgrade_and_add_target():
    assert_if_valid_cluster_count()
    project_id = project_detail["p1_id"]
    new_targets = [{"projectId": project_id, "type": "target"}]
    project_id_2 = project_detail["p2_id"]
    client = global_client["client"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    roles=["cluster-owner"],
                                                    targets=new_targets,
                                                    name=random_name(),
                                                    answers=[answer],
                                                    )
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    uuid = multiclusterapp.uuid
    name = multiclusterapp.name
    assert len(client.list_multiClusterApp(uuid=uuid, name=name).data[0]["targets"]) == 1, "did not start with 1"
    new_upgrade = {'rollingUpdate': {'batchSize': 1, 'interval': 1, 'type': '/v3/schemas/rollingUpdate'},
                   'type': '/v3/schemas/upgradeStrategy'}
    validate_multi_cluster_app_cluster(multiclusterapp)
    assert len(client.list_multiClusterApp(uuid=uuid, name=name).data[0]["targets"]) == 1, "did not start with one"
    multiclusterapp = client.update(multiclusterapp, upgradeStrategy=new_upgrade, roles=ROLES)
    multiclusterapp = client.reload(multiclusterapp)
    client.action(obj=multiclusterapp, action_name="addProjects",
                  projects=[project_id_2])
    multiclusterapp = client.reload(multiclusterapp)
    assert len(multiclusterapp["targets"]) == 2, "did not add target"


def test_multi_cluster_upgrade_and_delete_target():
    assert_if_valid_cluster_count()
    project_id = project_detail["p1_id"]
    targets = []
    for project_id in project:
        targets.append({"projectId": project_id, "type": "target"})
    client = global_client["client"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    roles=["cluster-owner"],
                                                    targets=targets,
                                                    name=random_name(),
                                                    answers=[answer],
                                                    )
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    uuid = multiclusterapp.uuid
    name = multiclusterapp.name
    assert len(client.list_multiClusterApp(uuid=uuid, name=name).data[0]["targets"]) == 2, "did not start with 2"
    new_upgrade = {'rollingUpdate': {'batchSize': 1, 'interval': 1, 'type': '/v3/schemas/rollingUpdate'},
                   'type': '/v3/schemas/upgradeStrategy'}
    multiclusterapp = client.update(multiclusterapp, upgradeStrategy=new_upgrade, roles=ROLES)
    multiclusterapp = client.reload(multiclusterapp)
    client.action(obj=multiclusterapp, action_name="removeProjects",
                  projects=[project_id])
    multiclusterapp = client.reload(multiclusterapp)
    assert len(multiclusterapp["targets"]) == 1, "did not delete target"


def test_multi_cluster_upgrade_and_edit_answers():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = global_client["client"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    targets=targets,
                                                    roles=ROLES,
                                                    name=random_name(),
                                                    answers=[answer],
                                                    upgradeStrategy=UPGRADE
                                                    )
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    expected_UPGRADE = {'rollingUpdate': {'batchSize': 1, 'interval': 1, 'type': '/v3/schemas/rollingUpdate'},
                        'type': '/v3/schemas/upgradeStrategy'}
    validate_multi_cluster_app_cluster(multiclusterapp)
    assert str(multiclusterapp['upgradeStrategy']) == str(expected_UPGRADE), "incorrect update strategy"
    new_upgrade = {'rollingUpdate': {'batchSize': 2, 'interval': 2, 'type': '/v3/schemas/rollingUpdate'}, 'type': '/v3/schemas/upgradeStrategy'}
    multiclusterapp = client.update(multiclusterapp, roles=["cluster-owner"], answers=[new_answers], upgradeStrategy=new_upgrade)
    assert str(multiclusterapp['upgradeStrategy']) == str(new_upgrade)
    hold = multiclusterapp['answers'][0]
    val = hold["values"]
    assert str(val) == str(new_answers["values"])


def test_multi_cluster_add_target():
    assert_if_valid_cluster_count()
    project_id = project_detail["p1_id"]
    targets = [{"projectId": project_id, "type": "target"}]
    project_id_2 = project_detail["p2_id"]
    client = global_client["client"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    roles=["cluster-owner"],
                                                    targets=targets,
                                                    name=random_name(),
                                                    answers=[answer],
                                                    )
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    uuid = multiclusterapp.uuid
    name = multiclusterapp.name
    assert len(client.list_multiClusterApp(uuid=uuid, name=name).data[0]["targets"]) == 1, "did not start with 1"
    client.action(obj=multiclusterapp, action_name="addProjects",
                  projects=[project_id_2])
    multiclusterapp = client.reload(multiclusterapp)
    assert len(multiclusterapp["targets"]) == 2, "did not add target"


def test_multi_cluster_delete_target():
    assert_if_valid_cluster_count()
    project_id = project_detail["p1_id"]
    targets = []
    for project_id in project:
        targets.append({"projectId": project_id, "type": "target"})
    client = global_client["client"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    roles=["cluster-owner"],
                                                    targets=targets,
                                                    name=random_name(),
                                                    answers=[answer],
                                                    )
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    uuid = multiclusterapp.uuid
    name = multiclusterapp.name
    assert len(client.list_multiClusterApp(uuid=uuid, name=name).data[0]["targets"]) == 2, "did not start with 2"
    client.action(obj=multiclusterapp, action_name="removeProjects",
                  projects=[project_id])
    # multiclusterapp = client.update(multiclusterapp, targets=new_targets, roles=["cluster-owner"])
    multiclusterapp = client.reload(multiclusterapp)
    assert len(multiclusterapp["targets"]) == 1, "did not delete target"


def test_multi_cluster_role_change():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = global_client["client"]
    original_role = ["project-member"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    targets=targets,
                                                    roles=original_role,
                                                    name=random_name(),
                                                    answers=[answer])
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    # validate_multi_cluster_app_cluster(multiclusterapp)
    new_role = ["cluster-owner"]
    multiclusterapp = client.update(multiclusterapp, roles=new_role)
    start = time.time()
    while multiclusterapp['roles'] != new_role:
        if time.time() - start > 120:
            raise AssertionError(
             "Timed out waiting")
        time.sleep(10)
        if multiclusterapp['roles'] == new_role:
            break
    assert multiclusterapp['roles'] == new_role, "role did not update"


def test_multi_cluster_answers():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = global_client["client"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    targets=targets,
                                                    roles=ROLES,
                                                    name=random_name(),
                                                    answers=[new_answers])
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    hold = multiclusterapp['answers'][0]
    values = hold["values"]
    assert str(values) == str(new_answers["values"])


def test_multi_cluster_edit_answers():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = global_client["client"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    targets=targets,
                                                    roles=ROLES,
                                                    name=random_name(),
                                                    answers=[answer],
                                                    )
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    multiclusterapp = client.update(multiclusterapp, roles=ROLES, answers=[new_answers])
    hold = multiclusterapp['answers'][0]
    values = hold["values"]
    assert str(values) == str(new_answers["values"])


def test_multi_cluster_app_delete():
    assert_if_valid_cluster_count()
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    client = global_client["client"]
    multiclusterapp = client.create_multiClusterApp(templateVersionId=TEMP_VER,
                                                    targets=targets,
                                                    roles=ROLES,
                                                    name=random_name(),
                                                    answers=[answer])
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    validate_multi_cluster_app_cluster(multiclusterapp)
    delete_multi_cluster_app(multiclusterapp)
    for i in range(0, len(multiclusterapp.targets)):
        app_id = multiclusterapp.targets[i].appId
        assert app_id is not None, "app_id is None"
        project_client = \
            project[multiclusterapp.targets[i].projectId]["p_client"]
        wait_for_app_to_be_deleted_project(project_client, app_id)


def wait_for_app_to_be_deleted_project(client, app_id, timeout=DEFAULT_MULTI_CLUSTER_APP_TIMEOUT):
    app_data = client.list_app(name=app_id).data
    start = time.time()
    if len(app_data) == 0:
        return
    application = app_data[0]
    while application.state == "removing":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to delete")
        time.sleep(.5)
        app = client.list_app(name=app_id).data
        if len(app) == 0:
            break


def test_multi_cluster_app_edit():
    assert_if_valid_cluster_count()
    client = global_client["client"]
    targets = []
    for projectid in project:
        targets.append({"projectId": projectid, "type": "target"})
    temp_ver = "cattle-global-data:library-wordpress-1.0.5"
    multiclusterapp = client.create_multiClusterApp(templateVersionId=temp_ver,
                                                    targets=targets,
                                                    roles=ROLES,
                                                    name=random_name(),
                                                    answers=[answer_105version]
                                                    )
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    # verify if this app is available in the cluster/project
    validate_multi_cluster_app_cluster(multiclusterapp)
    temp_ver = "cattle-global-data:library-wordpress-2.1.10"
    multiclusterapp = client.update(multiclusterapp, uuid=multiclusterapp.uuid,
                                    templateVersionId=temp_ver,
                                    roles=ROLES,
                                    answers=[answer])
    multiclusterapp = wait_for_mcapp_to_active(client, multiclusterapp)
    # verify if this app is available in the cluster/project
    #check if correct field was changed
    validate_multi_cluster_app_cluster(multiclusterapp)
    delete_multi_cluster_app(multiclusterapp)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, clusters = get_admin_client_and_cluster_mcapp()
    if len(clusters) > 1:
        global_client["cluster_count"] = True
    assert_if_valid_cluster_count()
    cluster1 = clusters[0]
    cluster2 = clusters[1]
    p1, ns1 = create_project_and_ns(ADMIN_TOKEN, cluster1, "target_test_1")
    p_client1 = get_project_client_for_token(p1, ADMIN_TOKEN)
    p2, ns2 = create_project_and_ns(ADMIN_TOKEN, cluster2, "target_test_2")
    p_client2 = get_project_client_for_token(p2, ADMIN_TOKEN)
    p3, ns3 = create_project_and_ns(ADMIN_TOKEN, cluster1, "target_test_1")
    p_client3 = get_project_client_for_token(p3, ADMIN_TOKEN)
    project_detail["p1_id"] = p1.id
    project_detail["namespace"] = ns1
    project_detail["p_client"] = p_client1
    project_detail["cluster"] = cluster1
    project_detail["project"] = p1
    project[p1.id] = project_detail
    project_detail["namespace"] = ns2
    project_detail["p2_id"] = p2.id
    project_detail["p_client"] = p_client2
    project_detail["cluster"] = cluster2
    project_detail["project"] = p2
    project[p2.id] = project_detail
    project_detail["p3_id"] = p3.id
    project_detail["namespace"] = ns3
    project_detail["p_client"] = p_client3
    project_detail["cluster"] = cluster1
    project_detail["project"] = p3
    project[p3.id] = project_detail
    global_client["client"] = client

    def fin():
        client_admin = get_admin_client()
        # client_admin.delete(project[p1.id]["project"])
        # client_admin.delete(project[p2.id]["project"])
        # client_admin.delete(project[p3.id]["project"])
    request.addfinalizer(fin)


def assert_if_valid_cluster_count():
    assert global_client["cluster_count"], \
        "Setup Failure. Tests require atleast 2 clusters"


def validate_multi_cluster_app_cluster(multiclusterapp):
    for i in range(1, len(multiclusterapp.targets)):
        app_id = multiclusterapp.targets[i].appId
        assert app_id is not None, "app_id is None"
        project_client = \
            project[multiclusterapp.targets[i].projectId]["p_client"]
        wait_for_app_to_active(project_client, app_id)
        validate_app_version(project_client, multiclusterapp, app_id)
        validate_response_app_endpoint(project_client, app_id)


def get_admin_client_and_cluster_mcapp():
    clusters = []
    client = get_admin_client()
    if CLUSTER_NAME != "" and CLUSTER_NAME_2 != "":
        assert len(client.list_cluster(name=CLUSTER_NAME).data) != 0,\
            "Cluster is not available: %r" % CLUSTER_NAME
        assert len(client.list_cluster(name=CLUSTER_NAME_2).data) != 0,\
            "Cluster is not available: %r" % CLUSTER_NAME_2
        clusters.append(client.list_cluster(name=CLUSTER_NAME).data[0])
        clusters.append(client.list_cluster(name=CLUSTER_NAME_2).data[0])
    else:
        clusters = client.list_cluster().data
    return client, clusters


def delete_multi_cluster_app(multiclusterapp):
    client = global_client["client"]
    uuid = multiclusterapp.uuid
    name = multiclusterapp.name
    client.delete(multiclusterapp)
    mcapps = client.list_multiClusterApp(uuid=uuid, name=name).data
    assert len(mcapps) == 0, "Multi Cluster App is not deleted"


def validate_app_version(project_client, multiclusterapp, app_id):
    temp_version = multiclusterapp.templateVersionId
    app = temp_version.split(":")[1].split("-")
    mcapp_template_version = "catalog://?catalog=" + app[0] + \
                             "&template=" + app[1] + "&version=" + app[2]
    app_template_version = \
        project_client.list_app(name=app_id).data[0].externalId
    assert mcapp_template_version == app_template_version, \
        "App Id is different from the Multi cluster app id"
