import kubernetes
from .conftest import kubernetes_api_client, wait_for
from .common import random_str
from rancher import ApiError
import pytest
from kubernetes.client import CustomObjectsApi


def cleanup_pspt(client, request, cluster):
    def remove_pspt_from_cluster_and_delete(cluster):
        pspt_id = cluster.defaultPodSecurityPolicyTemplateId
        pspt = client.by_id_pod_security_policy_template(pspt_id)
        cluster.defaultPodSecurityPolicyTemplateId = ""
        client.update_by_id_cluster(cluster.id, cluster)
        client.delete(pspt)

    request.addfinalizer(
        lambda: remove_pspt_from_cluster_and_delete(cluster)
    )


def create_pspt(client):
    """ Creates a minimally valid pspt with cleanup left to caller"""
    runas = {"rule": "RunAsAny"}
    selinx = {"rule": "RunAsAny"}
    supgrp = {"ranges": [{"max": 65535, "min": 1}],
              "rule": "MustRunAs"
              }
    fsgrp = {"ranges": [{"max": 65535, "min": 1, }],
             "rule": "MustRunAs",
             }
    pspt = \
        client.create_pod_security_policy_template(name="test" + random_str(),
                                                   description="Test PSPT",
                                                   privileged=False,
                                                   seLinux=selinx,
                                                   supplementalGroups=supgrp,
                                                   runAsUser=runas,
                                                   fsGroup=fsgrp,
                                                   volumes='*'
                                                   )
    return pspt


def setup_cluster_with_pspt(client, request):
    """
       Sets the 'local' cluster to mock a PSP by applying a minimally valid
       restricted type PSPT
    """
    pspt = create_pspt(client)
    pspt_id = pspt.id

    # this won't enforce pod security policies on the local cluster but it
    # will let us test that the role bindings are being created correctly
    cluster = client.by_id_cluster("local")
    setattr(cluster, "defaultPodSecurityPolicyTemplateId", pspt_id)
    client.update_by_id_cluster("local", cluster)
    cleanup_pspt(client, request, cluster)

    return pspt


def service_account_has_role_binding(rbac, pspt):
    try:
        rbac.read_namespaced_role_binding("default-asdf-default-" + pspt.id +
                                          "-clusterrole-binding", "default")
        return True
    except kubernetes.client.rest.ApiException:
        return False


def test_service_accounts_have_role_binding(admin_mc, request):
    api_client = admin_mc.client
    pspt = setup_cluster_with_pspt(api_client, request)

    k8s_client = kubernetes_api_client(admin_mc.client, 'local')
    core = kubernetes.client.CoreV1Api(api_client=k8s_client)
    rbac = kubernetes.client.RbacAuthorizationV1Api(api_client=k8s_client)

    service_account = kubernetes.client.V1ServiceAccount()
    service_account.metadata = kubernetes.client.V1ObjectMeta()
    service_account.metadata.name = "asdf"

    core.create_namespaced_service_account("default", service_account)
    request.addfinalizer(lambda: core.delete_namespaced_service_account(
        "asdf", "default"))
    request.addfinalizer(
        lambda: rbac.delete_namespaced_role_binding(
            "default-asdf-default-" + pspt.id + "-clusterrole-binding",
            "default"))

    wait_for(lambda: service_account_has_role_binding(rbac, pspt), timeout=30)


