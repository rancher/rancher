import os
import pytest
from rancher import ApiError
from .common import *  # NOQA
from .test_rke_cluster_provisioning import rke_config
from .test_rke_cluster_provisioning import random_node_name
from .test_rke_cluster_provisioning import create_and_validate_cluster

DO_ACCESSKEY = os.environ.get('DO_ACCESSKEY', "None")
engine_install_url = "https://releases.rancher.com/install-docker/19.03.sh"
user_clients = {"admin": None, "standard_user_1": None,
                "standard_user_2": None}


# --------------------- rbac tests for node template -----------------------

@if_test_rbac
def test_rbac_node_template_create(remove_resource):
    # As std user, create a node template
    node_template = create_node_template_do(user_clients["standard_user_1"])
    remove_resource(node_template)
    templates = user_clients["standard_user_1"].list_node_template(
                                                name=node_template.name)
    assert len(templates) == 1


@if_test_rbac
def test_rbac_node_template_list(remove_resource):
    # User client should be able to list template it has created
    node_template = create_node_template_do(user_clients["standard_user_1"])
    templates = user_clients["standard_user_1"].list_node_template(
                                                name=node_template.name)
    remove_resource(node_template)
    assert len(templates) == 1
    # Admin should be able to list template
    templates = user_clients["admin"].list_node_template(
                                      name=node_template.name)
    assert len(templates) == 1
    # User 2 should not be able to list templates
    templates2 = user_clients["standard_user_2"].list_node_template(
                                                 name=node_template.name)
    assert len(templates2) == 0


@if_test_rbac
def test_rbac_node_template_delete(remove_resource):
    # User client should be able to delete template it has created
    node_template = create_node_template_do(user_clients["standard_user_1"])
    # User1 should be able to delete own template
    user_clients["standard_user_1"].delete(node_template)
    templates = user_clients["standard_user_1"].list_node_template(
                                                name=node_template.name)
    assert len(templates) == 0
    # Admin should be able to delete template created by user1
    node_template2 = create_node_template_do(user_clients["standard_user_1"])
    user_clients["admin"].delete(node_template2)
    templates = user_clients["standard_user_1"].list_node_template(
                                                name=node_template2.name)
    assert len(templates) == 0
    # User 2 should not be able to delete template created by user1
    node_template3 = create_node_template_do(user_clients["standard_user_1"])
    remove_resource(node_template3)
    with pytest.raises(ApiError) as e:
        user_clients["standard_user_2"].delete(node_template3)
    assert e.value.error.status == 403


@if_test_rbac
def test_rbac_node_template_edit(remove_resource):
    # User client should be able to edit template it has created
    node_template = create_node_template_do(user_clients["standard_user_1"])
    remove_resource(node_template)
    # User1 should be able to edit own template
    name_edit=random_name()
    user_clients["standard_user_1"].update(node_template, name=name_edit,
                                           digitaloceanConfig=
                                           {"region": "nyc3",
                                            "size": "2gb",
                                            "image": "ubuntu-16-04-x64"})
    templates = user_clients["standard_user_1"].list_node_template(
                                                name=name_edit)
    assert len(templates) == 1
    # Admin should be able to edit template created by user1
    name_edit=random_name()
    user_clients["admin"].update(node_template, name=name_edit,
                                 digitaloceanConfig=
                                 {"region": "nyc3",
                                  "size": "2gb",
                                  "image": "ubuntu-16-04-x64"})
    templates = user_clients["standard_user_1"].list_node_template(
                                                name=name_edit)
    assert len(templates) == 1
    # User 2 should not be able to edit template created by user1
    with pytest.raises(ApiError) as e:
        user_clients["standard_user_2"].update(node_template,
                                               name=random_name(),
                                               digitaloceanConfig=
                                               {"region": "nyc3",
                                                "size": "2gb",
                                                "image": "ubuntu-16-04-x64"})
    assert e.value.error.status == 403


@if_test_rbac
def test_rbac_node_template_deploy_cluster(remove_resource):
    # Admin should be able to use template to create cluster
    node_template = create_node_template_do(user_clients["standard_user_1"])
    create_and_validate_do_cluster(node_template)


# -------------- rbac tests for cloud credentials --------------


@if_test_rbac
def test_rbac_cloud_credential_create(remove_resource):
    # As std user, create a node template
    cloud_credential = create_cloud_credential_do(user_clients[
                                                  "standard_user_1"])
    remove_resource(cloud_credential)
    credentials = user_clients["standard_user_1"].list_cloud_credential(
                                                  name=cloud_credential.name)
    assert len(credentials) == 1


@if_test_rbac
def test_rbac_cloud_credential_list(remove_resource):
    # User client should be able to list credential it has created
    cloud_credential = create_cloud_credential_do(user_clients[
                                                  "standard_user_1"])
    remove_resource(cloud_credential)
    credentials = user_clients["standard_user_1"].list_cloud_credential(
                                                  name=cloud_credential.name)
    assert len(credentials) == 1
    # Admin should be able to list credential
    credentials = user_clients["admin"].list_cloud_credential(
                                        name=cloud_credential.name)
    assert len(credentials) == 1
    # User 2 should not be able to list credential
    credentials2 = user_clients["standard_user_2"].list_cloud_credential(
                                                   name=cloud_credential.name)
    assert len(credentials2) == 0


