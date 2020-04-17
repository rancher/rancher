import pytest
import kubernetes
from .conftest import random_str, kubernetes_api_client
from rancher import ApiError


def test_cannot_create_azure_no_accountstoragetype(admin_pc, admin_cc,
                                                   admin_mc, remove_resource):
    """Tests that a PVC referencing a storage class with empty skuName and
    storageaccounttype fields fails to create
    """
    client = admin_pc.client
    # using k8s_client is required since rancher client will automatically
    # set default if sc has no storageaccounttype/skuName
    k8s_client = kubernetes_api_client(admin_mc.client, admin_cc.cluster.id)
    storage_client = kubernetes.client.StorageV1Api(api_client=k8s_client)

    ns = admin_pc.cluster.client.create_namespace(
        name="ns" + random_str(),
        projectId=admin_pc.project.id)
    remove_resource(ns)

    sc = storage_client.create_storage_class(
        body={
            "metadata": {
                "name": "sc" + random_str()
            },
            "parameters": {
                "kind": "shared"
            },
            "provisioner": "kubernetes.io/azure-disk"})
    remove_resource(sc)

    with pytest.raises(ApiError) as e:
        client.create_persistent_volume_claim(
            name="pc" + random_str(),
            storageClassId=sc.metadata.name,
            namespaceId=ns.id,
            accessModes=["ReadWriteOnce"],
            resources={
              "requests": {
                "storage": "30Gi"
              }
            })
    assert e.value.error.status == 422
    assert "must provide storageaccounttype or skuName" in \
        e.value.error.message


def test_can_create_azure_any_accountstoragetype(admin_pc, admin_cc,
                                                 remove_resource):
    """Tests that a PVC referencing a storage class with any non-empty skuName or
    storageaccounttype field successfully creates
    """
    cc_client = admin_cc.client
    pc_client = admin_pc.client

    ns = cc_client.create_namespace(
        name="ns" + random_str(),
        projectId=admin_pc.project.id)
    remove_resource(ns)

    # try with storageaccounttype value
    sc1 = cc_client.create_storage_class(
        name="sc" + random_str(),
        provisioner="kubernetes.io/azure-disk",
        kind="shared",
        parameters={
            "storageaccounttype": "asdf",
        },
    )
    remove_resource(sc1)

    pvc1 = pc_client.create_persistent_volume_claim(
        name="pc" + random_str(),
        storageClassId=sc1.name,
        namespaceId=ns.id,
        accessModes=["ReadWriteOnce"],
        resources={
          "requests": {
            "storage": "30Gi"
          }
        })
    remove_resource(pvc1)

    # try with skuName value
    sc2 = cc_client.create_storage_class(
        name="sc" + random_str(),
        provisioner="kubernetes.io/azure-disk",
        parameters={
            "skuName": "asdf",
        },
    )
    remove_resource(sc2)

    pvc2 = pc_client.create_persistent_volume_claim(
        name="pc" + random_str(),
        storageClassId=sc2.name,
        namespaceId=ns.id,
        accessModes=["ReadWriteOnce"],
        resources={
            "requests": {
                "storage": "30Gi"
            }
        })
    remove_resource(pvc2)


def test_can_create_pvc_no_storage_no_vol(admin_pc, remove_resource):
    """Tests that a PVC referencing no storage class and no volume
       can be created
    """
    ns = admin_pc.cluster.client.create_namespace(
        name="ns" + random_str(),
        projectId=admin_pc.project.id)
    remove_resource(ns)

    pvc = admin_pc.client.create_persistent_volume_claim(
        name="pc" + random_str(),
        namespaceId=ns.id,
        accessModes=["ReadWriteOnce"],
        resources={
            "requests": {
                "storage": "30Gi"
            }
        })
    remove_resource(pvc)
    assert pvc is not None
    assert pvc.state == "pending"
