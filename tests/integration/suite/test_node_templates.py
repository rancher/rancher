import pytest
import time

from .common import random_str
from .conftest import wait_for
from rancher import ApiError
from rancher import RestObject
from kubernetes.client import CustomObjectsApi
from kubernetes.client.rest import ApiException


def test_node_template_namespace(admin_mc, remove_resource):
    """asserts that node template is automatically created in
    'cattle-global-nt' namespace"""
    admin_client = admin_mc.client

    node_template = admin_client.create_node_template(name="nt-" +
                                                      random_str(),
                                                      azureConfig={})
    remove_resource(node_template)
    assert node_template.id.startswith("cattle-global-nt")


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
