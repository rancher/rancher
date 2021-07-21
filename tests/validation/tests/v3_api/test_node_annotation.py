import pytest
import time
from .common import create_kubeconfig
from .common import RESTRICTED_ADMIN
from .common import CLUSTER_MEMBER
from .common import CLUSTER_OWNER
from .common import PROJECT_MEMBER
from .common import PROJECT_OWNER
from .common import PROJECT_READ_ONLY
from .common import get_client_for_token
from .common import get_node_details
from .common import get_user_client_and_cluster
from .common import execute_kubectl_cmd
from .common import if_test_rbac
from .common import random_name
from .common import rbac_get_user_token_by_role
from rancher import ApiError

cluster_detail = {"cluster": None, "client": None}
roles = [
    RESTRICTED_ADMIN,
    CLUSTER_MEMBER,
    CLUSTER_OWNER,
    PROJECT_OWNER,
    PROJECT_MEMBER,
    PROJECT_READ_ONLY
]


def test_node_annotation_add():
    annotation_key = random_name()
    annotation_value = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])

    # add annotation through API
    node_annotations = node.annotations.data_dict()
    node_annotations[annotation_key] = annotation_value
    client.update(node, annotations=node_annotations)
    time.sleep(2)
    # annotation should be added
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value)

    # delete annotation
    del node_annotations[annotation_key]
    client.update(node, annotations=node_annotations)


def test_node_annotation_add_multiple():
    annotation_key_1 = random_name()
    annotation_value_1 = random_name()
    annotation_key_2 = random_name()
    annotation_value_2 = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])

    # add annotation through API
    node_annotations = node.annotations.data_dict()
    node_annotations[annotation_key_1] = annotation_value_1
    node_annotations[annotation_key_2] = annotation_value_2
    client.update(node, annotations=node_annotations)
    time.sleep(2)
    # annotation should be added
    validate_annotation_set_on_node(client, node,
                                    annotation_key_1,
                                    annotation_value_1)
    validate_annotation_set_on_node(client, node,
                                    annotation_key_2,
                                    annotation_value_2)

    # delete annotation
    del node_annotations[annotation_key_1]
    del node_annotations[annotation_key_2]
    client.update(node, annotations=node_annotations)


def test_node_annotation_edit():
    annotation_key = random_name()
    annotation_value = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])

    # add annotation through API
    node_annotations = node.annotations.data_dict()
    node_annotations[annotation_key] = annotation_value
    client.update(node, annotations=node_annotations)
    time.sleep(2)

    # annotation should be added
    node = client.reload(node)
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value)

    # edit annotation through API
    node = client.reload(node)
    node_annotations = node.annotations.data_dict()
    new_value = random_name()
    node_annotations[annotation_key] = new_value
    client.update(node, annotations=node_annotations)
    node = client.reload(node)
    time.sleep(2)

    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    new_value)

    # delete annotation
    del node_annotations[annotation_key]
    client.update(node, annotations=node_annotations)


def test_node_annotation_delete():
    annotation_key = random_name()
    annotation_value = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])

    # add annotation on node
    node_annotations = node.annotations.data_dict()
    node_annotations[annotation_key] = annotation_value
    client.update(node, annotations=node_annotations)
    time.sleep(2)

    # annotation should be added
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value)

    # delete annotation
    del node_annotations[annotation_key]
    client.update(node, annotations=node_annotations)
    time.sleep(2)

    # annotation should be deleted
    validate_annotation_deleted_on_node(client, node, annotation_key)


def test_node_annotation_delete_multiple():
    annotation_key_1 = random_name()
    annotation_value_1 = random_name()
    annotation_key_2 = random_name()
    annotation_value_2 = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])

    # add annotation on node
    node_annotations = node.annotations.data_dict()
    node_annotations[annotation_key_1] = annotation_value_1
    node_annotations[annotation_key_2] = annotation_value_2
    client.update(node, annotations=node_annotations)
    time.sleep(2)

    # annotation should be added
    validate_annotation_set_on_node(client, node,
                                    annotation_key_1,
                                    annotation_value_1)
    validate_annotation_set_on_node(client, node,
                                    annotation_key_2,
                                    annotation_value_2)

    # delete annotation
    del node_annotations[annotation_key_1]
    del node_annotations[annotation_key_2]
    client.update(node, annotations=node_annotations)
    time.sleep(2)

    # annotation should be deleted
    validate_annotation_deleted_on_node(client, node, annotation_key_1)
    validate_annotation_deleted_on_node(client, node, annotation_key_2)


