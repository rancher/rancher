import pytest
import time

from .common import random_str
from .conftest import wait_for
from rancher import ApiError
from rancher import RestObject
from kubernetes.client import CustomObjectsApi
from kubernetes.client.rest import ApiException


def test_legacy_template_migrate_and_delete(admin_mc, admin_cc,
                                            remove_resource, user_mc,
                                            raw_remove_custom_resource):
    """Asserts that any node template not in cattle-global-nt namespace is
    duplicated into cattle-global-nt, then deleted. Also, asserts that
    operations on legacy node templates are forwarded to corresponding
    migrated node templates"""
    admin_client = admin_mc.client
    admin_cc_client = admin_cc.client
    user_client = user_mc.client

    k8s_dynamic_client = CustomObjectsApi(admin_mc.k8s_client)

    ns = admin_cc_client.create_namespace(name="ns-" + random_str(),
                                          clusterId=admin_cc.cluster.id)
    remove_resource(ns)

    node_template_name = "nt-" + random_str()
    body = {
        "metadata": {
            "name": node_template_name,
            "annotations": {
                "field.cattle.io/creatorId": user_mc.user.id
            }
        },
        "kind": "NodeTemplate",
        "apiVersion": "management.cattle.io/v3",
        "azureConfig": {"customData": "asdfsadfsd"}
    }

    # create a node template that will be recognized as legacy
    dynamic_nt = k8s_dynamic_client.create_namespaced_custom_object(
        "management.cattle.io", "v3", ns.name, 'nodetemplates', body)
    raw_remove_custom_resource(dynamic_nt)

    def migrated_template_exists(id):
        try:
            nt = user_client.by_id_node_template(id=id)
            remove_resource(nt)
            return nt
        except ApiError as e:
            assert e.error.status == 403
            return False
    id = "cattle-global-nt:nt-" + ns.id + "-" + dynamic_nt["metadata"]["name"]
    legacy_id = dynamic_nt["metadata"]["name"]
    legacy_ns = dynamic_nt["metadata"]["namespace"]
    full_legacy_id = legacy_ns + ":" + legacy_id

    # wait for node template to be migrated
    nt = wait_for(lambda: migrated_template_exists(id), fail_handler=lambda:
                  "failed waiting for node template to migrate")

    # assert that config has not been removed from node template
    assert nt.azureConfig["customData"] ==\
        dynamic_nt["azureConfig"]["customData"]

    def legacy_template_deleted():
        try:
            k8s_dynamic_client.get_namespaced_custom_object(
                "management.cattle.io", "v3", ns.name, 'nodetemplates',
                legacy_id)
            return False
        except ApiException as e:
            return e.status == 404

    # wait for legacy node template to be deleted
    wait_for(lambda: legacy_template_deleted(), fail_handler=lambda:
             "failed waiting for old node template to delete")

    # retrieve node template via legacy id
    nt = admin_client.by_id_node_template(id=full_legacy_id)
    # retrieve node template via migrated id
    migrated_nt = admin_client.by_id_node_template(id=id)

    def compare(d1, d2):
        if d1 == d2:
            return True
        if d1.keys() != d2.keys():
            return False
        for key in d1.keys():
            if key in ["id", "namespace", "links", "annotations"]:
                continue
            if d1[key] == d2[key]:
                continue
            if callable(d1[key]):
                continue
            if isinstance(d1[key], RestObject):
                if compare(d1[key], d1[key]):
                    continue
            return False
        return True

    # ensure templates returned are identical aside from fields containing
    # id/ns
    if not compare(nt, migrated_nt):
        raise Exception("forwarded does not match migrated nodetemplate")

    nt.azureConfig.customData = "asdfasdf"
    new_config = nt.azureConfig
    new_config.customData = "adsfasdfadsf"

    # update node template via legacy id
    nt = admin_client.update_by_id_node_template(
        id=full_legacy_id,
        azureConfig=new_config)

    # assert node template is being updated
    assert nt.azureConfig.customData == new_config.customData
    nt2 = admin_client.by_id_node_template(id=id)
    # assert node template being updated is migrated node template
    assert nt2.azureConfig.customData == new_config.customData

    # delete node template via legacy id
    admin_client.delete(nt)
    wait_for(lambda: admin_client.by_id_node_template(id) is None,
             fail_handler=lambda:
             "failed waiting for migrate node template to delete")


def test_node_template_namespace(admin_mc, remove_resource):
    """asserts that node template is automatically created in
    'cattle-global-nt' namespace"""
    admin_client = admin_mc.client

    node_template = admin_client.create_node_template(name="nt-" +
                                                      random_str(),
                                                      azureConfig={})
    remove_resource(node_template)
    assert node_template.id.startswith("cattle-global-nt")


def test_user_can_only_view_own_template(user_factory, remove_resource):
    """Asserts that user can view template after they have created it"""
    user_client1 = user_factory().client
    user_client2 = user_factory().client

    node_template = user_client1.create_node_template(name="nt-" +
                                                      random_str(),
                                                      azureConfig={})
    remove_resource(node_template)

    def can_view_template():
        try:
            return user_client1.by_id_node_template(id=node_template.id)
        except ApiError as e:
            assert e.error.status == 403
            return None

    wait_for(can_view_template, fail_handler=lambda:
             "creator was unable to view node template")

    # assert user cannot view template created by another user
    ensure_user_cannot_view_template(user_client2, node_template.id)


def ensure_user_cannot_view_template(client, nodeTemplateId, timeout=5):
    """Asserts user is unable to view node template associated with given node
    template id"""
    can_view = False
    start = time.time()
    interval = 0.2
    while not can_view:
        if time.time() - start > timeout:
            return
        with pytest.raises(ApiError) as e:
            client.by_id_node_template(nodeTemplateId)

        assert e.value.error.status == 403
        time.sleep(interval)
        interval *= 2
