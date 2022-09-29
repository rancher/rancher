import requests
from time import sleep
from .conftest import SERVER_URL, AUTH_URL, TEST_USERNAME, TEST_PASSWORD

CLUSTER_URL = SERVER_URL + '/v1/provisioning.cattle.io.cluster'
NODE_URL = SERVER_URL + '/v1/node'
SECRET_URL = SERVER_URL + '/v1/secret'
RKE_CONFIG = {
    'upgradeStrategy': {
        'controlPlaneConcurrency': '1',
        'workerConcurrency': '1',
        'controlPlaneDrainOptions': {
            'deleteEmptyDirData': True,
            'enabled': True,
            'force': True,
            'ignoreDaemonSets': True,
            'preDrainHooks': [{
                'annotation': 'harvesterhci.io/pre-hook'
            }],
            'postDrainHooks': [{
                'annotation': 'harvesterhci.io/post-hook'
            }]
        },
        'workerDrainOptions': {
            'deleteEmptyDirData': True,
            'enabled': True,
            'force': True,
            'ignoreDaemonSets': True,
            'preDrainHooks': [{
                'annotation': 'harvesterhci.io/pre-hook'
            }],
            'postDrainHooks': [{
                'annotation': 'harvesterhci.io/post-hook'
            }]
        }
    }
}


def test_rke2_upgrade_without_version_bump():
    """
    Test when upgrading a provisioned RKE2 cluster (without version bump) will
    pre-drain and post-drain hooks be called
    Running this test needs an existing RKE2 cluster provisioned
    by a rancher installation.
    """
    token = get_token()

    cluster = get_local_cluster(token)
    current_gen = 0
    if 'rkeConfig' in cluster['spec']:
        if 'provisionGeneration' in cluster['spec']['rkeConfig']:
            current_gen = cluster['spec']['rkeConfig']['provisionGeneration']

    cluster['spec']['rkeConfig'] = RKE_CONFIG
    cluster['spec']['rkeConfig']['provisionGeneration'] = current_gen + 1
    updated = update_with_retries(url=CLUSTER_URL + '/fleet-local/local',
                                  json=cluster,
                                  token=token)

    assert updated['spec']['rkeConfig'] == RKE_CONFIG
    sleep(2)

    check_node_upgrade(version=None, token=token)


def test_rke2_upgrade_with_version_bump():
    """
    Test when upgrading a provisioned RKE2 cluster will pre-drain
    and post-drain hooks be called.
    Running this test needs an existing RKE2 cluster provisioned
    by a rancher installation.
    """
    token = get_token()

    cluster = get_local_cluster(token)
    release_upgrade = 'v1.23.12+rke2r1'

    cluster['spec']['kubernetesVersion'] = release_upgrade
    cluster['spec']['rkeConfig'] = RKE_CONFIG
    updated = update_with_retries(url=CLUSTER_URL + '/fleet-local/local',
                                  json=cluster,
                                  token=token)

    assert updated['spec']['kubernetesVersion'] == release_upgrade
    assert updated['spec']['rkeConfig'] == RKE_CONFIG
    sleep(2)

    check_node_upgrade(release_upgrade, token)


def get_local_cluster(token):
    """
    Get *local* cluster object in fleet-local namespace.
    :param token: Request token
    :return: Cluster object
    """
    response = requests.get(url=CLUSTER_URL + '/fleet-local/local', headers={
        'Cookie': 'R_USERNAME=admin; R_SESS=' + token
    }, verify=False)
    return response.json()


def check_node_upgrade(version, token):
    """
    Get all nodes in local cluster, first wait for pre-hook execution, then
    check and wait for kubernetes version
    upgrade if needed, and wait for post-hook execution at last.
    :param version: Kubernetes version to upgrade, None for upgrade
    without version bump
    :param token: Request token
    """
    nodes = get_nodes(token)
    node_ids = get_node_ids(nodes)

    plan_secrets = []
    for node_id in node_ids:
        plan_secrets.append(get_node_plan_secret(node_id, token))

    processed_secrets = []
    while len(processed_secrets) < len(plan_secrets):
        updating_secret = find_updating_node_secret(plan_secrets)
        sleep(5)
        secret_name = updating_secret['metadata']['name']
        pre_hook = updating_secret['metadata']['annotations'][
            'rke.cattle.io/pre-drain']

        updating_secret['metadata']['annotations'][
            'harvesterhci.io/pre-hook'] = pre_hook
        pre_hook_updated_secret = \
            update_with_retries(url=SECRET_URL + '/fleet-local/' + secret_name,
                                json=updating_secret,
                                token=token)
        assert pre_hook_updated_secret['metadata']['annotations'][
                   'harvesterhci.io/pre-hook'] == pre_hook

        if version is not None:
            updating_node_id = secret_name[:-13]
            for node in nodes:
                node_annotations = node['metadata']['annotations']
                if 'cluster.x-k8s.io/machine' in node_annotations and \
                        node_annotations[
                            'cluster.x-k8s.io/machine'] == updating_node_id:
                    wait_node_upgrade(node, version, token)
                    break

        post_hook = None
        while post_hook is None:
            new_secret = requests.get(
                url=SECRET_URL + '/fleet-local/' + secret_name, headers={
                    'Cookie': 'R_USERNAME=admin; R_SESS=' + token
                }, verify=False).json()
            if 'rke.cattle.io/post-drain' in \
                    new_secret['metadata']['annotations']:
                post_hook = \
                    new_secret['metadata']['annotations'][
                        'rke.cattle.io/post-drain']
            else:
                sleep(5)
        sleep(5)
        pre_hook_updated_secret['metadata']['annotations'][
            'harvesterhci.io/post-hook'] = post_hook
        post_hook_updated_secret = update_with_retries(
            url=SECRET_URL + '/fleet-local/' + secret_name,
            json=pre_hook_updated_secret,
            token=token)
        assert post_hook_updated_secret['metadata']['annotations'][
                   'harvesterhci.io/post-hook'] == post_hook

        processed_secrets.append(post_hook_updated_secret)


