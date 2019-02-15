import kubernetes

from .conftest import kubernetes_api_client, wait_for


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


def setup_cluster_with_pspt(client, request, template="pspt1"):
    """
    WARNING: This function is not safe for parallel exec, only used by
    test_service_accounts!!
    Sets the 'local' cluster to have the pspt given by template, if it exists,
    returns the pspt and cleans up. If it does, creates a copy and applies it.
    If it does not, it creates a generic one. Also adds a finalizer for cleanup
    """
    pspt = client.by_id_pod_security_policy_template(template)

    if pspt is None:
        # See: v3/podsecuritypolicytemplates
        pspt = client.create_pod_security_policy_template(template)

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
        "asdf", "default", service_account))
    request.addfinalizer(
        lambda: rbac.delete_namespaced_role_binding(
            "default-asdf-default-" + pspt.id + "-clusterrole-binding",
            "default", kubernetes.client.V1DeleteOptions()))

    wait_for(lambda: service_account_has_role_binding(rbac, pspt), timeout=30)


def test_pod_security_policy_template_del(admin_mc, request, remove_resource):
    """ Test for pod security policy template binding correctly
    ref https://github.com/rancher/rancher/issues/15728
    ref https://localhost:8443/v3/podsecuritypolicytemplates
    """
    api_client = admin_mc.client
    # these create a mock pspts... not valid for real psp's

    pspt_proj = api_client.create_pod_security_policy_template("pspt2")
    # add a finalizer to delete the pspt
    remove_resource(pspt_proj)

    #  create a project in order to establish bindings
    def create_project():
        p = api_client.create_project(name="test-unrestrict-proj",
                                      clusterId="local")
        p = api_client.wait_success(p)
        # In case something goes badly, add a finalizer to
        # delete the project as the admin
        remove_resource(p)

        p.setpodsecuritypolicytemplate(
            podSecurityPolicyTemplateId=pspt_proj.id)

        return p

    proj = create_project()
    # wait to use proj object until done transitioning
    proj = api_client.wait_success(proj)
    # Check that project was created successfully with pspt
    assert proj.state == 'active'
    assert proj.podSecurityPolicyTemplateId == pspt_proj.id

    # allow for binding deletion
    api_client.delete(proj)

    def check_project(client, proj):
        return client.by_id_project(proj.id) is None

    wait_for(lambda: check_project(api_client, proj))
    # delete the PSPT that was associated with the deleted project
    api_client.delete(pspt_proj)

    def pspt_del_check(client, p):
        if client.by_id_pod_security_policy_template(p.id) is None:
            return True
        else:  # keep checking to see delete occurred
            return False

    # will timeout if pspt is not deleted
    # this validates bug
    wait_for(lambda: pspt_del_check(api_client, pspt_proj))

    assert api_client.by_id_pod_security_policy_template(pspt_proj.id) is None
