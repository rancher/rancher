import pytest
import time
from .common import create_kubeconfig
from .common import CLUSTER_MEMBER
from .common import CLUSTER_OWNER
from .common import PROJECT_MEMBER
from .common import PROJECT_OWNER
from .common import PROJECT_READ_ONLY
from .common import get_client_for_token
from .common import delete_node
from .common import get_user_client
from .common import get_user_client_and_cluster
from .common import execute_kubectl_cmd
from .common import if_test_rbac
from .common import random_name
from .common import random_test_name
from .common import rbac_get_user_token_by_role
from .common import validate_cluster_state
from .common import wait_for_condition
from .conftest import wait_for_cluster_delete
from rancher import ApiError
from .test_rke_cluster_provisioning import DO_ACCESSKEY
from .test_rke_cluster_provisioning import evaluate_clustername
from .test_rke_cluster_provisioning import get_custom_host_registration_cmd
from .test_rke_cluster_provisioning import HOST_NAME
from .test_rke_cluster_provisioning import random_node_name
from .test_rke_cluster_provisioning import rke_config
from .test_rke_cluster_provisioning import wait_for_cluster_node_count
from lib.aws import AmazonWebServices

cluster_detail = {"cluster": None, "client": None}
cluster_node_template = {"cluster": None, "node_pools": None,
                         "node_template": None, "do_cloud_credential": None,
                         "label_value": None, "test_label": None}
cluster_custom = {"cluster": None, "test_label": None,
                  "label_value": None, "aws_node": None}
custom_cluster_add_edit = {"cluster": None, "aws_node": []}
cluster_node_template_2 = {"cluster": [], "node_template": []}
roles = [CLUSTER_MEMBER, CLUSTER_OWNER, PROJECT_OWNER, PROJECT_MEMBER,
         PROJECT_READ_ONLY]


def test_node_label_add():
    test_label = random_name()
    label_value = random_name()
    # get node details
    client, node = get_node_details()

    # add label through API
    node_labels = node.labels.data_dict()
    node_labels[test_label] = label_value
    client.update(node, labels=node_labels)

    # Label should be added
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, label_value)

    # delete label
    del node_labels[test_label]
    client.update(node, labels=node_labels)


def test_node_label_edit():
    test_label = random_name()
    label_value = random_name()
    # get node details
    client, node = get_node_details()

    # add label through API
    node_labels = node.labels.data_dict()
    node_labels[test_label] = label_value
    client.update(node, labels=node_labels)
    time.sleep(2)

    # Label should be added
    node = client.reload(node)
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, label_value)

    # edit label through API
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    new_value = random_name()
    node_labels[test_label] = new_value
    client.update(node, labels=node_labels)
    node = client.reload(node)
    time.sleep(2)

    # Label should be added
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, new_value)

    # delete label
    del node_labels[test_label]
    client.update(node, labels=node_labels)


def test_node_label_delete():
    test_label = random_name()
    label_value = random_name()
    # get node details
    client, node = get_node_details()

    # add labels on node
    node_labels = node.labels.data_dict()
    node_labels[test_label] = label_value
    client.update(node, labels=node_labels)
    time.sleep(2)

    # Label should be added
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, label_value)

    # delete label
    del node_labels[test_label]
    client.update(node, labels=node_labels)
    time.sleep(2)

    # label should be deleted
    wait_for_condition(client, node, check_label_removed(test_label), None, 10)
    validate_label_deleted_on_node(client, node, test_label)


def test_node_label_kubectl_add():
    test_label = random_name()
    label_value = random_name()
    # get node details
    client, node = get_node_details()
    node_name = node.nodeName

    # add label on node
    command = "label nodes " + node_name + " " + test_label + "=" + label_value
    print(command)
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # Label should be added
    node = client.reload(node)
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, label_value)

    # remove label
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    del node_labels[test_label]
    client.update(node, labels=node_labels)


