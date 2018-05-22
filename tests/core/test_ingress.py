from common import random_str, auth_check


def test_ingress_fields(client):
    auth_check(client.schema, 'ingress', 'crud', {
        'namespaceId': 'cr',
        'projectId': 'cr',
        'rules': 'cru',
        'tls': 'cru',
        'defaultBackend': 'cru',
        'publicEndpoints': 'r',
        'status': 'r',
    })

    auth_check(client.schema, 'ingressBackend', '', {
        'serviceId': 'cru',
        'targetPort': 'cru',
        'workloadIds': 'cru',
    })

    auth_check(client.schema, 'ingressRule', '', {
        'host': 'cru',
        'paths': 'cru',
    })

    assert 'httpIngressPath' not in client.schema.types


def test_ingress(pc):
    client = pc.client

    ns = pc.cluster.client.create_namespace(name=random_str(),
                                            projectId=pc.project.id)

    name = random_str()
    workload = client.create_workload(
                                    name=name,
                                    namespaceId=ns.id,
                                    scale=1,
                                    containers=[{
                                        'name': 'one',
                                        'image': 'nginx',
                                    }])

    name = random_str()
    ingress = client.create_ingress(name=name,
                                    namespaceId=ns.id,
                                    rules=[{
                                          'host': "foo.com",
                                          'paths': {
                                              '/': {
                                                  'targetPort': 80,
                                                  'workloadIds':
                                                  [workload.id],
                                              }
                                          }},
                                      ])

    assert len(ingress.rules) == 1
    assert ingress.rules[0]['host'] == "foo.com"
    path = ingress.rules[0]['paths']['/']
    assert path['targetPort'] == 80
    assert path['workloadIds'] == [workload.id]
    assert path['serviceId'] is None

    client.delete(ns)


def test_ingress_rules_same_hostPortPath(pc):
    client = pc.client

    ns = pc.cluster.client.create_namespace(name=random_str(),
                                            projectId=pc.project.id)

    name = random_str()
    workload1 = client.create_workload(
                                    name=name,
                                    namespaceId=ns.id,
                                    scale=1,
                                    containers=[{
                                        'name': 'one',
                                        'image': 'nginx',
                                    }])

    name = random_str()
    workload2 = client.create_workload(
                                    name=name,
                                    namespaceId=ns.id,
                                    scale=1,
                                    containers=[{
                                        'name': 'one',
                                        'image': 'nginx',
                                    }])

    name = random_str()
    ingress = client.create_ingress(name=name,
                                    namespaceId=ns.id,
                                    rules=[{
                                          'host': "foo.com",
                                          'paths': {
                                              '/': {
                                                  'targetPort': 80,
                                                  'workloadIds':
                                                  [workload1.id],
                                              }
                                          }},
                                          {
                                          'host': "foo.com",
                                          'paths': {
                                              '/': {
                                                  'targetPort': 80,
                                                  'workloadIds':
                                                  [workload2.id],
                                              }
                                          }},
                                      ])

    assert len(ingress.rules) == 1
    assert ingress.rules[0]['host'] == "foo.com"
    path = ingress.rules[0]['paths']['/']
    assert path['targetPort'] == 80
    assert len(path['workloadIds']) == 2
    assert set(path['workloadIds']) == set([workload1.id, workload2.id])
    assert path['serviceId'] is None

    client.delete(ns)
