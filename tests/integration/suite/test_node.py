import os
import tempfile
import pytest
from rancher import ApiError
from kubernetes.client import CoreV1Api
from .common import auth_check, random_str, string_to_encoding
from .conftest import wait_for, wait_for_condition
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
        'scaledownTime': 'cru',
        'runtimeHandlers': 'r',
        'features': 'r'
    }

    for name in cclient.schema.types['node'].resourceFields.keys():
        if name.endswith("Config"):
            fields[name] = 'cr'

    fields['customConfig'] = 'cru'

    auth_check(cclient.schema, 'node', 'crud', fields)


def test_node_template_delete(admin_mc, remove_resource):
    """Test deleting a nodeTemplate that is in use by a nodePool.
    The nodeTemplate should not be deleted while in use, after the nodePool is
    removed, the nodes referencing the nodeTemplate will be deleted
    and the nodeTemplate should delete
    """
    client = admin_mc.client
    node_template, cloud_credential = create_node_template(client)
    node_pool = client.create_node_pool(
        nodeTemplateId=node_template.id,
        hostnamePrefix=random_str(),
        clusterId="local")

    # node_pool needs to come first or the API will stop the delete if the
    # template still exists
    remove_resource(node_pool)
    remove_resource(node_template)

    assert node_pool.nodeTemplateId == node_template.id

    def _wait_for_no_remove_link():
        nt = client.reload(node_template)
        if not hasattr(nt.links, "remove"):
            return True
        return False

    wait_for(_wait_for_no_remove_link)

    # Attempting to delete the template should raise an ApiError
    with pytest.raises(ApiError) as e:
        client.delete(node_template)
    assert e.value.error.status == 405

    client.delete(node_pool)

    def _node_pool_reload():
        np = client.reload(node_pool)
        return np is None

    wait_for(_node_pool_reload)

    def _wait_for_remove_link():
        nt = client.reload(node_template)
        if hasattr(nt.links, "remove"):
            return True
        return False

    wait_for(_wait_for_remove_link)

    # NodePool and Nodes are gone, template should delete
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
        hostnamePrefix=random_str(),
        clusterId="local")
    assert node_pool.nodeTemplateId == node_template.id

    wait_for_node_template(client, node_template.id)

    # Attempting to delete the template should raise an ApiError
    with pytest.raises(ApiError) as e:
        client.delete(cloud_credential)
    assert e.value.error.status == 405


@pytest.mark.skip
def test_writing_config_to_disk(admin_mc, wait_remove_resource):
    """Test that userdata and other fields from node driver configs are being
    written to disk as expected.
    """
    client = admin_mc.client
    tempdir = tempfile.gettempdir()
    cloud_credential = client.create_cloud_credential(
        digitaloceancredentialConfig={"accessToken": "test"})
    wait_remove_resource(cloud_credential)

    data = {'userdata': 'do cool stuff' + random_str() + '\n',
            # This validates ssh keys don't drop the ending \n
            'id_rsa': 'some\nfake\nstuff\n' + random_str() + '\n'
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
        hostnamePrefix=random_str(),
        clusterId="local")

    def node_available():
        node = client.list_node(nodePoolId=node_pool.id)
        if len(node.data):
            return node.data[0]
        return None

    node = wait_for(node_available)
    wait_for_condition("Saved", "False", client, node)
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

        wait_for(file_exists, timeout=120,
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


def create_node_template(client, clientId="test"):
    cloud_credential = client.create_cloud_credential(
        azurecredentialConfig={"clientId": clientId,
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
        creds = client.list_cloud_credential()
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


@pytest.mark.skip(reason="flaky, todo in 27885")
def test_user_cluster_owner_access_to_pool(admin_mc,
                                           user_factory,
                                           remove_resource,
                                           wait_remove_resource):
    """Test that a cluster created by the admin is accessible by another user
    added as a cluster-owner, validate nodepool changing and switching
    nodetemplate"""

    # make an admin and user client
    admin_client = admin_mc.client
    k8sclient = CoreV1Api(admin_mc.k8s_client)
    user = user_factory()

    # make a cluster
    cluster = admin_client.create_cluster(
        name=random_str(),
        rancherKubernetesEngineConfig={
            "accessKey": "junk"
        }
    )
    remove_resource(cluster)

    # wait for the namespace created by the cluster
    def _check_namespace(cluster):
        for n in k8sclient.list_namespace().items:
            if n.metadata.name == cluster.id:
                return True
        return False

    wait_for(lambda: _check_namespace(cluster))

    # add user as cluster-owner to the cluster
    crtb = admin_client.create_cluster_role_template_binding(
        userId=user.user.id,
        roleTemplateId="cluster-owner",
        clusterId=cluster.id,
    )
    remove_resource(crtb)

    # admin creates a node template and assigns to a pool
    admin_node_template, admin_cloud_credential = create_node_template(
        admin_client, "admincloudcred-" + random_str())
    admin_pool = admin_client.create_node_pool(
        nodeTemplateId=admin_node_template.id,
        hostnamePrefix=random_str(),
        clusterId=cluster.id)
    wait_remove_resource(admin_pool)
    remove_resource(admin_cloud_credential)
    remove_resource(admin_node_template)

    # create a template for the user to try and assign
    user_node_template, user_cloud_credential = create_node_template(
        user.client, "usercloudcred-" + random_str())
    remove_resource(user_cloud_credential)
    remove_resource(user_node_template)

    # will pass, cluster owner user can change pool quantity
    user.client.update(admin_pool, quantity=2)
    # will pass, can set to a template owned by the user
    user.client.update(admin_pool, nodeTemplateId=user_node_template.id)

    # will fail, can not update nodepool template,
    # if no access to the original template
    with pytest.raises(ApiError) as e:
        user.client.update(admin_pool, nodeTemplateId=admin_node_template.id)
    assert e.value.error.status == 404
    assert e.value.error.message == "unable to find node template [%s]" % \
                                    admin_node_template.id

    # delete this by hand and the rest will cleanup
    admin_client.delete(admin_pool)


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
        hostnamePrefix=random_str(),
        clusterId="local")

    remove_list.insert(0, node_pool)


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
        hostnamePrefix=random_str(),
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
            hostnamePrefix=random_str(),
            clusterId="local")
    assert e.value.error.status == 404
    assert e.value.error.message == \
        "unable to find node template [%s]" % invalid_template_id