def test_node_label_kubectl_edit():
    test_label = random_name()
    label_value = random_name()
    # get node details
    client, node = get_node_details()
    node_name = node.nodeName

    # add label on node
    command = "label nodes " + node_name + " " + test_label + "=" + label_value
    print(command)
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # Label should be added
    node = client.reload(node)
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, label_value)

    # edit label through kubectl
    new_value = random_name()
    command = "label nodes " + node_name + " " + \
              test_label + "=" + new_value + " --overwrite"
    print(command)
    execute_kubectl_cmd(command, False)
    node = client.reload(node)
    time.sleep(2)

    # New Label should be added
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, label_value=new_value)

    # remove label
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    del node_labels[test_label]
    client.update(node, labels=node_labels)


def test_node_label_kubectl_delete():
    test_label = random_name()
    label_value = random_name()
    # get node details
    client, node = get_node_details()
    node_name = node.nodeName

    # add label on node
    command = "label nodes " + node_name + " " + test_label + "=" + label_value
    print(command)
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # Label should be added
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, label_value)

    # remove label through kubectl
    command = " label node " + node_name + " " + test_label + "-"
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # label should be deleted
    wait_for_condition(client, node, check_label_removed(test_label), None, 10)
    validate_label_deleted_on_node(client, node, test_label)


def test_node_label_k_add_a_delete_k_add():
    """Add via kubectl, Delete via API, Add via kubectl"""
    test_label = random_name()
    label_value = random_name()
    # get node details
    client, node = get_node_details()
    node_name = node.nodeName

    command = "label nodes " + node_name + " " + test_label + "=" + label_value
    print(command)
    execute_kubectl_cmd(command, False)

    # Label should be added
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, label_value)

    # delete label
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    del node_labels[test_label]
    client.update(node, labels=node_labels)

    # label should be deleted
    wait_for_condition(client, node, check_label_removed(test_label), None, 10)
    validate_label_deleted_on_node(client, node, test_label)

    # Add label via kubectl
    execute_kubectl_cmd(command, False)

    # Label should be added
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, label_value)

    # clean up label
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    del node_labels[test_label]
    client.update(node, labels=node_labels)


def test_node_label_k_add_a_edit_k_edit():
    """Add via kubectl, edit via API, edit via kubectl"""
    test_label = random_name()
    label_value = random_name()
    # get node details
    client, node = get_node_details()
    node_name = node.nodeName

    command = "label nodes " + node_name + " " + test_label + "=" + label_value
    execute_kubectl_cmd(command, False)

    # Label should be added
    node = client.reload(node)
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, label_value)

    # edit label through API
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    new_value = random_name()
    node_labels[test_label] = new_value
    client.update(node, labels=node_labels)
    time.sleep(2)

    # Label should be added
    node = client.reload(node)
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, new_value)

    # edit label through kubectl
    new_value_2 = random_name()
    command = "label nodes " + node_name + " " + \
              test_label + "=" + new_value_2 + " --overwrite"
    print(command)
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # New Label should be added
    node = client.reload(node)
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, new_value_2)

    # remove label
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    del node_labels[test_label]
    client.update(node, labels=node_labels)


def test_node_label_a_add_k_delete_a_add():
    """Add via API, Delete via kubectl, Add via API"""
    test_label = random_name()
    label_value = random_name()
    # get node details
    client, node = get_node_details()
    node_name = node.nodeName

    node_labels = node.labels.data_dict()
    node_labels[test_label] = label_value
    client.update(node, labels=node_labels)
    time.sleep(2)

    # Label should be added
    node = client.reload(node)
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, label_value)

    # delete label
    command = " label node " + node_name + " " + test_label + "-"
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # label should be deleted
    node = client.reload(node)
    wait_for_condition(client, node, check_label_removed(test_label), None, 10)
    validate_label_deleted_on_node(client, node, test_label)

    # Add label via API
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    node_labels[test_label] = label_value
    client.update(node, labels=node_labels)
    time.sleep(2)

    # Label should be added
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, label_value)

    # clean up label
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    del node_labels[test_label]
    client.update(node, labels=node_labels)


