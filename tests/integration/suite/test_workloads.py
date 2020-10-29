from .common import random_str
from rancher import ApiError
from .conftest import wait_for, wait_until_available, user_project_client

import time
import pytest


def test_workload_image_change_private_registry(admin_pc):
    client = admin_pc.client

    registry1_name = random_str()
    registries = {'index.docker.io': {
        'username': 'testuser',
                    'password': 'foobarbaz',
    }}
    registry1 = client.create_dockerCredential(name=registry1_name,
                                               registries=registries)
    assert registry1.name == registry1_name

    registry2_name = random_str()
    registries = {'quay.io': {
        'username': 'testuser',
                    'password': 'foobarbaz',
    }}
    registry2 = client.create_dockerCredential(name=registry2_name,
                                               registries=registries)

    ns = admin_pc.cluster.client.create_namespace(
        name=random_str(),
        projectId=admin_pc.project.id)
    name = random_str()
    workload = client.create_workload(
        name=name,
        namespaceId=ns.id,
        scale=1,
        containers=[{
            'name': 'one',
            'image': 'testuser/testimage',
        }])

    assert workload.name == name
    assert len(workload.imagePullSecrets) == 1
    for secret in workload.imagePullSecrets:
        assert secret['name'] == registry1_name

    containers = [{
        'name': 'one',
        'image': 'quay.io/testuser/testimage',
    }]

    workload = client.update(workload, containers=containers)

    for container in workload.containers:
        assert container['image'] == 'quay.io/testuser/testimage'

    assert len(workload.imagePullSecrets) == 1

    assert workload.imagePullSecrets[0]['name'] == registry2_name

    client.delete(registry1)
    client.delete(registry2)
    client.delete(ns)


def test_workload_ports_change(admin_pc):
    client = admin_pc.client
    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=admin_pc.
                                                  project.id)
    # create workload with no ports assigned
    # and verify headless service is created
    name = random_str()
    workload = client.create_workload(
        name=name,
        namespaceId=ns.id,
        scale=1,
        containers=[{
            'name': 'one',
            'image': 'nginx',
        }])
    svc = wait_for_service_create(client, name)
    assert svc.clusterIp is None
    assert svc.name == workload.name
    assert svc.kind == "ClusterIP"

    # update workload with port, and validate cluster ip is set
    ports = [{
        'sourcePort': '0',
        'containerPort': '80',
        'kind': 'ClusterIP',
        'protocol': 'TCP', }]
    client.update(workload,
                  namespaceId=ns.id,
                  scale=1,
                  containers=[{
                      'name': 'one',
                      'image': 'nginx',
                      'ports': ports,
                  }]),
    svc = wait_for_service_cluserip_set(client, name)
    assert svc.clusterIp is not None

    # update workload with no ports, and validate cluster ip is reset
    client.update(workload,
                  namespaceId=ns.id,
                  scale=1,
                  containers=[{
                      'name': 'one',
                      'image': 'nginx',
                      'ports': [],
                  }]),
    svc = wait_for_service_cluserip_reset(client, name)
    assert svc.clusterIp is None

    client.delete(ns)


def test_workload_probes(admin_pc):
    client = admin_pc.client
    ns = admin_pc.cluster.client.create_namespace(
        name=random_str(),
        projectId=admin_pc.project.id)
    # create workload with probes
    name = random_str()
    container = {
            'name': 'one',
            'image': 'nginx',
            'livenessProbe': {
              'failureThreshold': 3,
              'initialDelaySeconds': 10,
              'periodSeconds': 2,
              'successThreshold': 1,
              'tcp': False,
              'timeoutSeconds': 2,
              'host': 'localhost',
              'path': '/healthcheck',
              'port': 80,
              'scheme': 'HTTP',
            },
            'readinessProbe': {
              'failureThreshold': 3,
              'initialDelaySeconds': 10,
              'periodSeconds': 2,
              'successThreshold': 1,
              'timeoutSeconds': 2,
              'tcp': True,
              'host': 'localhost',
              'port': 80,
            },
        }
    workload = client.create_workload(name=name,
                                      namespaceId=ns.id,
                                      scale=1,
                                      containers=[container])
    assert workload.containers[0].livenessProbe.host == 'localhost'
    assert workload.containers[0].readinessProbe.host == 'localhost'
    container['livenessProbe']['host'] = 'updatedhost'
    container['readinessProbe']['host'] = 'updatedhost'
    workload = client.update(workload,
                             namespaceId=ns.id,
                             scale=1,
                             containers=[container])
    assert workload.containers[0].livenessProbe.host == 'updatedhost'
    assert workload.containers[0].readinessProbe.host == 'updatedhost'
    client.delete(ns)


