import subprocess
import pytest
from .conftest import create_kubeconfig, wait_for
from sys import platform
from kubernetes.client import CustomObjectsApi
from rancher import ApiError


# test if the kubeconfig works to list api-resources for the fqdn context
def exec_kubectl(request, dind_cc, client, cmd='api-resources'):
    cluster_kubeconfig_file = create_kubeconfig(request, dind_cc, client)
    # verify cluster scoped access
    try:
        return subprocess.check_output(
            'kubectl ' + cmd +
            ' --kubeconfig ' + cluster_kubeconfig_file +
            ' --context ' + dind_cc.name + '-fqdn',
            stderr=subprocess.STDOUT, shell=True,
        )
    except subprocess.CalledProcessError as err:
        print('kubectl error: ' + str(err.output))
        raise err


# test generator for multiple attempts
def kubectl_available(request, dind_cc, client):
    def test():
        try:
            exec_kubectl(request, dind_cc, client)
            return True
        except subprocess.CalledProcessError:
            return False
    return test


# as an admin, we should have access
@pytest.mark.skip(reason='cluster testing needs refactor')
@pytest.mark.skipif(platform != 'linux', reason='requires linux for dind')
@pytest.mark.nonparallel
def test_admin_api_resources(request, dind_cc):
    wait_for(kubectl_available(request, dind_cc, dind_cc.admin_mc.client))


# as a user which has not been given permission, we should fail
@pytest.mark.skip(reason='cluster testing needs refactor')
@pytest.mark.skipif(platform != 'linux', reason='requires linux for dind')
@pytest.mark.nonparallel
def test_user_no_template(request, dind_cc, user_mc):
    test_admin_api_resources(request, dind_cc)
    with pytest.raises(ApiError) as e:
        exec_kubectl(request, dind_cc, user_mc.client)
    assert e.value.error.status == 403, 'user should not have permission'


# as a user that is a cluster member, we should have access
@pytest.mark.skip(reason='cluster testing needs refactor')
@pytest.mark.skipif(platform != 'linux', reason='requires linux for dind')
@pytest.mark.nonparallel
def test_user_with_template(request, dind_cc, user_mc):
    test_user_no_template(request, dind_cc, user_mc)
    role_template = {
        'clusterId': dind_cc.cluster.id,
        'userPrincipalId': 'local://' + user_mc.user.id,
        'roleTemplateId': 'cluster-member'
    }
    dind_cc.admin_mc.client.create_clusterRoleTemplateBinding(role_template)
    wait_for(kubectl_available(request, dind_cc, user_mc.client))


# as a user that is part of a group that has access, we should have access
@pytest.mark.skip(reason='cluster testing needs refactor')
@pytest.mark.skipif(platform != 'linux', reason='requires linux for dind')
@pytest.mark.nonparallel
def test_user_group_with_template(request, dind_cc, user_mc):
    test_user_no_template(request, dind_cc, user_mc)
    crdClient = CustomObjectsApi(dind_cc.admin_mc.k8s_client)
    user_attribute = crdClient.get_cluster_custom_object(
        'management.cattle.io',
        'v3',
        'userattributes',
        user_mc.user.id
    )
    user_attribute['GroupPrincipals']['local']['Items'] = [{
        'metadata': {
            'name': 'local_group://test-123'
        }
    }]
    crdClient.replace_cluster_custom_object(
        'management.cattle.io',
        'v3',
        'userattributes',
        user_mc.user.id,
        user_attribute
    )
    role_template = {
        'clusterId': dind_cc.cluster.id,
        'groupPrincipalId': 'local_group://test-123',
        'roleTemplateId': 'cluster-member'
    }
    dind_cc.admin_mc.client.create_clusterRoleTemplateBinding(role_template)
    wait_for(kubectl_available(request, dind_cc, user_mc.client))
