import time
import pytest
from rancher import ApiError
from .test_catalog import wait_for_template_to_be_created
from .common import random_str
from .conftest import set_server_version, wait_for


def test_app_mysql(admin_pc, admin_mc):
    client = admin_pc.client
    name = random_str()

    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=admin_pc.
                                                  project.id)
    wait_for_template_to_be_created(admin_mc.client, "library")
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
        externalId="catalog://?catalog=library&template=mysql&version=1.3.1&"
                   "namespace=cattle-global-data",
        targetNamespace=ns.name,
        projectId=admin_pc.project.id,
        answers=answers
    )
    wait_for_workload(client, ns.name, count=1)


def test_app_wordpress(admin_pc, admin_mc):
    client = admin_pc.client
    name = random_str()

    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=admin_pc.
                                                  project.id)
    wait_for_template_to_be_created(admin_mc.client, "library")
    answers = {
        "defaultImage": "true",
        "externalDatabase.database": "",
        "externalDatabase.host": "",
        "externalDatabase.password": "",
        "externalDatabase.port": "3306",
        "externalDatabase.user": "",
        "image.repository": "bitnami/wordpress",
        "image.tag": "5.2.3",
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
                  "&version=7.3.8&namespace=cattle-global-data"
    client.create_app(
        name=name,
        externalId=external_id,
        targetNamespace=ns.name,
        projectId=admin_pc.project.id,
        answers=answers
    )
    wait_for_workload(client, ns.name, count=2)


@pytest.mark.skip(reason="istio disabled")
def test_app_istio(admin_cc, admin_pc, admin_mc):
    client = admin_pc.client
    name = "rancher-istio"
    url = "	https://github.com/guangbochen/system-charts.git"
    external_id = "catalog://?catalog=system-library" \
                  "&template=rancher-istio&version=1.1.5"

    ns = admin_pc.cluster.client.create_namespace(name="istio-system",
                                                  projectId=admin_pc.
                                                  project.id)
    admin_mc.client.create_catalog(name="system-library",
                                   branch="istio",
                                   url=url,
                                   )
    wait_for_template_to_be_created(admin_mc.client, "system-library")

    answers = {
        "certmanager.enabled": "false",
        "enableCRDs": "true",
        "galley.enabled": "true",
        "gateways.enabled": "false",
        "gateways.istio-ingressgateway.type": "NodePort",
        "grafana.enabled": "true",
        "grafana.persistence.enabled": "false",
        "istio_cni.enabled": "false",
        "istiocoredns.enabled": "false",
        "kiali.enabled": "true",
        "mixer.enabled": "true",
        "mixer.policy.enabled": "false",
        "mixer.telemetry.resources.limits.cpu": "4800m",
        "mixer.telemetry.resources.limits.memory": "4048Mi",
        "mixer.telemetry.resources.requests.cpu": "1000m",
        "mixer.telemetry.resources.requests.memory": "1024Mi",
        "mtls.enabled": "false",
        "nodeagent.enabled": "false",
        "pilot.enabled": "true",
        "pilot.resources.limits.cpu": "1000m",
        "pilot.resources.limits.memory": "4096Mi",
        "pilot.resources.requests.cpu": "500m",
        "pilot.resources.requests.memory": "2048Mi",
        "pilot.traceSampling": "1",
        "prometheus.enabled": "true",
        "prometheus.resources.limits.cpu": "1000m",
        "prometheus.resources.limits.memory": "1000Mi",
        "prometheus.resources.requests.cpu": "750m",
        "prometheus.resources.requests.memory": "750Mi",
        "prometheus.retention": "6h",
        "security.enabled": "true",
        "sidecarInjectorWebhook.enabled": "true",
        "tracing.enabled": "true"
    }

    client.create_app(
        name=name,
        externalId=external_id,
        targetNamespace=ns.name,
        projectId=admin_pc.project.id,
        answers=answers
    )
    wait_for_monitor_metric(admin_cc, admin_mc)


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
                  catalog.name + "&template=busybox&version=0.0.2" \
                                 "&namespace=cattle-global-data"
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


def test_app_namespace_annotation(admin_pc, admin_mc):
    client = admin_pc.client
    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=admin_pc.
                                                  project.id)
    wait_for_template_to_be_created(admin_mc.client, "library")
    app1 = client.create_app(
        name=random_str(),
        externalId="catalog://?catalog=library&template=mysql&version=1.3.1"
                   "&namespace=cattle-global-data",
        targetNamespace=ns.name,
        projectId=admin_pc.project.id,
    )
    wait_for_workload(client, ns.name, count=1)

    external_id = "catalog://?catalog=library&template=wordpress" \
                  "&version=7.3.8&namespace=cattle-global-data"
    app2 = client.create_app(
        name=random_str(),
        externalId=external_id,
        targetNamespace=ns.name,
        projectId=admin_pc.project.id,
    )
    wait_for_workload(client, ns.name, count=3)
    ns = admin_pc.cluster.client.reload(ns)
    ns = wait_for_app_annotation(admin_pc, ns, app1.name)
    ns = wait_for_app_annotation(admin_pc, ns, app2.name)
    client.delete(app1)
    wait_for_app_to_be_deleted(client, app1)

    ns = wait_for_app_annotation(admin_pc, ns, app1.name, exists=False)
    assert app1.name not in ns.annotations['cattle.io/appIds']
    assert app2.name in ns.annotations['cattle.io/appIds']

    client.delete(app2)
    wait_for_app_to_be_deleted(client, app2)

    ns = wait_for_app_annotation(admin_pc, ns, app2.name, exists=False)
    assert app2.name not in ns.annotations.get('cattle.io/appIds', [])