def test_node_annotation_kubectl_add():
    annotation_key = random_name()
    annotation_value = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])
    node_name = node.nodeName

    # add annotation on node
    command = "annotate nodes " + node_name + " " + \
              annotation_key + "=" + annotation_value
    print(command)
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # annotation should be added
    node = client.reload(node)
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value)

    # remove annotation
    node = client.reload(node)
    node_annotations = node.annotations.data_dict()
    del node_annotations[annotation_key]
    client.update(node, annotations=node_annotations)


def test_node_annotation_kubectl_edit():
    annotation_key = random_name()
    annotation_value = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])
    node_name = node.nodeName

    # add annotation on node
    command = "annotate nodes " + node_name + " " + \
              annotation_key + "=" + annotation_value
    print(command)
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # annotation should be added
    node = client.reload(node)
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value)

    # edit annotation through kubectl
    new_value = random_name()
    command = "annotate nodes " + node_name + " " + \
              annotation_key + "=" + new_value + " --overwrite"
    print(command)
    execute_kubectl_cmd(command, False)
    node = client.reload(node)
    time.sleep(2)

    # New annotation should be added
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value=new_value)

    # remove annotation
    node = client.reload(node)
    node_annotations = node.annotations.data_dict()
    del node_annotations[annotation_key]
    client.update(node, annotations=node_annotations)


def test_node_annotation_kubectl_delete():
    annotation_key = random_name()
    annotation_value = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])
    node_name = node.nodeName

    # add annotation on node
    command = "annotate nodes " + node_name + " " + \
              annotation_key + "=" + annotation_value
    print(command)
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # annotation should be added
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value)

    # remove annotation through kubectl
    command = "annotate node " + node_name + " " + annotation_key + "-"
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # annotationv should be deleted
    validate_annotation_deleted_on_node(client, node, annotation_key)


def test_node_annotation_k_add_a_delete_k_add():
    """Add via kubectl, Delete via API, Add via kubectl"""
    annotation_key = random_name()
    annotation_value = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])
    node_name = node.nodeName

    command = "annotate nodes " + node_name + " " + \
              annotation_key + "=" + annotation_value
    print(command)
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # annotation should be added
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value)

    # delete annotation
    node = client.reload(node)
    node_annotations = node.annotations.data_dict()
    del node_annotations[annotation_key]
    client.update(node, annotations=node_annotations)
    time.sleep(2)

    # annotation should be deleted
    validate_annotation_deleted_on_node(client, node, annotation_key)

    # Add annotation via kubectl
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # annotation should be added
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value)

    # clean up annotation
    node = client.reload(node)
    node_annotations = node.annotations.data_dict()
    del node_annotations[annotation_key]
    client.update(node, annotations=node_annotations)


def test_node_annotation_k_add_a_edit_k_edit():
    """Add via kubectl, edit via API, edit via kubectl"""
    annotation_key = random_name()
    annotation_value = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])
    node_name = node.nodeName

    command = "annotate nodes " + node_name + " " + \
              annotation_key + "=" + annotation_value
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # annotation should be added
    node = client.reload(node)
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value)

    # edit annotation through API
    node = client.reload(node)
    node_annotations = node.annotations.data_dict()
    new_value = random_name()
    node_annotations[annotation_key] = new_value
    client.update(node, annotations=node_annotations)
    time.sleep(2)

    # annotation should be added
    node = client.reload(node)
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    new_value)

    # edit annotation through kubectl
    new_value_2 = random_name()
    command = "annotate nodes " + node_name + " " + \
              annotation_key + "=" + new_value_2 + " --overwrite"
    print(command)
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # New annotation should be added
    node = client.reload(node)
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    new_value_2)

    # remove annotation
    node = client.reload(node)
    node_annotations = node.annotations.data_dict()
    del node_annotations[annotation_key]
    client.update(node, annotations=node_annotations)


