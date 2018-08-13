import kubernetes
from rancher import ApiError
from .common import random_str
from .conftest import wait_until_available,\
    cluster_and_client, kubernetes_api_client, wait_for


def test_multi_user(admin_mc, user_mc):
    """Tests a bug in the python client where multiple clients would not
    work properly. All clients would get the auth header of the last  client"""
    # Original admin client should be able to get auth configs
    ac = admin_mc.client.list_auth_config()
    assert len(ac) > 0

    # User client should not. We currently dont 404 on this, which would be
    # more correct. Instead, list gets filtered to zero
    ac = user_mc.client.list_auth_config()
    assert len(ac) == 0


def test_project_owner(admin_cc, admin_mc, user_mc, request):
    """Tests that a non-admin member can create a project, create and
    add a namespace to it, and can do workload related things in the namespace.

    This is the first test written incorporating a non-admin user and the
    kubernetes python client. It does a lot partially as an experiment and
    partially as an example for other yet-to-be-written tests
    """
    admin_client = admin_mc.client
    admin_client.create_cluster_role_template_binding(
        userId=user_mc.user.id,
        roleTemplateId="cluster-member",
        clusterId=admin_cc.cluster.id,
    )

    user_client = user_mc.client

    # When this returns, the user can successfully access the cluster and thus
    # can create a project in it. We generally need this wait_until_available
    # call when we are creating cluster, project, and namespaces as non-admins
    # because until the rbac controllers have had a chance to run and the
    # creator is bootstrapped into the resource, they will not be able to
    # access it
    wait_until_available(user_client, admin_cc.cluster)

    p = user_client.create_project(name='test-' + random_str(),
                                   clusterId=admin_cc.cluster.id)

    # In case something goes badly as the user, add a finalizer to
    # delete the project as the admin
    request.addfinalizer(lambda: admin_client.delete(p))

    # When this returns, the user can successfully access the project and thus
    # can create a namespace in it
    p = wait_until_available(user_client, p)
    p = user_client.wait_success(p)
    assert p.state == 'active'

    k8s_client = kubernetes_api_client(user_client, 'local')
    auth = kubernetes.client.AuthorizationV1Api(k8s_client)

    # Rancher API doesn't have a surefire way of knowing exactly when the user
    # has the ability to create namespaces yet. So we have to rely on an actual
    # kubernetes auth check.
    def can_create_ns():
        access_review = kubernetes.client.V1SelfSubjectAccessReview(spec={
            "resourceAttributes": {
                'verb': 'create',
                'resource': 'namespaces',
                'group': '',
            },
        })
        response = auth.create_self_subject_access_review(access_review)
        return response.status.allowed

    wait_for(can_create_ns)

    cluster, c_client = cluster_and_client('local', user_mc.client)
    ns = c_client.create_namespace(name='test-' + random_str(),
                                   projectId=p.id)
    ns = wait_until_available(c_client, ns)
    ns = c_client.wait_success(ns)
    assert ns.state == 'active'

    # Simple proof that user can get pods in the created namespace.
    # We just care that the list call does not error out
    core = kubernetes.client.CoreV1Api(api_client=k8s_client)
    core.list_namespaced_pod(ns.name)

    # As the user, assert that the two expected role bindings exist in the
    # namespace for the user. There should be one for the rancher role
    # 'project-owner' and one for the k8s built-in role 'admin'
    rbac = kubernetes.client.RbacAuthorizationV1Api(api_client=k8s_client)
    rbs = rbac.list_namespaced_role_binding(ns.name)
    rb_dict = {}
    assert len(rbs.items) == 2
    for rb in rbs.items:
        if rb.subjects[0].name == user_mc.user.id:
            rb_dict[rb.role_ref.name] = rb
    assert 'project-owner' in rb_dict
    assert 'admin' in rb_dict

    # As an additional measure of proof and partially just as an exercise in
    # using this particular k8s api, check that the user can create
    # deployments using the subject access review api
    access_review = kubernetes.client.V1LocalSubjectAccessReview(spec={
        "resourceAttributes": {
            'namespace': ns.name,
            'verb': 'create',
            'resource': 'deployments',
            'group': 'extensions',
        },
    })
    response = auth.create_self_subject_access_review(access_review)
    assert response.status.allowed is True


def test_removing_user_from_cluster(admin_pc, admin_mc, user_mc, admin_cc,
                                    remove_resource):
    """Test that a user added to a project in a cluster is able to see that
    cluster and after being removed from the project they are no longer able
    to see the cluster.
    """

    # Yes, this is misspelled, it's how the actual label is spelled.
    mbo = 'memberhsip-binding-owner'

    admin_client = admin_mc.client
    prtb = admin_client.create_project_role_template_binding(
        userId=user_mc.user.id,
        roleTemplateId="project-member",
        projectId=admin_pc.project.id,
    )
    remove_resource(prtb)

    # Verify the user can see the cluster
    wait_until_available(user_mc.client, admin_cc.cluster)

    api_instance = kubernetes.client.RbacAuthorizationV1Api(
        admin_mc.k8s_client)

    # Find the expected k8s clusterRoleBinding
    crbs = api_instance.list_cluster_role_binding(
        label_selector=prtb.uuid+"="+mbo)

    assert len(crbs.items) == 1

    # Delete the projectRoleTemplateBinding, this should cause the user to no
    # longer be able to see the cluster
    admin_mc.client.delete(prtb)

    def crb_callback():
        crbs = api_instance.list_cluster_role_binding(
            label_selector=prtb.uuid+"="+mbo)
        return len(crbs.items) == 0

    def fail_handler():
        return "failed waiting for cluster role binding to be deleted"

    wait_for(crb_callback, fail_handler=fail_handler)

    try:
        cluster = user_mc.client.by_id_cluster(admin_cc.cluster.id)
        assert cluster is None
    except ApiError as e:
        assert e.error.status == 403
