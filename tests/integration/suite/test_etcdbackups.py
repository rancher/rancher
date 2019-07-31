from .conftest import wait_until
import kubernetes


role_template = "backups-manage"


def test_access_to_etcd_backups(admin_mc, user_factory, remove_resource):
    client = admin_mc.client
    restricted_user = user_factory(globalRoleId='user-base')

    # add user to local cluster with "Manage cluster backups" role
    crtb_rstrd = client.create_cluster_role_template_binding(
        clusterId="local",
        roleTemplateId=role_template,
        userId=restricted_user.user.id, )
    remove_resource(crtb_rstrd)
    wait_until(crtb_cb(client, crtb_rstrd))

    # check that role "backups-manage" was created in the cluster
    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)
    role = rbac.read_namespaced_role(role_template, "local")
    assert role is not None
    assert "etcdbackups" in role.rules[0].resources


def crtb_cb(client, crtb):
    """Wait for the crtb to have the userId populated"""
    def cb():
        c = client.reload(crtb)
        return c.userPrincipalId is not None
    return cb
