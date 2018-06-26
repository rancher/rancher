from .common import random_str
import time


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
        assert secret.data_dict()['name'] == registry1_name

    containers = [{
                    'name': 'one',
                    'image': 'quay.io/testuser/testimage',
                 }]

    workload = client.update(workload, containers=containers)

    for container in workload.containers:
        assert container.data_dict()['image'] == 'quay.io/testuser/testimage'

    assert len(workload.imagePullSecrets) == 1

    assert workload.imagePullSecrets[0].data_dict()['name'] == registry2_name

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

    # update workload wiht port, and validate cluster ip is set
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

    # update workload wiht no ports, and validate cluster ip is reset
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
