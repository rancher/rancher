import pytest
from .common import * # NOQA

project = {}
project_detail = {"p_client": None, "namespace": None,
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
ROLES = ["project-member"]
TEMP_VER = "cattle-global-data:library-wordpress-2.1.10"


def test_multi_cluster_app_create():
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
    # verify if this app is available in the cluster/project
    validate_multi_cluster_app_cluster(multiclusterapp)
    delete_multi_cluster_app(multiclusterapp)


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
    p1, ns1 = create_project_and_ns(ADMIN_TOKEN, cluster1, "test_mcapp1")
    p_client1 = get_project_client_for_token(p1, ADMIN_TOKEN)
    p2, ns2 = create_project_and_ns(ADMIN_TOKEN, cluster2, "test_mcapp2")
    p_client2 = get_project_client_for_token(p2, ADMIN_TOKEN)
    project_detail["namespace"] = ns1
    project_detail["p_client"] = p_client1
    project_detail["cluster"] = cluster1
    project_detail["project"] = p1
    project[p1.id] = project_detail
    project_detail["namespace"] = ns2
    project_detail["p_client"] = p_client2
    project_detail["cluster"] = cluster2
    project_detail["project"] = p2
    project[p2.id] = project_detail
    global_client["client"] = client

    def fin():
        client_admin = get_admin_client()
        client_admin.delete(project[p1.id]["project"])
        client_admin.delete(project[p2.id]["project"])
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