def test_node_label_a_add_k_edit_a_edit():
    """Add via API, Edit via kubectl, Edit via API"""
    test_label = random_name()
    label_value = random_name()
    # get node details
    client, node = get_node_details()
    node_name = node.nodeName

    node_labels = node.labels.data_dict()
    node_labels[test_label] = label_value
    client.update(node, labels=node_labels)
    time.sleep(2)

    # Label should be added
    node = client.reload(node)
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, label_value)

    # edit label through kubectl
    new_value = random_name()
    command = "label nodes " + node_name + " " + \
              test_label + "=" + new_value + " --overwrite"
    print(command)
    execute_kubectl_cmd(command, False)
    node = client.reload(node)
    time.sleep(2)

    # New Label should be added
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, label_value=new_value)

    # edit label through API
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    new_value_2 = random_name()
    node_labels[test_label] = new_value_2
    client.update(node, labels=node_labels)
    node = client.reload(node)
    time.sleep(2)

    # Label should be added
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, new_value_2)

    # clean up label
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    del node_labels[test_label]
    client.update(node, labels=node_labels)


def test_node_label_custom_add_edit_addnode():
    """ Create a custom cluster, Add node labels via register command
    Edit via API and change the existing label value ONLY
    Add a control plane node with label same as the ORIGINAL one
    And check the labels on all the nodes."""
    test_label = random_name()
    label_value = random_name()
    cluster_custom["test_label"] = test_label
    cluster_custom["label_value"] = label_value
    client = cluster_detail["client"]
    node_roles = [["worker", "controlplane", "etcd"]]
    aws_nodes_list = []
    cluster, aws_nodes = \
        create_custom_node_label(node_roles, test_label, label_value, True)
    create_kubeconfig(cluster)
    for node in aws_nodes:
        aws_nodes_list.append(node)

    nodes = client.list_node(clusterId=cluster.id).data
    node = nodes[0]

    validate_label_set_on_node(client, node, test_label, label_value)

    node_name_1 = node.nodeName
    # edit label through API
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    new_value_2 = random_name()
    node_labels[test_label] = new_value_2
    client.update(node, labels=node_labels)
    # cluster will go into updating state
    cluster = validate_cluster_state(client, cluster, True,
                                     intermediate_state="updating",
                                     nodes_not_in_active_state=[])

    node = client.reload(node)
    # Label should be added
    validate_label_set_on_node(client, node, test_label, new_value_2)

    # add a control plane node with original label
    aws_nodes = \
        aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            1, random_test_name(HOST_NAME))
    for node in aws_nodes:
        aws_nodes_list.append(node)

    aws_node = aws_nodes[0]
    docker_run_cmd = get_custom_host_registration_cmd(client, cluster,
                                                      ["controlplane"],
                                                      aws_node)
    docker_run_cmd = \
        docker_run_cmd + " --label " + test_label + "=" + label_value
    aws_node.execute_command(docker_run_cmd)
    wait_for_cluster_node_count(client, cluster, 2)
    cluster = validate_cluster_state(client, cluster,
                                     intermediate_state="updating")
    nodes = client.list_node(clusterId=cluster.id).data
    # cluster cleanup
    custom_cluster_add_edit["cluster"] = cluster
    custom_cluster_add_edit["aws_node"] = aws_nodes_list

    for node in nodes:
        if node.nodeName != node_name_1:
            validate_label_set_on_node(client, node, test_label, label_value)
        else:
            validate_label_set_on_node(client, node, test_label, new_value_2)


def test_node_label_node_template_add():
    """
    This test validates label added through node template,
    add label on node template, and validates the label
    is available on the scaled up node
    :return: None
    """
    client = cluster_detail["client"]
    cluster = cluster_node_template["cluster"]
    create_kubeconfig(cluster)
    nodes = client.list_node(clusterId=cluster.id).data
    # get existing nodes info
    existing_labels = {}
    for node in nodes:
        existing_labels[node.nodeName] = {}
        existing_labels[node.nodeName] = node.labels.data_dict()

    test_label = random_name()
    label_value = random_name()
    # create a node template with a label
    node_template_new, do_cloud_credential = \
        create_node_template_label(client, test_label, label_value)
    # Add a node in cluster
    cluster, node_pools = add_node_cluster(node_template_new, cluster)
    nodes = client.list_node(clusterId=cluster.id).data

    # validate labels on nodes
    for node in nodes:
        if node.nodeName not in existing_labels.keys():
            # check if label is set on node
            validate_label_set_on_node(client, node, test_label, label_value)
        else:
            # check if the labels on the existing nodes are intact
            assert existing_labels[node.nodeName] == node.labels.data_dict(), \
                "Labels on existing nodes have changed"