def test_workload_scheduling(admin_pc):
    client = admin_pc.client
    ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                  projectId=admin_pc.
                                                  project.id)
    name = random_str()
    workload = client.create_workload(
        name=name,
        namespaceId=ns.id,
        scale=1,
        scheduling={
            "scheduler": "some-scheduler",
        },
        containers=[{
            'name': 'one',
            'image': 'nginx',
        }])
    assert workload.scheduling.scheduler == "some-scheduler"
    workload = client.update(workload,
                             namespaceId=ns.id,
                             scale=1,
                             scheduling={
                                 "scheduler": "test-scheduler",
                             },
                             containers=[{
                                 'name': 'one',
                                 'image': 'nginx',
                             }]),
    assert workload[0].scheduling.scheduler == "test-scheduler"
    client.delete(ns)


def test_statefulset_workload_volumemount_subpath(admin_pc):
    client = admin_pc.client
    # setup
    name = random_str()

    # valid volumeMounts
    volumeMounts = [{
        'name': 'vol1',
        'mountPath': 'var/lib/mysql',
        'subPath': 'mysql',
    }]

    containers = [{
        'name': 'mystatefulset',
        'image': 'ubuntu:xenial',
        'volumeMounts': volumeMounts,
    }]

    # invalid volumeMounts
    volumeMounts_one = [{
        'name': 'vol1',
        'mountPath': 'var/lib/mysql',
        'subPath': '/mysql',
    }]

    containers_one = [{
        'name': 'mystatefulset',
        'image': 'ubuntu:xenial',
        'volumeMounts': volumeMounts_one,
    }]

    volumeMounts_two = [{
        'name': 'vol1',
        'mountPath': 'var/lib/mysql',
        'subPath': '../mysql',
    }]

    containers_two = [{
        'name': 'mystatefulset',
        'image': 'ubuntu:xenial',
        'volumeMounts': volumeMounts_two,
    }]

    statefulSetConfig = {
        'podManagementPolicy': 'OrderedReady',
        'revisionHistoryLimit': 10,
        'strategy': 'RollingUpdate',
        'type': 'statefulSetConfig',
    }

    volumes = [{
        'name': 'vol1',
        'persistentVolumeClaim': {
                'persistentVolumeClaimId': "default: myvolume",
                'readOnly': False,
                'type': 'persistentVolumeClaimVolumeSource',
        },
        'type': 'volume',
    }]

    # 1. validate volumeMounts.subPath when workload creating
    # invalid volumeMounts.subPath: absolute path
    with pytest.raises(ApiError) as e:
        client.create_workload(name=name,
                               namespaceId='default',
                               scale=1,
                               containers=containers_one,
                               statefulSetConfig=statefulSetConfig,
                               volumes=volumes)

        assert e.value.error.status == 422

    # invalid volumeMounts.subPath: contains '..'
    with pytest.raises(ApiError) as e:
        client.create_workload(name=name,
                               namespaceId='default',
                               scale=1,
                               containers=containers_two,
                               statefulSetConfig=statefulSetConfig,
                               volumes=volumes)

        assert e.value.error.status == 422

    # 2. validate volumeMounts.subPath when workload update
    # create a validate workload then update
    workload = client.create_workload(name=name,
                                      namespaceId='default',
                                      scale=1,
                                      containers=containers,
                                      statefulSetConfig=statefulSetConfig,
                                      volumes=volumes)

    with pytest.raises(ApiError) as e:
        client.update(workload,
                      namespaceId='default',
                      scale=1,
                      containers=containers_one,
                      statefulSetConfig=statefulSetConfig,
                      volumes=volumes)

        assert e.value.error.status == 422

    with pytest.raises(ApiError) as e:
        client.update(workload,
                      namespaceId='default',
                      scale=1,
                      containers=containers_two,
                      statefulSetConfig=statefulSetConfig,
                      volumes=volumes)

        assert e.value.error.status == 422


