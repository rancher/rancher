import json
import pytest
from rancher import ApiError
from .common import random_str
from .conftest import wait_for


@pytest.fixture(scope='module')
def check_cluster_kubernetes_version(admin_mc):
    """
       Checks the local cluster's k8s version
    """
    client = admin_mc.client
    cluster = client.by_id_cluster("local")
    version = cluster.get("version")
    if version is not None:
        k8s_version = int(version.get("gitVersion")[3:5])
        if k8s_version >= 25:
            pytest.skip("Needs to be reworked for PSA")


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
        if e.value.error.status == 404 or e.value.error.status == 500:
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


@pytest.mark.usefixtures('check_cluster_kubernetes_version')
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
