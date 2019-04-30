import pytest
from rancher import ApiError
from .common import auth_check
from .conftest import wait_for
import time


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

    # Attempting to delete the template should raise an ApiError
    with pytest.raises(ApiError) as e:
        client.delete(cloud_credential)
    assert e.value.error.status == 405


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
