from .common import random_str


def test_hpa(admin_pc):
    client = admin_pc.client
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
            'image': 'nginx',
            'resources': {
                'requests': '100m',
            },
        }])
    assert workload.id != ''
    name = random_str()
    client.create_horizontalPodAutoscaler(
        name=name,
        namespaceId=ns.id,
        maxReplicas=10,
        workloadId=workload.id,
        metrics=[{
            'name': 'cpu',
            'type': 'Resource',
            'target': {
                'type': 'Utilization',
                'utilization': '50',
            },
        }, {
            'name': 'pods-test',
            'type': 'Pods',
            'target': {
                'type': 'AverageValue',
                'averageValue': '50',
            },
        }, {
            'name': 'pods-external',
            'type': 'External',
            'target': {
                'type': 'Value',
                'value': '50',
            },
        }, {
            "describedObject": {
                "apiVersion": "extensions/v1beta1",
                "kind": "Ingress",
                "name": "test",
            },
            'name': 'object-test',
            'type': 'Object',
            'target': {
                'type': 'Value',
                'value': '50',
            },
        }],
    )
    hpas = client.list_horizontalPodAutoscaler(
        namespaceId=ns.id
    )
    assert len(hpas) == 1
    hpa = hpas.data[0]
    assert hpa.state == "initializing"
    client.delete(hpa)
    client.delete(workload)
    client.delete(ns)
