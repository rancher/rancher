import pytest
import time
from .common import random_str
from .conftest import wait_for
from rancher import ApiError
from kubernetes.client import CustomObjectsApi


def test_legacy_template_migrate_and_delete(admin_mc, admin_cc,
                                            remove_resource, user_mc,
                                            raw_remove_custom_resource):
    """Asserts that any node template not in cattle-global-nt namespace is
    duplicated into cattle-global-nt, then deleted"""
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
    nt = wait_for(lambda: migrated_template_exists(id), fail_handler=lambda:
                  "failed waiting for node template to migrate")

    # assert that config has not been removed from node template
    assert nt.azureConfig["customData"] == dynamic_nt["azureConfig"]["customData"]
    wait_for(lambda: admin_client.by_id_node_template(id=ns.name + ":" +
             node_template_name) is None, fail_handler=lambda:
             "failed waiting for old node template to delete")


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
            nt = user_client1.by_id_node_template(id=node_template.id)
            return nt
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