@pytest.mark.run(after='test_node_label_node_template_add')
def test_node_label_node_template_edit():
    """
    This test validates label added through node template,
    edit label on node template, and validates new label
    is available on the scaled up node
    :param remove_resource: to delete the resource
    :return:
    """
    client = cluster_detail["client"]
    cluster = cluster_node_template["cluster"]
    node_template = cluster_node_template["node_template"]
    do_cloud_credential = cluster_node_template["do_cloud_credential"]
    test_label = cluster_node_template["test_label"]
    create_kubeconfig(cluster)
    nodes = client.list_node(clusterId=cluster.id).data

    existing_labels = {}
    for node in nodes:
        existing_labels[node.nodeName] = {}
        existing_labels[node.nodeName] = node.labels.data_dict()

    template_label = node_template.labels.data_dict()
    assert test_label in template_label, \
        "Label is NOT available on the node template"

    new_value = random_name()
    template_label[test_label] = new_value
    node_template_new = client.update(node_template, labels=template_label,
                                      cloudCredentialId=do_cloud_credential.id,
                                      digitaloceanConfig=
                                      {"region": "nyc3",
                                       "size": "2gb",
                                       "image": "ubuntu-16-04-x64"})
    node_template_new = client.wait_success(node_template_new)
    assert test_label in node_template_new["labels"], \
        "Label is not set on node template"
    assert node_template_new["labels"][test_label] == new_value

    # Add a node in cluster
    cluster, node_pools = add_node_cluster(node_template_new, cluster)

    nodes = client.list_node(clusterId=cluster.id).data
    """check original label on the first node,
    and the new label on the added node"""

    # validate labels on nodes
    for node in nodes:
        if node.nodeName not in existing_labels.keys():
            # check if label is set on node
            validate_label_set_on_node(client, node, test_label, new_value)
        else:
            # check if the labels on the existing nodes are intact
            assert existing_labels[node.nodeName] == node.labels.data_dict(), \
                "Labels on existing nodes have changed"


@pytest.mark.run(after='test_node_label_node_template_edit')
def test_node_label_node_template_delete():
    """
    This test validates label added through node template,
    delete label on node template, and validates the label
    is NOT available on the scaled up node
    :return: None
    """
    client = cluster_detail["client"]
    cluster = cluster_node_template["cluster"]
    node_template = cluster_node_template["node_template"]
    do_cloud_credential = cluster_node_template["do_cloud_credential"]
    test_label = cluster_node_template["test_label"]
    create_kubeconfig(cluster_node_template["cluster"])

    nodes = client.list_node(clusterId=cluster.id).data

    existing_labels = {}
    for node in nodes:
        existing_labels[node.nodeName] = {}
        existing_labels[node.nodeName] = node.labels.data_dict()

    # delete label in node template
    template_label = node_template.labels.data_dict()
    del template_label[test_label]

    # update node template
    node_template_new = client.update(node_template, labels=template_label,
                                      cloudCredentialId=do_cloud_credential.id,
                                      digitaloceanConfig=
                                      {"region": "nyc3",
                                       "size": "2gb",
                                       "image": "ubuntu-16-04-x64"})
    node_template_new = client.wait_success(node_template_new)
    assert test_label not in node_template_new["labels"], \
        "Label is available on the node template"

    # Add a node in cluster with new node template
    cluster, node_pools = add_node_cluster(node_template_new, cluster)

    nodes = client.list_node(clusterId=cluster.id).data

    # validate labels on nodes
    for node in nodes:
        if node.nodeName not in existing_labels.keys():
            node_labels = node.labels.data_dict()
            assert test_label not in node_labels, \
                "Label is NOT deleted on the node"
        else:
            # check if the labels on the existing nodes are intact
            assert existing_labels[node.nodeName] == node.labels.data_dict(), \
                "Labels on existing nodes have changed"


