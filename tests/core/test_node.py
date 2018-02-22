from common import auth_check


def test_node_fields(mc):
    cclient = mc.client
    fields = {
        'annotations': 'r',
        'labels': 'r',
        'nodeTaints': 'r',
        'namespaceId': 'cr',
        'conditions': 'r',
        'allocatable': 'r',
        'capacity': 'r',
        'hostname': 'r',
        'info': 'r',
        'ipAddress': 'r',
        'limits': 'r',
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
        'unschedulable': 'ru',
        'providerId': 'r',
        'sshUser': 'r',
        'imported': "cru",
    }

    for name, field in cclient.schema.types['node'].resourceFields.items():
        if name.endswith("Config"):
            fields[name] = 'cr'

    fields['customConfig'] = 'cru'

    auth_check(cclient.schema, 'node', 'crud', fields)