def test_node_annotations_a_add_k_delete_a_add():
    """Add via API, Delete via kubectl, Add via API"""
    annotation_key = random_name()
    annotation_value = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])
    node_name = node.nodeName

    node_annotations = node.annotations.data_dict()
    node_annotations[annotation_key] = annotation_value
    client.update(node, annotations=node_annotations)
    time.sleep(2)

    # annotation should be added
    node = client.reload(node)
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value)

    # delete annotation
    command = " annotate node " + node_name + " " + annotation_key + "-"
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # annotation should be deleted
    node = client.reload(node)
    validate_annotation_deleted_on_node(client, node, annotation_key)

    # Add annotation via API
    node = client.reload(node)
    node_annotations = node.annotations.data_dict()
    node_annotations[annotation_key] = annotation_value
    client.update(node, annotations=node_annotations)
    time.sleep(2)

    # annotation should be added
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value)

    # clean up annotation
    node = client.reload(node)
    node_annotations = node.annotations.data_dict()
    del node_annotations[annotation_key]
    client.update(node, annotations=node_annotations)


def test_node_annotation_a_add_k_edit_a_edit():
    """Add via API, Edit via kubectl, Edit via API"""
    annotation_key = random_name()
    annotation_value = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])
    node_name = node.nodeName

    node_annotations = node.annotations.data_dict()
    node_annotations[annotation_key] = annotation_value
    client.update(node, annotations=node_annotations)
    time.sleep(2)

    # annotation should be added
    node = client.reload(node)
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value)

    # edit annotation through kubectl
    new_value = random_name()
    command = "annotate nodes " + node_name + " " + \
              annotation_key + "=" + new_value + " --overwrite"
    print(command)
    execute_kubectl_cmd(command, False)
    node = client.reload(node)
    time.sleep(2)

    # New annotation should be added
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value=new_value)

    # edit annotation through API
    node = client.reload(node)
    node_annotations = node.annotations.data_dict()
    new_value_2 = random_name()
    node_annotations[annotation_key] = new_value_2
    client.update(node, annotations=node_annotations)
    node = client.reload(node)
    time.sleep(2)

    # annotation should be added
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    new_value_2)

    # clean up annotation
    node = client.reload(node)
    node_annotations = node.annotations.data_dict()
    del node_annotations[annotation_key]
    client.update(node, annotations=node_annotations)


@if_test_rbac
@pytest.mark.parametrize("role", roles)
def test_rbac_node_annotation_add(role):
    annotation_key = random_name()
    annotation_value = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])
    node_annotations = node.annotations.data_dict()

    # get user token and client
    token = rbac_get_user_token_by_role(role)
    print("token: ", token)
    user_client = get_client_for_token(token)

    node_annotations[annotation_key] = annotation_value

    if role in [RESTRICTED_ADMIN, CLUSTER_OWNER]:
        user_client.update(node, annotations=node_annotations)
        node = client.reload(node)
        time.sleep(2)

        # annotation should be added
        validate_annotation_set_on_node(user_client, node,
                                        annotation_key,
                                        annotation_value)
        # cleanup annotation
        delete_node_annotation(annotation_key, node, client)
    else:
        with pytest.raises(ApiError) as e:
            user_client.update(node, annotations=node_annotations)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'


