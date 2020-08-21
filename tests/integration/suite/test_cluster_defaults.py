import json
import pytest
from rancher import ApiError
from .common import random_str
from .conftest import wait_for


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


def test_eks_cluster_immutable_subnets(admin_mc, remove_resource):
    cluster = admin_mc.client.create_cluster(
        name=random_str(), amazonElasticContainerServiceConfig={
            "accessKey": "asdfsd",
            "secretKey": "verySecretKey",
            "subnets": [
                "subnet-045bfaeca7d3f1cb3",
                "subnet-02388a166136f98c4"
            ]})
    remove_resource(cluster)

    def cannot_modify_error():
        with pytest.raises(ApiError) as e:
            # try to edit cluster subnets
            admin_mc.client.update_by_id_cluster(
                id=cluster.id,
                amazonElasticContainerServiceConfig={
                     "accessKey": "asdfsd",
                     "secretKey": "verySecretKey",
                     "subnets": [
                         "subnet-045bfaeca7d3f1cb3"
                     ]})
        if e.value.error.status == 404:
            return False
        print(e)
        assert e.value.error.status == 422
        assert e.value.error.message ==\
            'cannot modify EKS subnets after creation'
        return True

    # lister used by cluster validator may not be up to date, may need to retry
    wait_for(cannot_modify_error)

    # tests updates still work
    new = admin_mc.client.update_by_id_cluster(
       id=cluster.id,
       name=cluster.name,
       description="update",
       amazonElasticContainerServiceConfig={
           # required field when updating KE clusters
           "driverName": "amazonelasticcontainerservice",
           "accessKey": "asdfsd",
           "secretKey": "verySecretKey",
           "subnets": [
               "subnet-045bfaeca7d3f1cb3",
               "subnet-02388a166136f98c4"
           ]})

    assert new.id == cluster.id
    assert not hasattr(cluster, "description")
    assert hasattr(new, "description")


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


def test_psp_enabled_set(admin_mc, remove_resource):
    """Asserts podSecurityPolicy field is used to populate pspEnabled in
    cluster capabilities"""
    admin_client = admin_mc.client
    cluster = admin_client.create_cluster(
        name=random_str(), rancherKubernetesEngineConfig={
            "accessKey": "asdfsd",
            "services": {
                "kubeApi": {
                    "podSecurityPolicy": True,
                }
            }
        })
    remove_resource(cluster)

    def psp_set_to_true():
        updated_cluster = admin_client.by_id_cluster(id=cluster.id)
        capabilities = updated_cluster.get("capabilities")
        if capabilities is not None:
            return capabilities.get("pspEnabled") is True
        return None

    wait_for(lambda: psp_set_to_true(), fail_handler=lambda: "failed waiting "
             "for pspEnabled to be set")


def test_import_initial_conditions(admin_mc, remove_resource):
    cluster = admin_mc.client.create_cluster(name=random_str())
    remove_resource(cluster)

    assert not cluster.conditions


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
