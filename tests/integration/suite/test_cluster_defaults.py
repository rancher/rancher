import json
import pytest

from .common import random_str


@pytest.mark.skip(reason="cluster-defaults disabled")
def test_generic_initial_defaults(admin_mc):
    cclient = admin_mc.client
    schema_defaults = {}
    setting_defaults = {}

    data = cclient.schema.types['cluster'].resourceFields
    default = data["enableNetworkPolicy"]["default"]

    for name in cclient.schema.types['cluster'].resourceFields.keys():
        if name == "enableNetworkPolicy":
            schema_defaults["enableNetworkPolicy"] = default

    for name in cclient.schema.types['rancherKubernetesEngineConfig'] \
            .resourceFields.keys():
        if name == "ignoreDockerVersion":
            schema_defaults["ignoreDockerVersion"] = cclient.schema. \
                types["rancherKubernetesEngineConfig"]. \
                resourceFields["ignoreDockerVersion"]. \
                data_dict()["default"]

    setting = cclient.list_setting(name="cluster-defaults")
    data = json.loads(setting['data'][0]['default'])

    setting_defaults["enableNetworkPolicy"] = data["enableNetworkPolicy"]
    setting_defaults["ignoreDockerVersion"] = \
        data["rancherKubernetesEngineConfig"]["ignoreDockerVersion"]

    assert schema_defaults == setting_defaults


def test_generic_initial_conditions(admin_mc, remove_resource):
    cluster = admin_mc.client.create_cluster(
            name=random_str(), amazonElasticContainerServiceConfig={
                "accessKey": "asdfsd"})
    remove_resource(cluster)

    assert len(cluster.conditions) == 3
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'

    assert cluster.conditions[1].type == 'Provisioned'
    assert cluster.conditions[1].status == 'Unknown'

    assert cluster.conditions[2].type == 'Waiting'
    assert cluster.conditions[2].status == 'Unknown'


def test_rke_initial_conditions(admin_mc, remove_resource):
    cluster = admin_mc.client.create_cluster(
            name=random_str(), rancherKubernetesEngineConfig={
                "accessKey": "asdfsd"})
    remove_resource(cluster)

    assert len(cluster.conditions) == 3
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'

    assert cluster.conditions[1].type == 'Provisioned'
    assert cluster.conditions[1].status == 'Unknown'

    assert cluster.conditions[2].type == 'Waiting'
    assert cluster.conditions[2].status == 'Unknown'


def test_import_initial_conditions(admin_mc, remove_resource):
    cluster = admin_mc.client.create_cluster(name=random_str())
    remove_resource(cluster)

    assert cluster.conditions is None
