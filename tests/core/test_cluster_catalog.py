from .conftest import wait_until, wait_until_available
from rancher import ApiError
from .common import wait_to_ensure_user_in_crb_subject, \
    wait_to_ensure_user_removed_from_crb_subject, random_str
import kubernetes
import time


def test_cluster_catalog_creation(admin_mc, remove_resource,
                                  user_factory):
    client = admin_mc.client

    # When cluster-owner tries to create cluster catalog, it should succeed
    crtb_owner = client.create_cluster_role_template_binding(
        clusterId="local",
        roleTemplateId="cluster-owner",
        userId=admin_mc.user.id,)
    remove_resource(crtb_owner)

    wait_until(crtb_cb(client, crtb_owner))

    cluster_owner_client = admin_mc.client
    name = random_str()
    template_name = "local:"+name
    url = "https://github.com/mrajashree/charts.git"
    cluster_catalog = \
        cluster_owner_client.create_cluster_catalog(name=name,
                                                    branch="onlyOne",
                                                    url=url,
                                                    clusterId="local",
                                                    )
    wait_for_clustercatalog_template_to_be_created(cluster_owner_client,
                                                   template_name)

    cc = cluster_owner_client.list_cluster_catalog(name=name)
    assert len(cc) == 1
    templates = \
        cluster_owner_client.list_template(clusterCatalogId=template_name)
    assert len(templates) == 1

    # Create a user and add to the "local" cluster as "cluster-member"
    # cluster-member should be able to list cluster catalog and its templates
    user1 = user_factory()
    remove_resource(user1)
    crtb_member = client.create_cluster_role_template_binding(
        clusterId="local",
        roleTemplateId="cluster-member",
        userId=user1.user.id)
    remove_resource(crtb_member)

    wait_until(crtb_cb(client, crtb_member))
    # wait_until_available(client, crtb_member)

    cluster_member_client = user1.client

    cc = cluster_member_client.list_cluster_catalog(name=name)
    assert len(cc) == 1
    # Both should also be able to list templates of the cluster catalog
    templates = \
        cluster_member_client.list_template(clusterCatalogId=template_name)
    assert len(templates) == 1

    # But cluster-member should not be able to create a cluster catalog
    try:
        cluster_member_client.create_cluster_catalog(name=random_str(),
                                                     branch="onlyOne",
                                                     url=url,
                                                     clusterId="local",
                                                     )
    except ApiError as e:
        assert e.error.status == 403

    # Create another user and don't add to cluster, this user should not
    # be able to access this cluster catalog or its templates
    user2 = user_factory()
    templates = \
        user2.client.list_template(clusterCatalogId=template_name)
    assert len(templates) == 0
    cc = user2.client.list_cluster_catalog(name=name)
    assert len(cc) == 0

    client.delete(cluster_catalog)
    wait_for_clustercatalog_template_to_be_deleted(client, template_name)