@if_test_rbac
@pytest.mark.parametrize("role", roles)
def test_rbac_node_annotation_delete(role):
    annotation_key = random_name()
    annotation_value = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])

    # add annotation on node
    node_annotations = node.annotations.data_dict()
    node_annotations[annotation_key] = annotation_value
    client.update(node, annotations=node_annotations)
    node = client.reload(node)
    time.sleep(2)

    # annotation should be added
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value)

    # delete annotation
    del node_annotations[annotation_key]

    # get user token and client
    token = rbac_get_user_token_by_role(role)
    print("token: ", token)
    user_client = get_client_for_token(token)

    if role in [RESTRICTED_ADMIN, CLUSTER_OWNER]:
        user_client.update(node, annotations=node_annotations)
        node = client.reload(node)
        time.sleep(2)

        # annotation should be added
        validate_annotation_deleted_on_node(user_client, node, annotation_key)
    else:
        with pytest.raises(ApiError) as e:
            user_client.update(node, annotations=node_annotations)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
    if role not in [RESTRICTED_ADMIN, CLUSTER_OWNER]:
        # cleanup annotation
        delete_node_annotation(annotation_key, node, client)


@if_test_rbac
@pytest.mark.parametrize("role", roles)
def test_rbac_node_annotation_edit(role):
    annotation_key = random_name()
    annotation_value = random_name()
    annotation_value_new = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])

    # add annotation on node
    node_annotations = node.annotations.data_dict()
    node_annotations[annotation_key] = annotation_value
    client.update(node, annotations=node_annotations)
    node = client.reload(node)
    time.sleep(2)

    # annotation should be added
    validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value)

    # edit annotation
    node_annotations[annotation_key] = annotation_value_new

    # get user token and client
    token = rbac_get_user_token_by_role(role)
    print("token: ", token)
    user_client = get_client_for_token(token)

    if role in [RESTRICTED_ADMIN, CLUSTER_OWNER]:
        user_client.update(node, annotations=node_annotations)
        node = client.reload(node)
        time.sleep(2)

        # annotation should be added
        validate_annotation_set_on_node(user_client, node,
                                        annotation_key,
                                        annotation_value_new)
    else:
        with pytest.raises(ApiError) as e:
            user_client.update(node, annotations=node_annotations)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
    delete_node_annotation(annotation_key, node, client)


@if_test_rbac
@pytest.mark.parametrize("role", roles)
def test_rbac_node_annotation_add_kubectl(role):
    annotation_key = random_name()
    annotation_value = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])
    node_name = node.nodeName

    # get user token and client
    token = rbac_get_user_token_by_role(role)
    user_client = get_client_for_token(token)
    print(cluster_detail["cluster"]["id"])
    print(cluster_detail["cluster"])
    cluster = user_client.list_cluster(id=cluster_detail["cluster"]["id"]).data
    print(cluster)
    create_kubeconfig(cluster[0])

    # add annotation on node
    command = "annotate nodes " + node_name + " " + \
              annotation_key + "=" + annotation_value

    if role in [RESTRICTED_ADMIN, CLUSTER_OWNER]:
        execute_kubectl_cmd(command, False)
        node = client.reload(node)
        time.sleep(2)
        # annotation should be added
        validate_annotation_set_on_node(user_client, node,
                                        annotation_key,
                                        annotation_value)
        # cleanup annotation
        delete_node_annotation(annotation_key, node, client)
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


@if_test_rbac
@pytest.mark.parametrize("role", roles)
def test_rbac_node_annotation_delete_kubectl(role):
    annotation_key = random_name()
    annotation_value = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])
    node_name = node.nodeName

    # add annotation on node
    command = "annotate nodes " + node_name + " " + \
              annotation_key + "=" + annotation_value
    print(command)
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # annotation should be added
    node = client.reload(node)

    # get user token and client
    token = rbac_get_user_token_by_role(role)
    user_client = get_client_for_token(token)
    print(cluster_detail["cluster"]["id"])
    print(cluster_detail["cluster"])
    cluster = user_client.list_cluster(id=cluster_detail["cluster"]["id"]).data
    print(cluster)
    create_kubeconfig(cluster[0])

    # remove annotation through kubectl
    command = "annotate node " + node_name + " " + annotation_key + "-"

    if role in [RESTRICTED_ADMIN, CLUSTER_OWNER]:
        execute_kubectl_cmd(command, False)
        time.sleep(2)
        # annotation should be deleted
        validate_annotation_deleted_on_node(user_client, node, annotation_key)
    elif role == CLUSTER_MEMBER:
        result = execute_kubectl_cmd(command, False, stderr=True)
        result = result.decode('ascii')
        assert "cannot patch resource \"nodes\"" in result
        assert "forbidden" in result
        # cleanup annotation
        delete_node_annotation(annotation_key, node, client)
    else:
        result = execute_kubectl_cmd(command, False, stderr=True)
        result = result.decode('ascii')
        assert "cannot get resource \"nodes\"" in result
        assert "forbidden" in result
        # cleanup annotation
        delete_node_annotation(annotation_key, node, client)


