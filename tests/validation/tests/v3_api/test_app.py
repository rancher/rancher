from .common import *


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

    p = admin_client.reload(p)
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
    answers = {
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
        "mariadb.persistence.size": "8Gi",
        "mariadb.persistence.storageClass": "",
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

    external_id = "catalog://?catalog=library&template=wordpress" \
                  "&version=1.0.5&namespace=cattle-global-data"
    app = proj_client.create_app(
        name=name,
        externalId=external_id,
        targetNamespace=ns.name,
        projectId=p.id,
        answers=answers
    )

    app = proj_client.reload(app)
    # test for tiller to be stuck on bad installs
    wait_for_condition(proj_client, app, check_condition('Installed', 'False'))
    # cleanup by deleting project
    admin_client.delete(p)