def test_cluster_catalog_templates_access(admin_mc, user_factory,
                                          remove_resource, admin_pc):
    # Cluster-owner,cluster-member, and all project-owners/members
    # in that cluster should have access to cluster catalog's templates

    # First add a user as cluster member to this cluster
    user1 = user_factory()
    remove_resource(user1)
    admin_client = admin_mc.client
    crtb_member = admin_client.create_cluster_role_template_binding(
        clusterId="local",
        roleTemplateId="cluster-member",
        userId=user1.user.id)
    remove_resource(crtb_member)

    wait_until(crtb_cb(admin_client, crtb_member))

    # Now create a cluster catalog
    name = random_str()
    catalog_name = "local:" + name
    template_name = "local-" + name + '-etcd-operator'
    template_version_name = template_name + "-0.7.6"
    url = "https://github.com/mrajashree/charts.git"
    cc = admin_client.create_cluster_catalog(name=name,
                                             branch="onlyOne",
                                             url=url,
                                             clusterId="local",
                                             )
    wait_for_clustercatalog_template_to_be_created(admin_client, catalog_name)

    # Find the expected k8s clusterRole for the templates and templateversions
    # of this cluster catalog
    api_instance = kubernetes.client.RbacAuthorizationV1Api(
        admin_mc.k8s_client)

    cr_name = "cluster-local-use-templates-templateversions"
    cr = api_instance.read_cluster_role(cr_name)

    # Ensure that the newly created template name is added to resourenames
    # of the cluster role
    cr_rules = cr.rules
    wait_until(cr_rule_template(api_instance, cr_name, cr, "templates"))
    wait_until(cr_rule_template(api_instance, cr_name, cr, "templateversions"))
    for i in range(0, len(cr_rules)):
        if cr_rules[i].resources[0] == "templates":
            if cr_rules[i].resource_names is not None:
                assert template_name in cr_rules[i].resource_names
        if cr_rules[i].resources[0] == "templateversions":
            if cr_rules[i].resource_names is not None:
                assert template_version_name in cr_rules[i].resource_names

    # Now add a user to a project within this cluster as project-owner
    user2 = user_factory()
    remove_resource(user2)
    prtb_owner = admin_client.create_project_role_template_binding(
        userId=user2.user.id,
        roleTemplateId="project-owner",
        projectId=admin_pc.project.id,
    )
    remove_resource(prtb_owner)
    wait_until(prtb_cb(admin_client, prtb_owner))

    # Get cluster role binding for this cluster role
    # Ensure that cluster-member user1 is added in subjects
    crb_name = "local-templates-templateversions-crb"
    wait_to_ensure_user_in_crb_subject(api_instance, crb_name,
                                       user1.user.id)
    # Ensure that project-owner user2 is added in subjects
    wait_to_ensure_user_in_crb_subject(api_instance, crb_name,
                                       user2.user.id)

    wait_until_available(admin_client, prtb_owner)
    project_owner_client = user2.client

    templates = \
        project_owner_client.list_template(clusterCatalogId=catalog_name)
    assert len(templates) == 1

    cluster_member_client = user1.client
    templates = \
        cluster_member_client.list_template(clusterCatalogId=catalog_name)
    assert len(templates) == 1

    # Now remove user1 also from the cluster, this should mean user1 should
    # no longer be able to access the catalog and templates
    admin_client.delete(crtb_member)
    wait_to_ensure_user_removed_from_crb_subject(api_instance, crb_name,
                                                 user1.user.id)
    templates = \
        user1.client.list_template(clusterCatalogId=catalog_name)
    assert len(templates) == 0

    # Now remove the user admin_pc from the project of this cluster,
    # so admin_pc should no longer have access to catalog and templates
    admin_client.delete(prtb_owner)
    wait_to_ensure_user_removed_from_crb_subject(api_instance, crb_name,
                                                 user2.user.id)
    templates = \
        user2.client.list_template(clusterCatalogId=catalog_name)
    assert len(templates) == 0

    admin_client.delete(cc)
    wait_for_clustercatalog_template_to_be_deleted(admin_client, catalog_name)


def wait_for_clustercatalog_template_to_be_created(client, name, timeout=45):
    found = False
    start = time.time()
    interval = 0.5
    while not found:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for templates")
        templates = client.list_template(clusterCatalogId=name)
        if len(templates) > 0:
            found = True
        time.sleep(interval)
        interval *= 2


def wait_for_clustercatalog_template_to_be_deleted(client, name, timeout=45):
    found = False
    start = time.time()
    interval = 0.5
    while not found:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for templates")
        templates = client.list_template(clusterCatalogId=name)
        if len(templates) == 0:
            found = True
        time.sleep(interval)
        interval *= 2


def cr_rule_template(api_instance, cr_name, cr, resource):
    def cb():
        c = api_instance.read_cluster_role(cr_name)
        cr_rules = c.rules
        for i in range(0, len(cr_rules)):
            if cr_rules[i].resources[0] == resource:
                return cr_rules[i].resource_names is not None
    return cb


def crtb_cb(client, crtb):
    """Wait for the crtb to have the userId populated"""
    def cb():
        c = client.reload(crtb)
        return c.userPrincipalId is not None
    return cb


def prtb_cb(client, prtb):
    """Wait for the crtb to have the userId populated"""
    def cb():
        p = client.reload(prtb)
        return p.userPrincipalId is not None
    return cb
