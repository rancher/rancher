from .common import *

url = "https://git.rancher.io/charts"
mysql_answers = {
    "defaultImage": "true",
    "image": "mysql",
    "imageTag": "5.7.14",
    "mysqlDatabase": "admin",
    "mysqlPassword": "",
    "mysqlUser": "admin",
    "persistence.enabled": "false",
    "persistence.size": "8Gi",
    "persistence.storageClass": "",
    "service.nodePort": "",
    "service.port": "3306",
    "service.type": "ClusterIP"
}
wp_external_id = "catalog://?catalog=library&template=wordpress&version=2.1.12"
mysql_external_id = "catalog://?catalog=library&template=mysql&version=0.3.7"
answers = {
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
new_answers = {
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
    "wordpressUsername": "test1234"
}


def try_failure(cluster, externalId):
    try:
        p, ns = create_project_and_ns(
            ADMIN_TOKEN,
            cluster,
            "test-" + random_str())
        proj_client = get_project_client_for_token(p, ADMIN_TOKEN)
        app2 = proj_client.create_app(
            name=random_test_name(),
            externalId=externalId,
            answers=answers,
            targetNamespace=ns.name,
            projectId=p.id)
        wait_for_app_to_active(proj_client, app2.id)
    except Exception:
        print("expected failure: should not be able to access deleted catalog")
        pass  # expected fail


def wait_for_catalog_to_activate(catalog):
    admin_client = get_admin_client()
    catalog_state = catalog.state
    while catalog_state != "active":
        time.sleep(1)
        admin_client.reload(catalog)
        catalog_state = catalog.state


def test_app_delete_cluster_catalog():
    admin_client = get_admin_client()
    clusters = admin_client.list_cluster(name=CLUSTER_NAME).data
    assert len(clusters) > 0
    p, ns = create_project_and_ns(
        ADMIN_TOKEN,
        clusters[0],
        "test-" + random_str())

    client = get_admin_client()
    catalog = client.create_clusterCatalog(name="clustercatalog",
                                           baseType="clusterCatalog",
                                           branch="master",
                                           url=url,
                                           clusterId=clusters[0].id
                                           )
    time.sleep(5)
    proj_external_id = "catalog://?catalog=" + p.clusterId + \
                       "/clustercatalog&type=clusterCatalog&template=" \
                       "wordpress&version=2.1.12"
    proj_client = get_project_client_for_token(p, ADMIN_TOKEN)
    app = proj_client.create_app(
        name=random_test_name(),
        externalId=proj_external_id,
        answers=answers,
        targetNamespace=ns.name,
        projectId=p.id,
    )
    wait_for_app_to_active(proj_client, app.id)
    admin_client.delete(p, ns, proj_client)
    admin_client.delete(catalog)
    assert len(admin_client.list_clusterCatalog()["data"]) == 0
    try_failure(clusters[0], proj_external_id)


def test_app_delete_project_catalog():
    name = random_test_name()
    admin_client = get_admin_client()
    clusters = admin_client.list_cluster(name=CLUSTER_NAME).data
    assert len(clusters) > 0
    p, ns = create_project_and_ns(
        ADMIN_TOKEN,
        clusters[0],
        "test-" + random_str())
    client = get_admin_client()
    catalog = client.create_projectCatalog(name="projectcatalog",
                                           baseType="projectCatalog",
                                           branch="master",
                                           url=url,
                                           projectId=p.id,
                                           clusterId=clusters[0].id
                                           )
    time.sleep(5)
    pId = p.id.split(":")[1]
    proj_external_id = "catalog://?catalog=" + pId + \
                       "/projectcatalog&type=clusterCatalog&template=" \
                       "wordpress&version=2.1.12"
    proj_client = get_project_client_for_token(p, ADMIN_TOKEN)
    app = proj_client.create_app(
        name=name,
        externalId=proj_external_id,
        answers=answers,
        targetNamespace=ns.name,
        projectId=p.id,
    )
    wait_for_app_to_active(proj_client, app.id)
    admin_client.delete(p, ns, proj_client)
    admin_client.delete(catalog)
    assert len(admin_client.list_projectCatalog()["data"]) == 0
    try_failure(clusters[0], proj_external_id)


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


def test_app_tiller():
    name = random_test_name()
    admin_client = get_admin_client()
    clusters = admin_client.list_cluster(name=CLUSTER_NAME).data
    assert len(clusters) > 0
    cluster_id = clusters[0].id

    p = admin_client. \
        create_project(name="test-" + random_str(),
                       clusterId=cluster_id,
                       resourceQuota={
                           "limit": {
                               "secrets": "1"
                           }
                       },
                       namespaceDefaultResourceQuota={
                           "limit": {
                               "secrets": "1"
                           }
                       })

    proj_client = rancher.Client(url=p.links.self + '/schemas', verify=False,
                                 token=ADMIN_TOKEN)
    # need a cluster scoped client to create a namespace
    _cluster, cluster_client = cluster_and_client(cluster_id, admin_client)
    ns = cluster_client.create_namespace(name=random_str(),
                                         projectId=p.id,
                                         resourceQuota={
                                             "limit": {
                                                 "secrets": "1"
                                             }
                                         })
    wait_for_template_to_be_created(admin_client, "library")
    app = proj_client.create_app(
        name=name,
        externalId=wp_external_id,
        targetNamespace=ns.name,
        projectId=p.id,
        answers=answers
    )
    # test for tiller to be stuck on bad installs
    wait_for_condition(proj_client, app, check_condition('Installed', 'False'))
    # cleanup by deleting project
    admin_client.delete(p)


def test_app_deploy_cluster_scope():
    admin_client = get_admin_client()
    clusters = admin_client.list_cluster(name=CLUSTER_NAME).data
    assert len(clusters) > 0
    p, ns = create_project_and_ns(
        ADMIN_TOKEN,
        clusters[0],
        "test-" + random_str())
    p2, ns2 = create_project_and_ns(
        ADMIN_TOKEN,
        clusters[0],
        "test-" + random_str())
    projects = [[p, ns], [p2, ns2]]

    client = get_admin_client()
    catalog = client.create_clusterCatalog(name="clustercatalog",
                                           baseType="clusterCatalog",
                                           branch="master",
                                           url=url,
                                           clusterId=clusters[0].id
                                           )
    time.sleep(5)
    proj_external_id = "catalog://?catalog=" + p.clusterId + \
                       "/clustercatalog&type=clusterCatalog&template=" \
                       "wordpress&version=2.1.12"
    for proj in projects:
        proj_client = get_project_client_for_token(proj[0], ADMIN_TOKEN)
        app = proj_client.create_app(
            name=random_test_name(),
            externalId=proj_external_id,
            answers=answers,
            targetNamespace=proj[1].name,
            projectId=proj[0].id,
        )
        wait_for_app_to_active(proj_client, app.id)
    admin_client.delete(proj[0], proj[1], proj_client, catalog)


def test_app_deploy_project_scope():
    name = random_test_name()
    admin_client = get_admin_client()
    clusters = admin_client.list_cluster(name=CLUSTER_NAME).data
    assert len(clusters) > 0
    p, ns = create_project_and_ns(
        ADMIN_TOKEN,
        clusters[0],
        "test-" + random_str())
    proj_client = get_project_client_for_token(p, ADMIN_TOKEN)

    client = get_admin_client()
    catalog = client.create_projectCatalog(name="projectcatalog",
                                           baseType="projectCatalog",
                                           branch="master",
                                           url=url,
                                           projectId=p.id,
                                           clusterId=clusters[0].id
                                           )
    time.sleep(5)
    pId = p.id.split(":")[1]
    proj_external_id = "catalog://?catalog=" + pId + \
                       "/projectcatalog&type=clusterCatalog&template=" \
                       "wordpress&version=2.1.12"
    app = proj_client.create_app(
        name=name,
        externalId=proj_external_id,
        answers=answers,
        targetNamespace=ns.name,
        projectId=p.id,
    )

    wait_for_app_to_active(proj_client, app.id)
    p2, ns2 = create_project_and_ns(
        ADMIN_TOKEN,
        clusters[0],
        "test-" + random_str())
    proj_client2 = get_project_client_for_token(p2, ADMIN_TOKEN)
    proj_external_id2 = "catalog://?catalog=" + p2.id.split(":")[1] + \
                        "/projectcatalog&type=clusterCatalog&template=" \
                        "wordpress&version=2.1.12"
    try:
        app1 = proj_client2.create_app(
            name=name,
            externalId=proj_external_id2,
            answers=answers,
            targetNamespace=ns2.name,
            projectId=p2.id)
    except Exception:
        print("expected failure: other projects should not be able to access catalog")
        pass  # expected fail
    admin_client.delete(p, ns, proj_client, catalog)
    admin_client.delete(p2, ns2, proj_client2)


def test_app_deploy():
    name = random_test_name()
    admin_client = get_admin_client()
    clusters = admin_client.list_cluster(name=CLUSTER_NAME).data
    assert len(clusters) > 0
    p, ns = create_project_and_ns(
        ADMIN_TOKEN,
        clusters[0],
        "test-" + random_str())
    proj_client = get_project_client_for_token(p, ADMIN_TOKEN)
    wait_for_template_to_be_created(admin_client, "library")
    app = proj_client.create_app(
        name=name,
        externalId=wp_external_id,
        targetNamespace=ns.name,
        projectId=p.id,
        answers=answers
    )

    wait_for_app_to_active(proj_client, app.id)
    # cleanup by deleting project
    admin_client.delete(p, ns, proj_client)


def test_app_answer_override():
    name = random_test_name()
    admin_client = get_admin_client()
    clusters = admin_client.list_cluster(name=CLUSTER_NAME).data
    assert len(clusters) > 0
    p, ns = create_project_and_ns(
        ADMIN_TOKEN,
        clusters[0],
        "test-" + random_str())
    proj_client = get_project_client_for_token(p, ADMIN_TOKEN)
    wait_for_template_to_be_created(admin_client, "library")
    app = proj_client.create_app(
        name=name,
        externalId=wp_external_id,
        targetNamespace=ns.name,
        projectId=p.id,
        answers=answers
    )

    wait_for_app_to_active(proj_client, app.id)
    app = proj_client.update(
        obj=app,
        name=name,
        externalId=wp_external_id,
        targetNamespace=ns.name,
        projectId=p.id,
        answers=new_answers)

    assert app["answers"].get("wordpressUsername") == "test1234", \
        "incorrect answer upgrade"
    admin_client.delete(p, ns, proj_client)


def test_app_upgrade_version():
    name = random_test_name()
    admin_client = get_admin_client()
    clusters = admin_client.list_cluster(name=CLUSTER_NAME).data
    assert len(clusters) > 0
    p, ns = create_project_and_ns(
        ADMIN_TOKEN,
        clusters[0],
        "test-" + random_str())
    proj_client = get_project_client_for_token(p, ADMIN_TOKEN)
    wait_for_template_to_be_created(admin_client, "library")
    app = proj_client.create_app(
        name=name,
        externalId=mysql_external_id,
        targetNamespace=ns.name,
        projectId=p.id,
        answers=mysql_answers
    )

    wait_for_app_to_active(proj_client, app.id)
    app = proj_client.update(
        obj=app,
        name=name,
        externalId="catalog://?catalog=library&template="
                   "mysql&version=0.3.8",
        targetNamespace=ns.name,
        projectId=p.id,
        answers=mysql_answers)

    assert app.externalId == "catalog://?catalog=library&template=" \
                             "mysql&version=0.3.8", \
        "incorrect template version"
    admin_client.delete(p, ns, proj_client)


def test_app_rollback():
    name = random_test_name()
    admin_client = get_admin_client()
    clusters = admin_client.list_cluster(name=CLUSTER_NAME).data
    assert len(clusters) > 0
    p, ns = create_project_and_ns(
        ADMIN_TOKEN,
        clusters[0],
        "test-" + random_str())
    proj_client = get_project_client_for_token(p, ADMIN_TOKEN)
    wait_for_template_to_be_created(admin_client, "library")
    app = proj_client.create_app(
        name=name,
        externalId=mysql_external_id,
        targetNamespace=ns.name,
        projectId=p.id,
        answers=mysql_answers
    )
    app = proj_client.reload(app)
    wait_for_app_to_active(proj_client, app.id)
    app = proj_client.update(
        obj=app,
        name=name,
        externalId="catalog://?catalog=library&template="
                   "mysql&version=0.3.8",
        targetNamespace=ns.name,
        projectId=p.id,
        answers=mysql_answers)
    app = proj_client.reload(app)
    wait_for_app_to_active(proj_client, app.id)
    assert app.externalId == "catalog://?catalog=library&template=" \
                             "mysql&version=0.3.8",\
        "incorrect template version"
    rev_id = app.appRevisionId
    proj_client.action(obj=app,
                       action_name='rollback',
                       revisionId=rev_id)
    app = proj_client.reload(app)
    assert app.externalId == "catalog://?catalog=library&template=" \
                             "mysql&version=0.3.7", \
        "incorrect template version"
    admin_client.delete(p, ns, proj_client)


def test_app_delete():
    name = random_test_name()
    admin_client = get_admin_client()
    clusters = admin_client.list_cluster(name=CLUSTER_NAME).data
    assert len(clusters) > 0
    p, ns = create_project_and_ns(
        ADMIN_TOKEN,
        clusters[0],
        "test-" + random_str())
    proj_client = get_project_client_for_token(p, ADMIN_TOKEN)
    wait_for_template_to_be_created(admin_client, "library")
    app = proj_client.create_app(
        name=name,
        externalId=wp_external_id,
        targetNamespace=ns.name,
        projectId=p.id,
        answers=answers
    )

    wait_for_app_to_active(proj_client, app.id)
    admin_client.delete(app, p, ns, proj_client)
    validate_app_deletion(proj_client, app.id, 120)


def validate_app_deletion(client, app_id, timeout=DEFAULT_MULTI_CLUSTER_APP_TIMEOUT):
    app_data = client.list_app(id=app_id).data
    start = time.time()
    if len(app_data) == 0:
        return
    application = app_data[0]
    while application.state == "removing":
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for app to delete")
        time.sleep(.5)
        app = client.list_app(id=app_id).data
        if len(app) == 0:
            break


def wait_for_app_to_active(client, app_id,
                           timeout=DEFAULT_MULTI_CLUSTER_APP_TIMEOUT):
    start = time.time()
    while True:
        app_data = client.list_app(id=app_id).data
        if len(app_data) == 1:
            application = app_data[0]
            if application.state == "active":
                return application
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for state to get to active")
        time.sleep(.5)