def test_workload_redeploy(admin_pc, remove_resource):
    client = admin_pc.client

    ns = admin_pc.cluster.client.create_namespace(
        name=random_str(),
        projectId=admin_pc.project.id)
    remove_resource(ns)

    name = random_str()
    workload = client.create_workload(
        name=name,
        namespaceId=ns.id,
        scale=1,
        containers=[{
            'name': 'one',
            'image': 'nginx',
        }])
    remove_resource(workload)

    client.action(workload, 'redeploy')

    def _timestamp_reset():
        workloads = client.list_workload(uuid=workload.uuid).data
        return len(workloads) > 0 and workloads[0].annotations[
            'cattle.io/timestamp'] is not None

    wait_for(_timestamp_reset,
             fail_handler=lambda: 'Timed out waiting for timestamp reset')


def test_perform_workload_action_read_only(admin_mc, admin_pc, remove_resource,
                                           user_mc, user_factory):
    """Tests workload actions with a read-only user and a member user."""
    client = admin_pc.client
    project = admin_pc.project
    user = user_mc
    user_member = user_factory()

    ns = admin_pc.cluster.client.create_namespace(
        name=random_str(),
        projectId=project.id)
    remove_resource(ns)

    # Create a read-only user binding.
    prtb1 = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user.user.id,
        projectId=project.id,
        roleTemplateId="read-only")
    remove_resource(prtb1)
    wait_until_available(user.client, project)

    # Then, create a member user binding.
    prtb2 = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user_member.user.id,
        projectId=project.id,
        roleTemplateId="project-member")
    remove_resource(prtb2)
    wait_until_available(user_member.client, project)
    user_pc = user_project_client(user, project)
    user_member_pc = user_project_client(user_member, project)

    # Admin user creates the workload.
    workload_name = random_str()
    workload = client.create_workload(
        name=workload_name,
        namespaceId=ns.id,
        scale=1,
        containers=[{
            'name': 'foo',
            'image': 'nginx:1.18',
        }])
    remove_resource(workload)
    wait_for_workload(client, workload.id)

    # Admin user updates the workload to yield a rollback option. We change the
    # name below.
    workload.containers = [{
        'name': 'foo',
        'image': 'nginx:1.19',
    }]
    workload = client.update_by_id_workload(workload.id, workload)
    wait_for_workload(client, workload.id)
    workload = client.reload(workload)
    original_rev_id = workload.revisions().data[0].id

    # Read-only users should receive a 404 error.
    with pytest.raises(ApiError) as e:
        user_pc.action(obj=workload, action_name="rollback",
                       replicaSetId=original_rev_id)
    assert e.value.error.status == 404

    # Member users will be able to perform the rollback.
    user_member_pc.action(obj=workload, action_name="rollback",
                          replicaSetId=original_rev_id)


def wait_for_service_create(client, name, timeout=30):
    start = time.time()
    services = client.list_service(name=name, kind="ClusterIP")
    while len(services) == 0:
        time.sleep(.5)
        services = client.list_service(name=name, kind="ClusterIP")
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for workload service')
    return services.data[0]


def wait_for_service_cluserip_set(client, name, timeout=30):
    start = time.time()
    services = client.list_service(name=name, kind="ClusterIP")
    while len(services) == 0 or services.data[0].clusterIp is None:
        time.sleep(.5)
        services = client.list_service(name=name, kind="ClusterIP")
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for workload service')
    return services.data[0]


def wait_for_service_cluserip_reset(client, name, timeout=30):
    start = time.time()
    services = client.list_service(name=name, kind="ClusterIP")
    while len(services) == 0 or services.data[0].clusterIp is not None:
        time.sleep(.5)
        services = client.list_service(name=name, kind="ClusterIP")
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for workload service')
    return services.data[0]


def wait_for_workload(client, workload_id, timeout=30):
    """Wait for a given workload to be active."""
    def _get_result():
        try:
            return client.by_id_workload(workload_id)["state"]
        except KeyError:
            return ""
    start = time.time()
    while _get_result() != "active":
        time.sleep(.5)
        if time.time() - start > timeout:
            raise Exception("Timeout waiting for workload")
