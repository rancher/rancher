from common import auth_check


def test_machine_fields(cclient):
    fields = {
        'clusterId': 'r',
        'allocatable': 'r',
        'capacity': 'r',
        'hostname': 'r',
        'info': 'r',
        'ipAddress': 'r',
        'limits': 'r',
        'nodeName': 'r',
        'requested': 'r',
        'requestedClusterId': 'cr',
        'requestedRoles': 'cr',
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
    }

    for name, field in cclient.schema.types['machine'].resourceFields.items():
        if name.endswith("Config"):
            fields[name] = 'cru'

    auth_check(cclient.schema, 'machine', 'crud', fields)
