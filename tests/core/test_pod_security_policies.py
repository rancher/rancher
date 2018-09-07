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


def setup_cluster_with_pspt(client, request):
    pspt = client.create_pod_security_policy_template("pspt1")
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

    k8s_client = kubernetes_api_client(admin_mc.client, admin_mc.cluster_id)
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