def test_node_label_node_template_edit_api():
    """
    This test validates edit of label via API
    which is added through node template
    :return: None
    """
    test_label = random_name()
    label_value = random_name()
    cluster, node_pools, node_template, do_cloud_credential = \
        create_cluster_node_template_label(test_label, label_value)
    client = get_user_client()
    cluster_node_template_2["cluster"].append(cluster)
    cluster_node_template_2["node_template"].append(node_template)
    create_kubeconfig(cluster)

    node = client.list_node(clusterId=cluster.id).data
    node_id = node[0].id
    node = client.by_id_node(node_id)
    # Edit label on node via API
    node_labels = node.labels.data_dict()
    assert node_labels[test_label] == label_value

    # edit label through API
    new_value = random_name()
    node_labels[test_label] = new_value
    client.update(node, labels=node_labels)
    node = client.reload(node)
    time.sleep(2)

    # Label should be added
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, test_label, new_value)


def test_node_label_node_template_delete_api():
    """
    This test validates delete of label via API
    which is added through node template
    :return: None
    Expected failure because of issue -
    https://github.com/rancher/rancher/issues/26604
    """
    test_label = random_name()
    label_value = random_name()
    cluster, node_pools, node_template, do_cloud_credential = \
        create_cluster_node_template_label(test_label, label_value)
    client = get_user_client()
    cluster_node_template_2["cluster"].append(cluster)
    cluster_node_template_2["node_template"].append(node_template)
    create_kubeconfig(cluster)

    node = client.list_node(clusterId=cluster.id).data
    node_id = node[0].id
    node = client.by_id_node(node_id)
    node_labels = node.labels.data_dict()
    assert node_labels[test_label] == label_value

    # delete label
    del node_labels[test_label]
    client.update(node, labels=node_labels)
    # cluster will go into updating state
    cluster = validate_cluster_state(client, cluster, True,
                                     intermediate_state="updating",
                                     nodes_not_in_active_state=[])

    node = client.reload(node)
    # label should be deleted
    validate_label_deleted_on_node(client, node, test_label)


def test_node_label_custom_add():
    """
    This test validates the label on a custom node
    added through the registration command
    :return:
    """
    test_label = random_name()
    label_value = random_name()
    cluster_custom["test_label"] = test_label
    cluster_custom["label_value"] = label_value
    client = cluster_detail["client"]
    node_roles = [["worker", "controlplane", "etcd"]]
    if cluster_custom["cluster"] is None:
        cluster_custom["cluster"], aws_nodes = \
            create_custom_node_label(node_roles, test_label, label_value, True)
        cluster = cluster_custom["cluster"]
        cluster_custom["aws_node"] = aws_nodes
    else:
        cluster = cluster_custom["cluster"]
    create_kubeconfig(cluster_custom["cluster"])
    nodes = client.list_node(clusterId=cluster.id).data
    node = nodes[0]

    validate_label_set_on_node(client, node, test_label, label_value)


@pytest.mark.run(after='test_node_label_custom_add')
def test_node_label_custom_edit():
    """
    This test validates edit on the label on the node -
    added through custom registration command
    :return: None
    """
    create_kubeconfig(cluster_custom["cluster"])
    client = cluster_detail["client"]
    cluster = cluster_custom["cluster"]
    test_label = cluster_custom["test_label"]

    nodes = client.list_node(clusterId=cluster.id).data
    assert len(nodes) > 0
    node_id = nodes[0].id
    node = client.by_id_node(node_id)

    # edit label through API
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    new_value = random_name()
    node_labels[test_label] = new_value
    client.update(node, labels=node_labels)
    # cluster will go into updating state
    cluster = validate_cluster_state(client, cluster, True,
                                     intermediate_state="updating",
                                     nodes_not_in_active_state=[])
    node = client.reload(node)

    validate_label_set_on_node(client, node, test_label, new_value)
    cluster_custom["label_value"] = new_value


