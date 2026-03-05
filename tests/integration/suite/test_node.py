from .common import auth_check


def test_node_fields(admin_mc):
    cclient = admin_mc.client
    fields = {
        'annotations': 'cru',
        'appliedNodeVersion': 'r',
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
        'nodePlan': 'r',
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
        'podCidrs': 'r',
        'name': 'cru',
        'taints': 'ru',
        'unschedulable': 'r',
        'providerId': 'r',
        'sshUser': 'r',
        'imported': 'cru',
        'dockerInfo': 'r',
        'scaledownTime': 'cru',
        'runtimeHandlers': 'r',
        'features': 'r',
        'declaredFeatures': 'r',
    }

    for name in cclient.schema.types['node'].resourceFields.keys():
        if name.endswith("Config"):
            fields[name] = 'cr'

    fields['customConfig'] = 'cru'

    auth_check(cclient.schema, 'node', 'crud', fields)


def test_node_driver_schema(admin_mc):
    """Test node driver schemas have path fields removed."""
    drivers = ['amazonec2config', 'digitaloceanconfig', 'azureconfig']
    bad_fields = ['sshKeypath', 'sshKeyPath', 'existingKeyPath']
    client = admin_mc.client
    for driver in drivers:
        schema = client.schema.types[driver]
        for field in bad_fields:
            assert field not in schema.resourceFields, \
                'Driver {} has field {}'.format(driver, field)


def test_amazon_node_driver_schema(admin_mc):
    """Test amazon node driver schema supports AWS-specific resource fields"""
    required_fields = ['encryptEbsVolume']
    client = admin_mc.client
    schema = client.schema.types['amazonec2config']
    for field in required_fields:
        assert field in schema.resourceFields, \
            'amazonec2config missing support for field {}'.format(field)
