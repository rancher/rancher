from .common import random_str
from .test_catalog import wait_for_template_to_be_created
import time


def test_app_mysql(admin_pc):
    client = admin_pc.client
    name = random_str()

    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=admin_pc.
                                                  project.id)
    answers = {
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
    client.create_app(
        name=name,
        externalId="catalog://?catalog=library&template=mysql&version=0.3.7",
        targetNamespace=ns.name,
        projectId=admin_pc.project.id,
        answers=answers
    )
    wait_for_workload(client, ns.name, count=1)


def test_app_wordpress(admin_pc):
    client = admin_pc.client
    name = random_str()

    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=admin_pc.
                                                  project.id)

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
    external_id = "catalog://?catalog=library&template=wordpress&version=1.0.5"
    client.create_app(
        name=name,
        externalId=external_id,
        targetNamespace=ns.name,
        projectId=admin_pc.project.id,
        answers=answers
    )
    wait_for_workload(client, ns.name, count=2)


def test_prehook_chart(admin_pc, admin_mc):
    client = admin_pc.client
    name = random_str()

    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=admin_pc.
                                                  project.id)
    url = "https://github.com/StrongMonkey/charts-1.git"
    catalog = admin_mc.client.create_catalog(name=random_str(),
                                             branch="test",
                                             url=url,
                                             )
    wait_for_template_to_be_created(admin_mc.client, catalog.name)
    external_id = "catalog://?catalog=" + \
                  catalog.name + "&template=busybox&version=0.0.2"
    client.create_app(
        name=name,
        externalId=external_id,
        targetNamespace=ns.name,
        projectId=admin_pc.project.id,
    )
    # it will be only one workload(job), because the deployment has to
    # wait for job to be finished, and it will never finish because we
    # can't create real container
    wait_for_workload(client, ns.name, count=1)
    jobs = client.list_job(namespaceId=ns.id)
    assert len(jobs) == 1


def test_app_namespace_annotation(admin_pc):
    client = admin_pc.client
    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=admin_pc.
                                                  project.id)
    app1 = client.create_app(
        name=random_str(),
        externalId="catalog://?catalog=library&template=mysql&version=0.3.7",
        targetNamespace=ns.name,
        projectId=admin_pc.project.id,
    )
    wait_for_workload(client, ns.name, count=1)

    external_id = "catalog://?catalog=library&template=wordpress&version=1.0.5"
    app2 = client.create_app(
        name=random_str(),
        externalId=external_id,
        targetNamespace=ns.name,
        projectId=admin_pc.project.id,
    )
    wait_for_workload(client, ns.name, count=3)
    ns = admin_pc.cluster.client.reload(ns)
    assert app1.name in ns.annotations.data_dict()['cattle.io/appIds']
    assert app2.name in ns.annotations.data_dict()['cattle.io/appIds']
    client.delete(app1)
    wait_for_app_to_be_deleted(client, app1)

    ns = admin_pc.cluster.client.reload(ns)
    assert app1.name not in ns.annotations.data_dict()['cattle.io/appIds']
    assert app2.name in ns.annotations.data_dict()['cattle.io/appIds']

    client.delete(app2)
    wait_for_app_to_be_deleted(client, app2)
    ns = admin_pc.cluster.client.reload(ns)
    assert 'cattle.io/appIds' not in ns.annotations.data_dict()


def wait_for_workload(client, ns, timeout=60, count=0):
    start = time.time()
    interval = 0.5
    workloads = client.list_workload(namespaceId=ns)
    while len(workloads.data) != count:
        workloads = client.list_workload(namespaceId=ns)
        time.sleep(interval)
        if time.time() - start > timeout:
            print(workloads)
            raise Exception('Timeout waiting for workload service')
        interval *= 2
    return workloads


def wait_for_app_to_be_deleted(client, app, timeout=120):
    start = time.time()
    interval = 0.5
    while True:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for apps to be deleted")
        apps = client.list_app()
        found = False
        for a in apps:
            if a.id == app.id:
                found = True
                break
        if not found:
            break
        time.sleep(interval)
        interval *= 2