@if_test_rbac
@pytest.mark.parametrize("role", roles)
def test_rbac_node_annotation_edit_kubectl(role):
    annotation_key = random_name()
    annotation_value = random_name()
    annotation_value_new = random_name()
    # get node details
    client, node = \
        get_node_details(cluster_detail["cluster"], cluster_detail["client"])
    node_name = node.nodeName

    # add annotation on node
    command = "annotate nodes " + node_name + " " + \
              annotation_key + "=" + annotation_value
    print(command)
    execute_kubectl_cmd(command, False)
    time.sleep(2)

    # annotation should be added
    node = client.reload(node)

    # get user token and client
    token = rbac_get_user_token_by_role(role)
    user_client = get_client_for_token(token)
    print(cluster_detail["cluster"]["id"])
    print(cluster_detail["cluster"])
    cluster = user_client.list_cluster(id=cluster_detail["cluster"]["id"]).data
    print(cluster)
    create_kubeconfig(cluster[0])

    # edit annotation through kubectl
    command = "annotate nodes " + node_name + " " + \
              annotation_key + "=" + annotation_value_new + " --overwrite"

    if role in [RESTRICTED_ADMIN, CLUSTER_OWNER]:
        execute_kubectl_cmd(command, False)
        time.sleep(2)
        # annotation should be deleted
        validate_annotation_set_on_node(user_client, node,
                                        annotation_key,
                                        annotation_value_new)
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
    # cleanup annotation
    delete_node_annotation(annotation_key, node, client)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    cluster_detail["client"], cluster_detail["cluster"] = \
        get_user_client_and_cluster()


def validate_annotation_set_on_node(client, node,
                                    annotation_key,
                                    annotation_value):
    """
    This method checks if the annotation is
    added on the node via API and kubectl
    :param client: user client
    :param node: node on which user has to validate if the annotation is added
    :param annotation_key: annotation to be validated on the node
    :param annotation_value: annotaton value to be checked
    :return: None
    """
    print("annotaton_value: ", annotation_value)
    # check via API
    node = client.reload(node)
    node_annotations = node.annotations.data_dict()
    assert node_annotations[annotation_key] == annotation_value

    # check via kubectl
    node_name = node.nodeName
    command = " get nodes " + node_name
    node_detail = execute_kubectl_cmd(command)
    print(node_detail["metadata"]["annotations"])
    assert annotation_key in node_detail["metadata"]["annotations"], \
        "Annotation is not set in kubectl"
    assert node_detail["metadata"]["annotations"][annotation_key] \
        == annotation_value


def validate_annotation_deleted_on_node(client, node, annotation_key):
    """
    This method checks if the annotation is deleted
    on the node via API and kubectl
    :param client: user client
    :param node: node on which user has to validate if the
    annotation is deleted
    :param annotation_key: annotation to be validated on the node
    :return: None
    """
    # check via API
    node = client.reload(node)
    node_annotations = node.annotations.data_dict()
    assert annotation_key not in node_annotations

    # check via kubectl
    node_name = node.nodeName
    command = " get nodes " + node_name
    print(command)
    node_detail = execute_kubectl_cmd(command)
    print(node_detail["metadata"]["annotations"])
    assert annotation_key not in node_detail["metadata"]["annotations"]


def delete_node_annotation(annotation_key, node, client):
    """

    :param annotation_key: annotation to be deleted on the node
    :param node: node in cluster
    :param client: client
    :return:
    """
    node = client.reload(node)
    node_annotations = node.annotations.data_dict()
    del node_annotations[annotation_key]
    client.update(node, annotations=node_annotations)