def test_pod_security_policy_template_del(admin_mc, admin_pc, remove_resource):
    """ Test for pod security policy template binding correctly
    ref https://github.com/rancher/rancher/issues/15728
    ref https://localhost:8443/v3/podsecuritypolicytemplates
    """
    api_client = admin_mc.client
    k8s_dynamic_client = CustomObjectsApi(admin_mc.k8s_client)
    # these create a mock pspts... not valid for real psp's

    pspt_proj = create_pspt(api_client)
    # add a finalizer to delete the pspt
    remove_resource(pspt_proj)

    #  creates a project and handles cleanup
    proj = admin_pc.project
    # this will retry 3 times if there is an ApiError

    def set_psp_enabled(value):
        local_cluster = k8s_dynamic_client.get_cluster_custom_object(
            "management.cattle.io", "v3", "clusters", "local")
        local_cluster["metadata"]["annotations"]["capabilities/pspEnabled"] \
            = value
        k8s_dynamic_client.replace_cluster_custom_object(
            "management.cattle.io", "v3", "clusters", "local", local_cluster)

    set_psp_enabled("false")
    wait_for(lambda: not api_client.by_id_cluster(id="local").capabilities.
             pspEnabled)

    with pytest.raises(ApiError) as e:
        api_client.action(obj=proj,
                          action_name="setpodsecuritypolicytemplate",
                          podSecurityPolicyTemplateId=pspt_proj.id)
    assert e.value.error.status == 422
    assert "cluster [local] does not have Pod Security Policies enabled" in \
           e.value.error.message

    set_psp_enabled("true")
    wait_for(lambda: api_client.by_id_cluster(id="local").capabilities.
             pspEnabled)

    api_client.action(obj=proj, action_name="setpodsecuritypolicytemplate",
                      podSecurityPolicyTemplateId=pspt_proj.id)
    proj = api_client.wait_success(proj)

    # Check that project was created successfully with pspt
    assert proj.state == 'active'
    assert proj.podSecurityPolicyTemplateId == pspt_proj.id

    def check_psptpb():
        proj_obj = proj.podSecurityPolicyTemplateProjectBindings()
        for data in proj_obj.data:
            if (data.targetProjectId == proj.id and
                    data.podSecurityPolicyTemplateId == pspt_proj.id):
                return True

        return False

    wait_for(check_psptpb, lambda: "PSPTB project binding not found")
    # allow for binding deletion
    api_client.delete(proj)

    def check_project():
        return api_client.by_id_project(proj.id) is None

    wait_for(check_project)
    # delete the PSPT that was associated with the deleted project
    api_client.delete(pspt_proj)

    def pspt_del_check():
        if api_client.by_id_pod_security_policy_template(pspt_proj.id) is None:
            return True
        else:  # keep checking to see delete occurred
            return False

    # will timeout if pspt is not deleted
    wait_for(pspt_del_check)
    assert api_client.by_id_pod_security_policy_template(pspt_proj.id) is None

    set_psp_enabled("false")
    wait_for(lambda: not api_client.by_id_cluster(id="local").capabilities.
             pspEnabled)


def test_incorrect_pspt(admin_mc, remove_resource):
    """ Test that incorrect pod security policy templates cannot be created"""
    api_client = admin_mc.client

    name = "pspt" + random_str()
    with pytest.raises(ApiError) as e:
        api_client.create_podSecurityPolicyTemplate(name=name)
    assert e.value.error.status == 422

    name = "pspt" + random_str()
    with pytest.raises(ApiError) as e:
        args = {'name': name,
                'description': 'Test PSPT',
                'fsGroup': {"rule": "RunAsAny"},
                'runAsUser': {"rule": "RunAsAny"},
                'seLinux': {"rule": "RunAsAny"},
                'supplementalGroups': {"rule": "RunAsAny"},
                'allowPrivilegeEscalation': False,
                'defaultAllowPrivilegeEscalation': True}
        # Should not set the default True if allowPrivilegedEscalation is false
        api_client.create_podSecurityPolicyTemplate(**args)
    assert e.value.error.status == 422
    assert e.value.error.code == 'InvalidBodyContent'


def test_pspt_binding(admin_mc, admin_pc, remove_resource):
    """Test that a PSPT binding is validated before creating it"""
    api_client = admin_mc.client

    # No podSecurityPolicyTemplateId causes a 422
    name = random_str()
    with pytest.raises(ApiError) as e:
        b = api_client.create_podSecurityPolicyTemplateProjectBinding(
            name=name,
            namespaceId='default',
            podSecurityPolicyTemplateId=None,
            targetProjectId=admin_pc.project.id,
        )
        remove_resource(b)
    assert e.value.error.status == 422
    assert e.value.error.message == \
        'missing required podSecurityPolicyTemplateId'

    # An invalid podSecurityPolicyTemplateId causes a 422
    name = random_str()
    with pytest.raises(ApiError) as e:
        b = api_client.create_podSecurityPolicyTemplateProjectBinding(
            name=name,
            namespaceId='default',
            podSecurityPolicyTemplateId='thisdoesntexist',
            targetProjectId=admin_pc.project.id,
        )
        remove_resource(b)
    assert e.value.error.status == 422
    assert e.value.error.message == 'podSecurityPolicyTemplate not found'
