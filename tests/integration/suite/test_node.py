import os
import tempfile
import pytest
from rancher import ApiError
from .common import auth_check, random_str, string_to_encoding
from .conftest import wait_for
import time


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
    }

    for name in cclient.schema.types['node'].resourceFields.keys():
        if name.endswith("Config"):
            fields[name] = 'cr'

    fields['customConfig'] = 'cru'

    auth_check(cclient.schema, 'node', 'crud', fields)


def test_node_template_delete(admin_mc, remove_resource):
    """Test deleting a nodeTemplate that is in use by a nodePool.
    The nodeTemplate should not be deleted while in use, after the nodePool is
    removed it should delete.
    """
    client = admin_mc.client
    node_template, cloud_credential = create_node_template(client)
    node_pool = client.create_node_pool(
        nodeTemplateId=node_template.id,
        hostnamePrefix="test1",
        clusterId="local")

    # node_pool needs to come first or the API will stop the delete if the
    # template still exists
    remove_resource(node_pool)
    remove_resource(node_template)

    assert node_pool.nodeTemplateId == node_template.id

    # Attempting to delete the template should raise an ApiError
    with pytest.raises(ApiError) as e:
        client.delete(node_template)
    assert e.value.error.status == 405

    # remove link should not be available
    node_template = client.reload(node_template)
    assert 'remove' not in node_template.links

    client.delete(node_pool)

    def _node_pool_reload():
        np = client.reload(node_pool)
        return np is None

    wait_for(_node_pool_reload)

    node_template = client.reload(node_template)
    assert 'remove' in node_template.links
    # NodePool is gone, template should delete
    client.delete(node_template)

    node_template = client.reload(node_template)
    assert node_template is None


def test_cloud_credential_delete(admin_mc, remove_resource):
    """Test deleting a cloud credential that is referenced by nodeTemplate, which
    is in use by nodePool
    """
    client = admin_mc.client
    node_template, cloud_credential = create_node_template(client)
    node_pool = client.create_node_pool(
        nodeTemplateId=node_template.id,
        hostnamePrefix="test1",
        clusterId="local")
    assert node_pool.nodeTemplateId == node_template.id

    wait_for_node_template(client, node_template.id)

    # Attempting to delete the template should raise an ApiError
    with pytest.raises(ApiError) as e:
        client.delete(cloud_credential)
    assert e.value.error.status == 405


def test_writing_config_to_disk(admin_mc, wait_remove_resource):
    """Test that userdata and other fields from node driver configs are being
    writting to disk as expected.
    """
    client = admin_mc.client
    tempdir = tempfile.gettempdir()
    cloud_credential = client.create_cloud_credential(
        digitaloceancredentialConfig={"accessToken": "test"})
    wait_remove_resource(cloud_credential)

    data = {'userdata': 'do cool stuff\n',
            # This validates ssh keys don't drop the ending \n
            'id_rsa': 'some\nfake\nstuff\n'
            }

    def _node_template():
        try:
            return client.create_node_template(
                digitaloceanConfig={
                    'userdata': data['userdata'],
                    'sshKeyContents': data['id_rsa']
                },
                name=random_str(),
                cloudCredentialId=cloud_credential.id)

        except ApiError:
            return False

    node_template = wait_for(_node_template,
                             fail_handler=lambda:
                             'failed to create node template')
    wait_remove_resource(node_template)

    node_pool = client.create_node_pool(
        nodeTemplateId=node_template.id,
        hostnamePrefix="test1",
        clusterId="local")
    wait_remove_resource(node_pool)

    for key, value in data.items():
        dir_name = string_to_encoding(value)

        full_path = os.path.join(tempdir, dir_name, key)

        def file_exists():
            try:
                os.stat(full_path)
                return True
            except FileNotFoundError:
                return False

        wait_for(file_exists, timeout=10,
                 fail_handler=lambda: 'file is missing from disk')

        with open(full_path, 'r') as f:
            contents = f.read()

        assert contents == value


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


def create_node_template(client):
    cloud_credential = client.create_cloud_credential(
        azurecredentialConfig={"clientId": "test",
                               "subscriptionId": "test",
                               "clientSecret": "test"})
    wait_for_cloud_credential(client, cloud_credential.id)
    node_template = client.create_node_template(
        azureConfig={},
        cloudCredentialId=cloud_credential.id)
    assert node_template.cloudCredentialId == cloud_credential.id
    return node_template, cloud_credential