@if_test_rbac
def test_rbac_cloud_credential_delete(remove_resource):
    # User client should be able to delete credential it has created
    cloud_credential = create_cloud_credential_do(user_clients[
                                                  "standard_user_1"])
    # User1 should be able to delete own credential
    user_clients["standard_user_1"].delete(cloud_credential)
    credentials = user_clients["standard_user_1"].list_cloud_credential(
                                                  name=cloud_credential.name)
    assert len(credentials) == 0
    # Admin should be able to delete credential created by user1
    cloud_credential2 = create_cloud_credential_do(user_clients[
                                                   "standard_user_1"])
    user_clients["admin"].delete(cloud_credential2)
    credentials = user_clients["standard_user_1"].list_cloud_credential(
                                                  name=cloud_credential2.name)
    assert len(credentials) == 0
    # User 2 should not be able to delete credential created by user1
    cloud_credential3 = create_cloud_credential_do(user_clients[
                                                   "standard_user_1"])
    remove_resource(cloud_credential3)
    with pytest.raises(ApiError) as e:
        user_clients["standard_user_2"].delete(cloud_credential3)
    assert e.value.error.status == 403


@if_test_rbac
def test_rbac_cloud_credential_edit(remove_resource):
    # User client should be able to edit credential it has created
    cloud_credential = create_cloud_credential_do(user_clients[
                                                  "standard_user_1"])
    remove_resource(cloud_credential)
    # User1 should be able to edit own credential
    do_cloud_credential_config = {"name": "testName1"}
    user_clients["standard_user_1"].update(cloud_credential,
                                           digitaloceancredentialConfig=
                                           do_cloud_credential_config)
    # Admin should be able to edit credential created by user1
    do_cloud_credential_config = {"name": "testname2"}
    user_clients["admin"].update(cloud_credential,
                                 digitaloceancredentialConfig=
                                 do_cloud_credential_config)
    # User 2 should not be able to edit credential created by user1
    with pytest.raises(ApiError) as e:
        do_cloud_credential_config = {"name": "testname3"}
        user_clients["standard_user_2"].update(cloud_credential,
                                               digitaloceancredentialConfig=
                                               do_cloud_credential_config)
    assert e.value.error.status == 403


@if_test_rbac
def test_rbac_cloud_credential_deploy_cluster(remove_resource):
    # Admin should be able to use credential created by user1
    # to create a cluster using a node template
    cloud_credential = create_cloud_credential_do(user_clients[
                                                  "standard_user_1"])
    node_template = create_node_template_do(user_clients["standard_user_1"],
                                            cloud_credential)
    create_and_validate_do_cluster(node_template)


# --------------------- helper functions -----------------------

def create_node_template_do(client, cloud_credential=None):
    if cloud_credential:
        do_cloud_credential = cloud_credential
    else:
        do_cloud_credential_config = {"accessToken": DO_ACCESSKEY}
        do_cloud_credential = client.create_cloud_credential(
            digitaloceancredentialConfig=do_cloud_credential_config
        )
    node_template = client.create_node_template(
        digitaloceanConfig={"region": "nyc3",
                            "size": "2gb",
                            "image": "ubuntu-18-04-x64"},
        name=random_name(),
        driver="digitalocean",
        cloudCredentialId=do_cloud_credential.id,
        engineInstallURL=engine_install_url,
        useInternalIpAddress=True)
    node_template = client.wait_success(node_template)
    return node_template


def create_cloud_credential_do(client):
    do_cloud_credential_config = {"accessToken": DO_ACCESSKEY}
    do_cloud_credential = client.create_cloud_credential(
        digitaloceancredentialConfig=do_cloud_credential_config
    )
    return do_cloud_credential


def create_and_validate_do_cluster(node_template,
                                   rancherKubernetesEngineConfig=rke_config,
                                   attemptDelete=True):
    nodes = []
    node_name = random_node_name()
    node = {"hostnamePrefix": node_name,
            "nodeTemplateId": node_template.id,
            "controlPlane": True,
            "etcd": True,
            "worker": True,
            "quantity": 1,
            "clusterId": None}
    nodes.append(node)
    cluster, node_pools = create_and_validate_cluster(
        user_clients["admin"], nodes, rancherKubernetesEngineConfig,
        clusterName=random_name())
    if attemptDelete:
        cluster_cleanup(user_clients["admin"], cluster)
    else:
        return cluster, node_pools


@pytest.fixture(autouse="True")
def create_project_client(request):

    user_clients["standard_user_1"] = get_user_client()
    user_clients["admin"] = get_admin_client()
    user1, user1_token = create_user(user_clients["admin"])
    user_clients["standard_user_2"] = get_client_for_token(user1_token)

    def fin():
        user_clients["admin"].delete(user1)
    request.addfinalizer(fin)