def test_helm_timeout(admin_pc, admin_mc, remove_resource):
    """Test helm timeout flag. This test asserts timeout flag is properly being
    passed to helm.
    """
    client = admin_pc.client
    ns = admin_pc.cluster.client.create_namespace(name="ns-" + random_str(),
                                                  projectId=admin_pc.
                                                  project.id)
    remove_resource(ns)

    wait_for_template_to_be_created(admin_mc.client, "library")

    # timeout of one second is not sufficient for installing mysql and should
    # result in failure
    app1 = client.create_app(
        name="app-" + random_str(),
        externalId="catalog://?catalog=library&template=mysql&version=1.3.1&"
                   "namespace=cattle-global-data",
        targetNamespace=ns.name,
        projectId=admin_pc.project.id,
        wait=True,
        timeout=1,
    )
    remove_resource(app1)

    assert app1.timeout == 1
    assert app1.wait

    wait_for_workload(client, ns.name, count=1)

    def wait_for_transition_error(app):
        def transition_error():
            test_app = client.reload(app)
            if test_app.transitioning != "error":
                return False
            return test_app

        return wait_for(transition_error, timeout=15, fail_handler=lambda:
                        "expected transitioning to fail")

    app1 = wait_for_transition_error(app1)
    assert "timed out waiting for the condition" in app1.transitioningMessage


def wait_for_app_annotation(admin_pc, ns, app_name, exists=True, timeout=60):
    start = time.time()
    interval = 0.5
    ns = admin_pc.cluster.client.reload(ns)
    while (app_name in ns.annotations.get('cattle.io/appIds', [])) != exists:
        if time.time() - start > timeout:
            print(ns.annotations)
            raise Exception('Timeout waiting for app annotation')
        time.sleep(interval)
        interval *= 2
        ns = admin_pc.cluster.client.reload(ns)
    return ns


def test_app_custom_values_file(admin_pc, admin_mc):
    client = admin_pc.client
    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=admin_pc.
                                                  project.id)
    wait_for_template_to_be_created(admin_mc.client, "library")
    values_yaml = "replicaCount: 2\r\nimage:\r\n  " \
                  "repository: registry\r\n  tag: 2.7"
    answers = {
        "image.tag": "2.6"
    }
    app = client.create_app(
        name=random_str(),
        externalId="catalog://?catalog=library&template=docker-registry"
                   "&version=1.8.1&namespace=cattle-global-data",
        targetNamespace=ns.name,
        projectId=admin_pc.project.id,
        valuesYaml=values_yaml,
        answers=answers
    )
    workloads = wait_for_workload(client, ns.name, count=1)
    workloads = wait_for_replicas(client, ns.name, count=2)
    print(workloads)
    assert workloads.data[0].deploymentStatus.unavailableReplicas == 2
    assert workloads.data[0].containers[0].image == "registry:2.6"
    client.delete(app)
    wait_for_app_to_be_deleted(client, app)