@pytest.mark.run(after='test_node_label_custom_edit')
def test_node_label_custom_add_additional():
    """
    This test validates addition of labels on the custom nodes
    :return: None
    """
    create_kubeconfig(cluster_custom["cluster"])
    client = cluster_detail["client"]
    cluster = cluster_custom["cluster"]
    test_label = cluster_custom["test_label"]
    label_value = cluster_custom["label_value"]
    new_label = random_name()
    label_value_new = random_name()

    nodes = client.list_node(clusterId=cluster.id).data
    assert len(nodes) > 0
    node_id = nodes[0].id
    node = client.by_id_node(node_id)

    node_labels = node.labels.data_dict()
    node_labels[new_label] = label_value_new
    client.update(node, labels=node_labels)
    time.sleep(2)

    # Label should be added
    wait_for_condition(client, node, check_label_added(test_label), None, 10)
    validate_label_set_on_node(client, node, new_label, label_value_new)
    validate_label_set_on_node(client, node, test_label, label_value)

    # remove label
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    del node_labels[new_label]
    client.update(node, labels=node_labels)


@pytest.mark.run(after='test_node_label_custom_add_additional')
def test_node_label_custom_delete():
    """
    This test deletes the label on the node via API
    :return: None
    Expected failure because of issue -
    https://github.com/rancher/rancher/issues/26604
    """
    create_kubeconfig(cluster_custom["cluster"])
    client = cluster_detail["client"]
    cluster = cluster_custom["cluster"]
    test_label = cluster_custom["test_label"]

    nodes = client.list_node(clusterId=cluster.id).data
    assert len(nodes) > 0
    node_id = nodes[0].id
    node = client.by_id_node(node_id)

    # remove label
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    del node_labels[test_label]
    client.update(node, labels=node_labels)
    # cluster will go into updating state
    cluster = validate_cluster_state(client, cluster, True,
                                     intermediate_state="updating",
                                     nodes_not_in_active_state=[])

    node = client.reload(node)
    # label should be deleted
    validate_label_deleted_on_node(client, node, test_label)


@if_test_rbac
@pytest.mark.parametrize("role", roles)
def test_rbac_node_label_add(role):
    test_label = random_name()
    label_value = random_name()
    # get node details
    client, node = get_node_details()
    node_labels = node.labels.data_dict()

    # get user token and client
    token = rbac_get_user_token_by_role(role)
    print("token: ", token)
    user_client = get_client_for_token(token)

    node_labels[test_label] = label_value

    if role == CLUSTER_OWNER:
        user_client.update(node, labels=node_labels)
        time.sleep(2)

        # Label should be added
        wait_for_condition(user_client, node,
                           check_label_added(test_label), None, 10)
        validate_label_set_on_node(user_client, node, test_label, label_value)
    else:
        with pytest.raises(ApiError) as e:
            user_client.update(node, labels=node_labels)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'


@if_test_rbac
@pytest.mark.parametrize("role", roles)
def test_rbac_node_label_add_kubectl(role):
    test_label = random_name()
    label_value = random_name()
    # get node details
    client, node = get_node_details()
    node_name = node.nodeName

    # get user token and client
    token = rbac_get_user_token_by_role(role)
    user_client = get_client_for_token(token)
    print(cluster_detail["cluster"]["id"])
    print(cluster_detail["cluster"])
    cluster = user_client.list_cluster(id=cluster_detail["cluster"]["id"]).data
    print(cluster)
    create_kubeconfig(cluster[0])

    # add label on node
    command = "label nodes " + node_name + " " + test_label + "=" + label_value

    if role == CLUSTER_OWNER:
        execute_kubectl_cmd(command, False)
        time.sleep(2)
        # Label should be added
        wait_for_condition(user_client, node,
                           check_label_added(test_label), None, 10)
        validate_label_set_on_node(user_client, node, test_label, label_value)
    elif role == CLUSTER_MEMBER:
        result = execute_kubectl_cmd(command, False, stderr=True)
        result = result.decode('ascii')
        assert "cannot patch resource \"nodes\"" in result
        assert "forbidden" in result
    else:
        result = execute_kubectl_cmd(command, False, stderr=True)
        result = result.decode('ascii')
        assert "cannot get resource \"nodes\"" in result
        assert "forbidden" in result


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    cluster_detail["client"], cluster_detail["cluster"] = \
        get_user_client_and_cluster()
    test_label = random_name()
    label_value = random_name()

    """
    Create a cluster for node template related test cases
    """
    cluster_node_template["cluster"], \
    node_pools, \
    cluster_node_template["node_template"], \
    cluster_node_template["do_cloud_credential"] = \
        create_cluster_node_template_label(test_label, label_value)
    cluster_node_template["node_pools"] = node_pools[0]
    cluster_node_template["test_label"] = test_label
    cluster_node_template["label_value"] = label_value

    def fin():
        client = get_user_client()
        cluster = cluster_node_template["cluster"]
        if cluster is not None:
            node_pools_list = client.list_nodePool(clusterId=cluster.id).data
            # get unique node template ids
            client.delete(cluster_node_template["cluster"])
            wait_for_cluster_delete(client, cluster["name"])
            time.sleep(10)
            unique_node_pool = {}
            for node_pool in node_pools_list:
                if node_pool.nodeTemplateId not in unique_node_pool.keys():
                    unique_node_pool[node_pool.nodeTemplateId] = \
                        client.list_node_template(
                            id=node_pool.nodeTemplateId).data[0]
            print("unique_node_pool: ", unique_node_pool)
            for key, value in unique_node_pool.items():
                client.delete(value)
        if cluster_custom["cluster"] is not None:
            client.delete(cluster_custom["cluster"])
        if cluster_custom["aws_node"] is not None:
            delete_node(cluster_custom["aws_node"])
        if custom_cluster_add_edit["cluster"] is not None:
            client.delete(custom_cluster_add_edit["cluster"])
        if custom_cluster_add_edit["aws_node"] is not None:
            delete_node(custom_cluster_add_edit["aws_node"])
        if len(cluster_node_template_2["cluster"]) != 0:
            for cluster in cluster_node_template_2["cluster"]:
                client.delete(cluster)
                wait_for_cluster_delete(client, cluster.name)
        time.sleep(10)
        for node_template in cluster_node_template_2["node_template"]:
            client.reload(node_template)
            client.delete(node_template)

    request.addfinalizer(fin)


