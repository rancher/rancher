from common import auth_check


def test_machine_fields(cclient):
    fields = {
        'useInternalIpAddress': 'cr',
        'nodeTaints': 'r',
        'nodeLabels': 'r',
        'nodeAnnotations': 'r',
        'namespaceId': 'cr',
        'conditions': 'r',
        'allocatable': 'r',
        'capacity': 'r',
        'hostname': 'r',
        'info': 'r',
        'ipAddress': 'r',
        'limits': 'r',
        'nodeName': 'r',
        'requested': 'r',
        'clusterId': 'cr',
        'role': 'cr',
        'requestedHostname': 'cr',
        'volumesAttached': 'r',
        'machineTemplateId': 'cr',
        'volumesInUse': 'r',
        'podCidr': 'r',
        'name': 'cru',
        'taints': 'ru',
        'unschedulable': 'ru',
        'providerId': 'r',
        'sshUser': 'r',
        'imported': "cru",
    }

    for name, field in cclient.schema.types['machine'].resourceFields.items():
        if name.endswith("Config"):
            fields[name] = 'cr'

    fields['customConfig'] = 'cru'

    auth_check(cclient.schema, 'machine', 'crud', fields)