@pytest.mark.nonparallel
def test_app_create_validation(admin_mc, admin_pc, custom_catalog,
                               remove_resource, restore_rancher_version):
    """Test create validation for apps. This test will set the rancher version
    explicitly and attempt to create apps with rancher version requirements.
    """
    # 2.3.1 uses 2.4.1-2.6.0
    # 2.7.0 uses 2.5.0-2.7.0
    client = admin_mc.client

    c_name = random_str()
    custom_catalog(name=c_name)

    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=admin_pc.
                                                  project.id)
    remove_resource(ns)

    cat_base = "catalog://?catalog="+c_name+"&template=chartmuseum&version="

    app_data = {
        'name': random_str(),
        'externalId': cat_base+"2.7.0",
        'targetNamespace': ns.name,
        'projectId': admin_pc.project.id,
        "answers": [{
            "type": "answer",
            "clusterId": None,
            "projectId": None,
            "values": {
                "defaultImage": "true",
                "image.repository": "chartmuseum/chartmuseum",
                "image.tag": "v0.11.0",
                "env.open.STORAGE": "local",
                "gcp.secret.enabled": "false",
                "gcp.secret.key": "credentials.json",
                "persistence.enabled": "true",
                "persistence.size": "10Gi",
                "ingress.enabled": "true",
                "ingress.hosts[0]": "xip.io",
                "service.type": "NodePort",
                "env.open.SHOW_ADVANCED": "false",
                "env.open.DEPTH": "0",
                "env.open.ALLOW_OVERWRITE": "false",
                "env.open.AUTH_ANONYMOUS_GET": "false",
                "env.open.DISABLE_METRICS": "true"
            }
        }]
    }

    set_server_version(client, "2.4.2-beta2")

    # First try requires a min of 2.5.0 so an error should be returned
    with pytest.raises(ApiError) as e:
        app1 = admin_pc.client.create_app(app_data)
        remove_resource(app1)
    assert e.value.error.status == 422
    assert e.value.error.message == 'rancher min version not met'

    set_server_version(client, "2.7.1")

    # Second try requires a max of 2.7.0 so an error should be returned
    with pytest.raises(ApiError) as e:
        app1 = admin_pc.client.create_app(app_data)
        remove_resource(app1)
    assert e.value.error.status == 422
    assert e.value.error.message == 'rancher max version exceeded'

    set_server_version(client, "2.5.1-rc4")

    # Third try should work
    app1 = admin_pc.client.create_app(app_data)
    remove_resource(app1)
    wait_for_workload(admin_pc.client, ns.name, count=1)


@pytest.mark.nonparallel
def test_app_update_validation(admin_mc, admin_pc, custom_catalog,
                               remove_resource, restore_rancher_version):
    """Test update validation for apps. This test will set the rancher version
    explicitly and attempt to update apps with rancher version requirements.
    """
    # 2.3.1 uses 2.4.1-2.6.0
    # 2.7.0 uses 2.5.0-2.7.0
    client = admin_mc.client

    c_name = random_str()
    custom_catalog(name=c_name)

    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=admin_pc.
                                                  project.id)
    remove_resource(ns)

    cat_base = "catalog://?catalog="+c_name+"&template=chartmuseum&version="

    app_data = {
        'name': random_str(),
        'externalId': cat_base+"2.3.1",
        'targetNamespace': ns.name,
        'projectId': admin_pc.project.id,
        "answers": [{
            "type": "answer",
            "clusterId": None,
            "projectId": None,
            "values": {
                "defaultImage": "true",
                "image.repository": "chartmuseum/chartmuseum",
                "image.tag": "v0.9.0",
                "env.open.STORAGE": "local",
                "gcp.secret.enabled": "false",
                "gcp.secret.key": "credentials.json",
                "persistence.enabled": "true",
                "persistence.size": "10Gi",
                "ingress.enabled": "true",
                "ingress.hosts[0]": "xip.io",
                "service.type": "NodePort",
                "env.open.SHOW_ADVANCED": "false",
                "env.open.DEPTH": "0",
                "env.open.ALLOW_OVERWRITE": "false",
                "env.open.AUTH_ANONYMOUS_GET": "false",
                "env.open.DISABLE_METRICS": "true"
            }
        }]
    }

    set_server_version(client, "2.4.2-rc3")

    # Launch the app version 2.3.1 with rancher 2.4.2-rc3
    app1 = admin_pc.client.create_app(app_data)
    remove_resource(app1)
    wait_for_workload(admin_pc.client, ns.name, count=1)

    upgrade_dict = {
        'obj': app1,
        'action_name': 'upgrade',
        'answers': app_data['answers'],
        'externalId': cat_base+"2.7.0",
        'forceUpgrade': False,
    }

    # Attempt to upgrade, app version 2.7.0 requires a min of 2.5.0 so this
    # will error
    with pytest.raises(ApiError) as e:
        app1 = client.action(**upgrade_dict)
    assert e.value.error.status == 422
    assert e.value.error.message == 'rancher min version not met'

    set_server_version(client, "2.7.1")

    # # Second try requires a max of 2.7.0 so an error should be returned
    with pytest.raises(ApiError) as e:
        app1 = client.action(**upgrade_dict)
    assert e.value.error.status == 422
    assert e.value.error.message == 'rancher max version exceeded'


