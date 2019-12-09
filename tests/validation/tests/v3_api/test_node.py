import pytest

from .common import *  # NOQA

def test_add_node_label():
    client, cluster = get_global_admin_client_and_cluster()
    test_label = "foo"
    nodes = client.list_node(clusterId=cluster.id)
    assert len(nodes.data) > 0
    node_id = nodes.data[0].id
    node = client.by_id_node(node_id)

    # Make sure there is no test label and add test label
    node_labels = node.labels.data_dict()
    assert test_label not in node_labels

    node_labels[test_label] = "bar"
    client.update(node, labels=node_labels)

    # Label should be added
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    assert node_labels[test_label] == "bar"

     # Label should be delete
    del node_labels[test_label]
    client.update(node, labels=node_labels)
    wait_for_condition(client, node, check_label_removed(test_label), None, 10)
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    assert test_label not in node_labels

def check_label_added(test_label):
    def _find_condition(resource):
        node_labels = resource.labels.data_dict()

        if test_label in node_labels:
            return True
        else: 
            return False 

    return _find_condition

def check_label_removed(test_label):
    def _find_condition(resource):
        node_labels = resource.labels.data_dict()

        if test_label not in node_labels:
            return True
        else: 
            return False 

    return _find_condition