def wait_node_upgrade(node, version, token):
    """
    Wait for the specific node to upgrade to specific kubernetes version.
    :param node: Node to wait for
    :param version: Kubernetes version to upgrade
    :param token: Request token
    """
    current_version = node['status']['nodeInfo']['kubeletVersion']
    name = node['metadata']['name']

    while current_version != version:
        response = requests.get(url=NODE_URL + '/' + name, headers={
            'Cookie': 'R_USERNAME=admin; R_SESS=' + token
        }, verify=False)
        current_version = response.json()['status']['nodeInfo'][
            'kubeletVersion']
        if current_version != version:
            sleep(2)


def find_updating_node_secret(node_secrets):
    """
    Try to find the machine plan secret of
    an upgrading node in the local cluster.
    :param node_secrets: All node secrets
    :return:
    """
    updating_node_secret = None
    while updating_node_secret is None:
        for node_secret in node_secrets:
            annotations = node_secret['metadata']['annotations']
            if 'rke.cattle.io/pre-drain' in annotations \
                    and annotations['rke.cattle.io/pre-drain'] is not None:
                updating_node_secret = node_secret
                break
            sleep(3)

    return updating_node_secret


def get_node_plan_secret(node_id, token):
    """
    Get node's machine plan secret using node's id.
    :param node_id: Node id
    :param token: Request token
    :return: Node's machine plan secret
    """
    secret_name = node_id + '-machine-plan'
    response = requests.get(url=SECRET_URL + '/fleet-local/' + secret_name,
                            headers={
                                'Cookie': 'R_USERNAME=admin; R_SESS=' + token
                            }, verify=False)

    return response.json()


def get_node_ids(nodes):
    """
    Convert an array of node objects to the array containing node's id.
    :param nodes: Node array
    :return: Node id array
    """
    ids = []
    for node in nodes:
        if 'cluster.x-k8s.io/machine' in node['metadata']['annotations']:
            ids.append(
                node['metadata']['annotations']['cluster.x-k8s.io/machine'])

    return ids


def get_nodes(token):
    """
    Get all nodes in the local cluster.
    :param token: Request token
    :return: All nodes
    """
    response = requests.get(url=NODE_URL, headers={
        'Cookie': 'R_USERNAME=admin; R_SESS=' + token
    }, verify=False)
    return response.json()['data']


def get_token():
    """
    Try to authenticate with specified credential and fetch the token
    :return: Token
    """
    response = requests.post(AUTH_URL, json={
        'username': TEST_USERNAME,
        'password': TEST_PASSWORD,
        'responseType': 'json',
    }, verify=False)
    return response.json()['token']


def update_with_retries(url, json, token):
    """
    Update an object, retry 5 times if failed
    because of Internal Server Error (500).

    :param url: Object update URL
    :param json: Object to upgrade
    :param token: Request token
    :return: Updated object
    """
    response = requests.get(url=url, headers={
        'Cookie': 'R_USERNAME=admin; R_SESS=' + token
    }, verify=False)
    json['metadata']['resourceVersion'] = response.json()['metadata'][
        'resourceVersion']

    response = requests.put(url=url, json=json, headers={
        'Cookie': 'R_USERNAME=admin; R_SESS=' + token
    }, verify=False)
    retries = 5
    while response.status_code == 500 and retries > 0:
        response = requests.get(url=url, headers={
            'Cookie': 'R_USERNAME=admin; R_SESS=' + token
        }, verify=False)
        json['metadata']['resourceVersion'] = response.json()['metadata'][
            'resourceVersion']
        response = requests.put(url=url, json=json, headers={
            'Cookie': 'R_USERNAME=admin; R_SESS=' + token
        }, verify=False)
        retries -= 1
        if response.status_code == 500:
            sleep(3)

    if response.status_code == 500:
        raise ConnectionError(
            'Failed to update, body: ' + str(response.json()))
    return response.json()