@pytest.mark.nonparallel
def test_app_rollback_validation(admin_mc, admin_pc, custom_catalog,
                                 remove_resource, restore_rancher_version):
    """Test rollback validation for apps. This test will set the rancher version
    explicitly and attempt to rollback apps with rancher version requirements.
    """
    # 2.3.1 uses 2.4.1-2.6.0
    # 2.7.0 uses 2.5.0-2.7.0
    client = admin_mc.client

    c_name = random_str()
    custom_catalog(name=c_name)

    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=admin_pc.
                                                  project.id)
    remove_resource(ns)

    cat_base = "catalog://?catalog="+c_name+"&template=chartmuseum&version="

    app_data = {
        'name': random_str(),
        'externalId': cat_base+"2.3.1",
        'targetNamespace': ns.name,
        'projectId': admin_pc.project.id,
        "answers": [{
            "type": "answer",
            "clusterId": None,
            "projectId": None,
            "values": {
                "defaultImage": "true",
                "image.repository": "chartmuseum/chartmuseum",
                "image.tag": "v0.9.0",
                "env.open.STORAGE": "local",
                "gcp.secret.enabled": "false",
                "gcp.secret.key": "credentials.json",
                "persistence.enabled": "true",
                "persistence.size": "10Gi",
                "ingress.enabled": "true",
                "ingress.hosts[0]": "xip.io",
                "service.type": "NodePort",
                "env.open.SHOW_ADVANCED": "false",
                "env.open.DEPTH": "0",
                "env.open.ALLOW_OVERWRITE": "false",
                "env.open.AUTH_ANONYMOUS_GET": "false",
                "env.open.DISABLE_METRICS": "true"
            }
        }]
    }

    set_server_version(client, "2.5.0")

    # Launch the app version 2.3.1 with rancher 2.4.2
    app1 = admin_pc.client.create_app(app_data)
    remove_resource(app1)
    wait_for_workload(admin_pc.client, ns.name, count=1)

    def _app_revision():
        app = admin_pc.client.reload(app1)
        return app.appRevisionId is not None

    wait_for(_app_revision, fail_handler=lambda: 'app has no revision')

    app1 = admin_pc.client.reload(app1)

    assert app1.appRevisionId is not None, 'app has no revision'

    original_rev = app1.appRevisionId

    upgrade_dict = {
        'obj': app1,
        'action_name': 'upgrade',
        'answers': app_data['answers'],
        'externalId': cat_base+"2.7.0",
        'forceUpgrade': False,
    }

    # Upgrade the app to get a rollback revision
    client.action(**upgrade_dict)

    def _app_revisions():
        app = admin_pc.client.reload(app1)
        return len(app.revision().data) > 1

    wait_for(_app_revisions, fail_handler=lambda: 'app did not upgrade')

    app1 = admin_pc.client.reload(app1)

    assert app1.appRevisionId != original_rev, 'app did not upgrade'

    rollback_dict = {
        'obj': app1,
        'action_name': 'rollback',
        'revisionId': original_rev,
        'forceUpgrade': False,
    }

    set_server_version(client, "2.6.1")

    # Rollback requires a max of 2.6.0 so an error should be returned
    with pytest.raises(ApiError) as e:
        client.action(**rollback_dict)
    assert e.value.error.status == 422
    assert e.value.error.message == 'rancher max version exceeded'

    set_server_version(client, "2.0.0-rc3")

    # Second try requires a min of 2.4.1 so an error should be returned
    with pytest.raises(ApiError) as e:
        client.action(**rollback_dict)

    msg = e.value.error
    assert e.value.error.message == 'rancher min version not met', msg
    assert e.value.error.status == 422


def wait_for_workload(client, ns, timeout=60, count=0):
    start = time.time()
    interval = 0.5
    workloads = client.list_workload(namespaceId=ns)
    while len(workloads.data) < count:
        if time.time() - start > timeout:
            print(workloads)
            raise Exception('Timeout waiting for workload service')
        time.sleep(interval)
        interval *= 2
        workloads = client.list_workload(namespaceId=ns)
    return workloads


def wait_for_replicas(client, ns, timeout=60, count=0):
    start = time.time()
    interval = 0.5
    workloads = client.list_workload(namespaceId=ns)
    while workloads.data[0].deploymentStatus.replicas != count:
        if time.time() - start > timeout:
            print(workloads)
            raise Exception('Timeout waiting for workload replicas')
        time.sleep(interval)
        interval *= 2
        workloads = client.list_workload(namespaceId=ns)
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


def wait_for_monitor_metric(admin_cc, admin_mc, timeout=60):
    client = admin_mc.client
    start = time.time()
    interval = 0.5
    monitorMetrics = client.list_monitor_metric(namespaceId=admin_cc.
                                                cluster.id)
    while len(monitorMetrics.data) == 0:
        if time.time() - start > timeout:
            print(monitorMetrics)
            raise Exception('Timeout waiting for monitorMetrics service')
        time.sleep(interval)
        interval *= 2
        monitorMetrics = client.list_monitor_metric(namespaceId=admin_cc.
                                                    cluster.id)
    found = False
    for m in monitorMetrics:
        if m.labels.component == "istio":
            found = True
            break
    if not found:
        raise AssertionError(
            "not found istio expression")