def wait_for_cloud_credential(client, cloud_credential_id, timeout=60):
    start = time.time()
    interval = 0.5
    creds = client.list_cloud_credential()
    cred = None
    for val in creds:
        if val["id"] == cloud_credential_id:
            cred = val
    while cred is None:
        if time.time() - start > timeout:
            print(cred)
            raise Exception('Timeout waiting for cloud credential')
        time.sleep(interval)
        interval *= 2
        for val in creds:
            if val["id"] == cloud_credential_id:
                cred = val
    return cred


def wait_for_node_template(client, node_template_id, timeout=60):
    start = time.time()
    interval = 0.5
    template = None
    while template is None:
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for node template lister')
        time.sleep(interval)
        interval *= 2
        nodeTemplates = client.list_node_template()
        for each_template in nodeTemplates:
            if each_template["id"] == node_template_id:
                template = each_template


def test_user_access_to_other_template(user_factory, remove_resource):
    """Asserts that a normal user's nodepool cannot reference another user's
    nodetemplate"""
    user1_client = user_factory().client
    user2_client = user_factory().client

    user2_node_template = user2_client.create_node_template(name="nt-" +
                                                                 random_str(),
                                                            azureConfig={})
    remove_resource(user2_node_template)
    wait_for_node_template(user2_client, user2_node_template.id)

    with pytest.raises(ApiError) as e:
        user1_client.create_node_pool(
            nodeTemplateId=user2_node_template.id,
            hostnamePrefix="test1",
            clusterId="local")
    assert e.value.error.status == 404
    assert e.value.error.message == \
        "unable to find node template [%s]" % user2_node_template.id


def test_admin_access_to_node_template(admin_mc, list_remove_resource):
    """Asserts that an admin user's nodepool can reference
    nodetemplates they have created"""
    admin_client = admin_mc.client

    admin_node_template = admin_client.create_node_template(name="nt-" +
                                                                 random_str(),
                                                            azureConfig={})
    remove_list = [admin_node_template]
    list_remove_resource(remove_list)

    # Admin has access to create nodepool and nodepool create only happens
    # after it passes validation.
    node_pool = admin_client.create_node_pool(
        nodeTemplateId=admin_node_template.id,
        hostnamePrefix="test1",
        clusterId="local")

    remove_list.insert(0, node_pool)


def test_user_access_to_node_template(user_mc, remove_resource):
    """Asserts that a normal user's nodepool can reference
    nodetemplates they have created"""
    user_client = user_mc.client

    user_node_template = user_client.create_node_template(name="nt-" +
                                                               random_str(),
                                                          azureConfig={})
    remove_resource(user_node_template)
    wait_for_node_template(user_client, user_node_template.id)

    with pytest.raises(ApiError) as e:
        user_client.create_node_pool(
            nodeTemplateId=user_node_template.id,
            hostnamePrefix="test1",
            clusterId="local")
    # User does not have access to create nodepools but has
    # access to nodetemplate. Nodepool create happens after
    # validation has passed.
    assert e.value.error.status == 403
    assert 'cannot create resource "nodepools"' in e.value.error.message


def test_admin_access_user_template(admin_mc, user_mc, list_remove_resource):
    """Asserts that an admin user's nodepool can reference another user's
    nodetemplates"""
    admin_client = admin_mc.client
    user_client = user_mc.client

    user_node_template = user_client.create_node_template(name="nt-" +
                                                               random_str(),
                                                          azureConfig={})
    remove_list = [user_node_template]
    list_remove_resource(remove_list)
    # Admin has access to create nodepool and nodepool create only happens
    # after it passes validation.
    node_pool = admin_client.create_node_pool(
        nodeTemplateId=user_node_template.id,
        hostnamePrefix="test1",
        clusterId="local")
    remove_list.insert(0, node_pool)


def test_no_node_template(user_mc):
    """Asserts that a nodepool cannot create without a valid
    nodetemplate"""
    user_client = user_mc.client

    invalid_template_id = "thisinsnotatemplateid"

    with pytest.raises(ApiError) as e:
        user_client.create_node_pool(
            nodeTemplateId=invalid_template_id,
            hostnamePrefix="test1",
            clusterId="local")
    assert e.value.error.status == 404
    assert e.value.error.message == \
        "unable to find node template [%s]" % invalid_template_id