def check_cluster_deleted(client):
    def _find_condition(resource):
        cluster = client.reload(resource)
        if len(cluster["data"]) == 0:
            return True
        else:
            return False

    return _find_condition


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


def validate_label_set_on_node(client, node, test_label, label_value):
    """
    This method checks if the label is added on the node via API and kubectl
    :param client: user client
    :param node: node on which user has to validate if the label is added
    :param test_label: Label to be validated on the node
    :param label_value: label value to be checked
    :return: None
    """
    print("label_value: ", label_value)
    # check via API
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    assert node_labels[test_label] == label_value

    # check via kubectl
    node_name = node.nodeName
    command = " get nodes " + node_name
    node_detail = execute_kubectl_cmd(command)
    print(node_detail["metadata"]["labels"])
    assert test_label in node_detail["metadata"]["labels"], \
        "Label is not set in kubectl"
    assert node_detail["metadata"]["labels"][test_label] == label_value


def validate_label_deleted_on_node(client, node, test_label):
    """
    This method checks if the label is deleted on the node via API and kubectl
    :param client: user client
    :param node: node on which user has to validate if the label is deleted
    :param test_label: Label to be validated on the node
    :return: None
    """
    # check via API
    node = client.reload(node)
    node_labels = node.labels.data_dict()
    assert test_label not in node_labels

    # check via kubectl
    node_name = node.nodeName
    command = " get nodes " + node_name
    print(command)
    node_detail = execute_kubectl_cmd(command)
    print(node_detail["metadata"]["labels"])
    assert test_label not in node_detail["metadata"]["labels"]


def add_node_cluster(node_template, cluster):
    """
    This method adds a node pool to a given cluster
    :param node_template: node pool uses this to create a node
    :param cluster: node pool is added to this cluster
    :return: cluster, node_pools
    """
    client = get_user_client()
    nodes = []
    node_name = random_node_name()
    node = {"hostnamePrefix": node_name,
            "nodeTemplateId": node_template.id,
            "controlPlane": False,
            "etcd": False,
            "worker": True,
            "quantity": 1,
            "clusterId": None}
    nodes.append(node)
    node_pools = []
    for node in nodes:
        node["clusterId"] = cluster.id
        success = False
        start = time.time()
        while not success:
            if time.time() - start > 10:
                raise AssertionError(
                    "Timed out waiting for cluster owner global Roles")
            try:
                time.sleep(1)
                node_pool = client.create_node_pool(**node)
                success = True
            except ApiError:
                success = False
        node_pool = client.wait_success(node_pool)
        node_pools.append(node_pool)

    cluster = validate_cluster_state(client, cluster,
                                     check_intermediate_state=False)
    return cluster, node_pools


