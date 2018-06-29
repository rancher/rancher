from .common import auth_check


def test_node_fields(admin_mc):
    cclient = admin_mc.client
    fields = {
        'annotations': 'r',
        'labels': 'cru',
        'nodeTaints': 'r',
        'namespaceId': 'cr',
        'conditions': 'r',
        'allocatable': 'r',
        'capacity': 'r',
        'hostname': 'r',
        'info': 'r',
        'ipAddress': 'r',
        'externalIpAddress': 'r',
        'limits': 'r',
        'publicEndpoints': 'r',
        'nodePoolId': 'r',
        'nodeName': 'r',
        'requested': 'r',
        'clusterId': 'cr',
        'etcd': 'cr',
        'controlPlane': 'cr',
        'worker': 'cr',
        'requestedHostname': 'cr',
        'volumesAttached': 'r',
        'nodeTemplateId': 'cr',
        'volumesInUse': 'r',
        'podCidr': 'r',
        'name': 'cru',
        'taints': 'ru',
        'unschedulable': 'r',
        'providerId': 'r',
        'sshUser': 'r',
        'imported': 'cru',
        'dockerInfo': 'r',
    }

    for name in cclient.schema.types['node'].resourceFields.keys():
        if name.endswith("Config"):
            fields[name] = 'cr'

    fields['customConfig'] = 'cru'

    auth_check(cclient.schema, 'node', 'crud', fields)
