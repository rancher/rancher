from .common import *  # NOQA
import pytest

namespace = {'client': None, 'cluster': None}


def test_fleet_simple():
    client = namespace['client']

    template = read_json_from_resource_dir("fleet_1.json")
    name = random_name()
    # set name
    template['metadata']['name'] = name
    # set target
    cluster_id = namespace['cluster']['id']
    match_labels = template['spec']['targets'][0]['clusterSelector']['matchLabels']
    match_labels['management.cattle.io/cluster-name'] = cluster_id
    res = client.create_fleet_cattle_io_gitrepo(template)
    res = validate_fleet(client, res)
    # delete the fleet
    client.delete(res)


def validate_fleet(client, fleet):
    # the gitRepo's state shows active at the beginning
    # which is not the actual state
    time.sleep(10)
    try:
        wait_for(lambda: client.reload(fleet).metadata.state.name == 'active',
                 timeout_message='time out waiting for gitRepo to be ready')
    except Exception as e:
        assert False, str(e)
    fleet = client.reload(fleet)
    # validate the bundle is active
    bundle = get_bundle_by_fleet_name(client, fleet.metadata.name)
    wait_for(lambda: client.reload(bundle).metadata.state.name == 'active',
             timeout_message='time out waiting for the bundle to be ready')
    return fleet


def get_bundle_by_fleet_name(client, name):
    start_time = time.time()
    while time.time() - start_time < 30:
        res = client.list_fleet_cattle_io_bundle()
        for bundle in res['data']:
            keys = bundle['metadata']['labels'].keys()
            print("------")
            print(keys)
            if 'fleet.cattle.io/repo-name' in keys:
                bundle_name = \
                    bundle['metadata']['labels']['fleet.cattle.io/repo-name']
                if bundle_name == name:
                    return bundle
        time.sleep(5)
    return None


@pytest.fixture(scope='module', autouse='True')
def create_client(request):
    if CLUSTER_NAME == '':
        assert False, 'no cluster is provided, cannot run tests for fleet'
    client = get_admin_client_v1()
    namespace['client'] = client
    res = client.list_management_cattle_io_cluster()
    for cluster in res.data:
        if cluster['spec']['displayName'] == CLUSTER_NAME:
            namespace['cluster'] = cluster
    if namespace['cluster'] is None:
        assert False, 'cannot find the target cluster'