def create_cluster_node_template_label(test_label, label_value):
    """
    This method create a node template with the label key and value provided.
    Creates a cluster with nodepool, which uses the above node template.
    Cluster spec: 1 node all roles
    :param test_label: label to add in the node template
    :param label_value: label value
    :return: cluster, node_pools, node_template, do_cloud_credential
    """
    client = get_user_client()
    node_template, do_cloud_credential = \
        create_node_template_label(client, test_label, label_value)
    assert test_label in node_template["labels"], \
        "Label is not set on node template"
    assert node_template["labels"][test_label] == label_value

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
    cluster = client.create_cluster(
        name=random_name(),
        rancherKubernetesEngineConfig=rke_config)
    node_pools = []
    for node in nodes:
        node["clusterId"] = cluster.id
        success = False
        start = time.time()
        while not success:
            if time.time() - start > 10:
                raise AssertionError(
                    "Timed out waiting for cluster owner global Roles")
            try:
                time.sleep(1)
                node_pool = client.create_node_pool(**node)
                success = True
            except ApiError:
                success = False
        node_pool = client.wait_success(node_pool)
        node_pools.append(node_pool)
    cluster = validate_cluster_state(client, cluster)

    return cluster, node_pools, node_template, do_cloud_credential


def create_custom_node_label(node_roles, test_label,
                             label_value, random_cluster_name=False):
    """
    This method creates nodes from AWS and adds the label key and value to
    the register command and deploys a custom cluster.
    :param node_roles: list of node roles for the cluster
    :param test_label: label to add in the docker register command
    :param label_value: label value to add in the docker register command
    :param random_cluster_name: cluster name
    :return: cluster and aws nodes created
    """
    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            len(node_roles), random_test_name(HOST_NAME))

    client = get_user_client()
    cluster_name = random_name() if random_cluster_name \
        else evaluate_clustername()
    cluster = client.create_cluster(name=cluster_name,
                                    driver="rancherKubernetesEngine",
                                    rancherKubernetesEngineConfig=rke_config)
    assert cluster.state == "provisioning"
    i = 0
    for aws_node in aws_nodes:
        docker_run_cmd = \
            get_custom_host_registration_cmd(client, cluster, node_roles[i],
                                             aws_node)
        for nr in node_roles[i]:
            aws_node.roles.append(nr)
        docker_run_cmd = docker_run_cmd + " --label " + \
                         test_label + "=" + label_value
        aws_node.execute_command(docker_run_cmd)
        i += 1
    cluster = validate_cluster_state(client, cluster)
    return cluster, aws_nodes


def get_node_details():
    """
    lists the nodes from the cluster. This cluster has only 1 node.
    :return: client and node object
    """
    create_kubeconfig(cluster_detail["cluster"])
    client = cluster_detail["client"]
    cluster = cluster_detail["cluster"]
    nodes = client.list_node(clusterId=cluster.id).data
    assert len(nodes) > 0
    node_id = nodes[0].id
    node = client.by_id_node(node_id)
    return client, node


def create_node_template_label(client, test_label, label_value):
    """
    This method adds a given label with key: test_label and value: label_value
    to a node template and returns the node template
    :param client: user client
    :param test_label: label to add in the node template
    :param label_value: value of the label to add in the node template
    :return: node template and do cloud credential value
    """
    do_cloud_credential_config = {"accessToken": DO_ACCESSKEY}
    do_cloud_credential = client.create_cloud_credential(
        digitaloceancredentialConfig=do_cloud_credential_config
    )
    node_template = client.create_node_template(
        digitaloceanConfig={"region": "nyc3",
                            "size": "2gb",
                            "image": "ubuntu-16-04-x64"},
        name=random_name(),
        driver="digitalocean",
        cloudCredentialId=do_cloud_credential.id,
        useInternalIpAddress=True,
        labels={"cattle.io/creator": "norman", test_label: label_value})
    node_template = client.wait_success(node_template)
    return node_template, do_cloud_credential
