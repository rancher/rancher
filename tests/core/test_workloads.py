from .common import random_str
import time


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
