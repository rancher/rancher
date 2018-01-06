from common import random_str, auth_check


def test_ingress_fields(client):
    auth_check(client.schema, 'ingress', 'crud', {
        'namespaceId': 'cr',
        'projectId': 'cr',
        'rules': 'cru',
        'tls': 'cru',
        'defaultBackend': 'cru',
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

    ns = client.create_namespace(name=random_str())
    wl = client.create_workload(namespaceId=ns.id,
                                scale=1,
                                containers=[{
                                    'name': 'one',
                                    'image': 'nginx',
                                }])

    name = random_str()
    client.create_ingress(name=name,
                          namespaceId=ns.id,
                          rules=[
                              {
                                  'paths': {
                                      '/': {
                                          'targetPort': 80,
                                          'workloadIds': [wl.id],
                                      }
                                  }
                              },
                          ])

    # assert ingress.rules[0]['paths']['/'] == {
    #     'targetPort': 80,
    #     'workloadIds': [wl.id]
    # }

    client.delete(ns)
