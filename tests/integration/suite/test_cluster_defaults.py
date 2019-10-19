import json
import pytest
from rancher import ApiError
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

    assert 'exportYaml' not in cluster.actions


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

    assert 'exportYaml' in cluster.actions


def test_import_initial_conditions(admin_mc, remove_resource):
    cluster = admin_mc.client.create_cluster(name=random_str())
    remove_resource(cluster)

    assert cluster.conditions is None


def test_rke_k8s_deprecated_versions(admin_mc, remove_resource):
    client = admin_mc.client
    deprecated_versions_setting = client.by_id_setting(
                                "k8s-versions-deprecated")
    client.update_by_id_setting(id=deprecated_versions_setting.id,
                                value="{\"v1.8.10-rancher1-1\":true}")
    with pytest.raises(ApiError) as e:
        cluster = client.create_cluster(
            name=random_str(), rancherKubernetesEngineConfig={
                "kubernetesVersion": "v1.8.10-rancher1-1"})
        remove_resource(cluster)
    assert e.value.error.status == 500
    assert e.value.error.message == 'Requested kubernetesVersion ' \
                                    'v1.8.10-rancher1-1 is deprecated'
    client.update_by_id_setting(id=deprecated_versions_setting.id,
                                value="")


def test_save_as_template_action_rbac(admin_mc, remove_resource, user_factory):
    cluster = admin_mc.client.create_cluster(name=random_str(),
                                             rancherKubernetesEngineConfig={
                                                 "services": {
                                                     "type":
                                                     "rkeConfigServices",
                                                     "kubeApi": {
                                                         "alwaysPullImages":
                                                         "false",
                                                         "podSecurityPolicy":
                                                         "false",
                                                         "serviceNodePort\
                                                         Range":
                                                         "30000-32767",
                                                         "type":
                                                         "kubeAPIService"
                                                     }
                                                 }
                                             })
    remove_resource(cluster)
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'
    try:
        admin_mc.client.action(obj=cluster, action_name="saveAsTemplate",
                               clusterTemplateName="template1",
                               clusterTemplateRevisionName="v1")
    except ApiError as e:
        assert e.error.status == 503

    user = user_factory()
    user_cluster = user.client.create_cluster(name=random_str())
    remove_resource(user_cluster)
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'
    try:
        user.client.action(obj=user_cluster, action_name="saveAsTemplate")
    except AttributeError as e:
        assert e is not None
