from .conftest import *  # NOQA
from .common import *  # NOQA


@pytest.mark.skip("Use as an example of how to test RBAC")
def test_install_rbac_1(test_name, cloud_provider, rke_client, kubectl):
    """
    Create a three node cluster and runs validation to create pods
    Removes cluster and validates components are removed
    """
    rke_template = 'cluster_install_rbac_1.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(3, test_name)
    create_rke_cluster(rke_client, kubectl, nodes, rke_template)

    result = kubectl.create_resourse_from_yml(
        'resources/k8s_ymls/daemonset_pods_per_node.yml', namespace='default')
    assert result.ok, result.stderr
    kubectl.create_ns('outside-role')
    result = kubectl.create_resourse_from_yml(
        'resources/k8s_ymls/daemonset_pods_per_node.yml',
        namespace='outside-role')
    assert result.ok, result.stderr

    # Create role and rolebinding to user1 in namespace 'default'
    # namespace is coded in role.yml and rolebinding.yml
    result = kubectl.create_resourse_from_yml('resources/k8s_ymls/role.yml')
    assert result.ok, result.stderr
    result = kubectl.create_resourse_from_yml(
        'resources/k8s_ymls/rolebinding.yml')
    assert result.ok, result.stderr

    # verify read in namespace
    admin_call_pods = kubectl.get_resource('pods', namespace='default')
    user_call_pods = kubectl.get_resource(
        'pods', as_user='user1', namespace='default')

    # Make sure the number of pods returned with out user is the same as user
    # for this namespace
    assert len(admin_call_pods['items']) > 0, "Pods should be greater than 0"
    assert (len(admin_call_pods['items']) == len(user_call_pods['items'])), (
        "Did not retrieve correct number of pods for 'user1'. Expected {0},"
        "Retrieved {1}".format(
            len(admin_call_pods['items']), len(user_call_pods['items'])))

    # verify restrictions no pods return in get pods in different namespaces
    user_call_pods = kubectl.get_resource(
        'pods', as_user='user1', namespace='outside-role')
    assert len(user_call_pods['items']) == 0, (
        "Should not be able to get pods outside of defined user1 namespace")

    # verify create fails as user for any namespace
    result = kubectl.run(test_name + '-pod2', image='nginx', as_user='user1',
                         namespace='outside-role')
    assert result.ok is False, (
        "'user1' should not be able to create pods in other namespaces:\n{0}"
        .format(result.stdout + result.stderr))
    assert "cannot create" in result.stdout + result.stderr

    result = kubectl.run(test_name + '-pod3', image='nginx', as_user='user1',
                         namespace='default')
    assert result.ok is False, (
        "'user1' should not be able to create pods in its own namespace:\n{0}"
        .format(result.stdout + result.stderr))
    assert "cannot create" in result.stdout + result.stderr

    for node in nodes:
        cloud_provider.delete_node(node)
