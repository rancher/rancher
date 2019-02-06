from .conftest import wait_until, wait_until_available
from rancher import ApiError
from .common import random_str
import time


def test_project_catalog_creation(admin_mc, remove_resource,
                                  user_mc, user_factory, admin_pc,
                                  admin_cc):
    client = admin_mc.client

    # When project-owner tries to create project catalog, it should succeed
    prtb_owner = client.create_project_role_template_binding(
        projectId=admin_pc.project.id,
        roleTemplateId="project-owner",
        userId=admin_mc.user.id,)
    remove_resource(prtb_owner)

    wait_until(prtb_cb(client, prtb_owner))

    project_owner_client = client
    name = random_str()
    project_name = str.lstrip(admin_pc.project.id, "local:")
    catalog_name = project_name + ":" + name
    url = "https://github.com/mrajashree/charts.git"

    project = admin_pc.project
    project_catalog = \
        project_owner_client.create_project_catalog(name=name,
                                                    branch="onlyOne",
                                                    url=url,
                                                    projectId=project.id,
                                                    )
    wait_for_projectcatalog_template_to_be_created(project_owner_client,
                                                   catalog_name)

    # The project-owner should now be able to access the project level
    # catalog and its templates
    cc = project_owner_client.list_project_catalog(name=name)
    assert len(cc) == 1
    templates = \
        project_owner_client.list_template(projectCatalogId=catalog_name)
    assert len(templates) == 1
    templateversions = \
        project_owner_client.list_template(projectCatalogId=catalog_name)
    assert len(templateversions) == 1

    # Now add a user as project-member to this project
    prtb_member = client.create_project_role_template_binding(
        projectId=project.id,
        roleTemplateId="project-member",
        userId=user_mc.user.id,)
    remove_resource(prtb_member)

    wait_until_available(user_mc.client, admin_cc.cluster)
    wait_until(prtb_cb(client, prtb_member))

    project_member_client = user_mc.client
    # The project-member should now be able to access the project level
    # catalog and its templates
    cc = project_member_client.list_project_catalog()
    assert len(cc) == 1
    templates = \
        project_member_client.list_template(projectCatalogId=catalog_name)
    assert len(templates) == 1
    templateversions = \
        project_member_client.list_template(projectCatalogId=catalog_name)
    assert len(templateversions) == 1

    # But project-member should not be able to create a project catalog
    try:
        project_member_client.create_project_catalog(name=random_str(),
                                                     branch="onlyOne",
                                                     url=url,
                                                     projectId=project.id,
                                                     )
    except ApiError as e:
        assert e.error.status == 403

    # Create another user and don't add to project, this user should not
    # be able to access this cluster catalog or its templates
    user2 = user_factory()
    templates = \
        user2.client.list_template(projectCatalogId=catalog_name)
    assert len(templates) == 0
    cc = user2.client.list_cluster_catalog(name=name)
    assert len(cc) == 0

    client.delete(project_catalog)
    wait_for_projectcatalog_template_to_be_deleted(client, catalog_name)


def test_create_project_catalog_after_user_addition(admin_mc,
                                                    user_factory,
                                                    remove_resource,
                                                    admin_pc):
    # Create a new user
    user1 = user_factory()
    remove_resource(user1)
    client = admin_mc.client
    project = admin_pc.project
    # Add this user as project-member
    prtb_member = client.create_project_role_template_binding(
        projectId=project.id,
        roleTemplateId="project-member",
        userId=user1.user.id)
    remove_resource(prtb_member)

    wait_until(prtb_cb(client, prtb_member))

    # Create project-level catalog for this project as admin
    name = random_str()
    project_name = str.lstrip(admin_pc.project.id, "local:")
    catalog_name = project_name + ":" + name
    url = "https://github.com/mrajashree/charts.git"

    project = admin_pc.project
    project_owner_client = client
    project_catalog = \
        project_owner_client.create_project_catalog(name=name,
                                                    branch="onlyOne",
                                                    url=url,
                                                    projectId=project.id,
                                                    )
    wait_for_projectcatalog_template_to_be_created(project_owner_client,
                                                   catalog_name)

    # The project-owner should now be able to access the project level
    # catalog and its templates
    cc = project_owner_client.list_project_catalog(name=name)
    assert len(cc) == 1
    templates = \
        project_owner_client.list_template(projectCatalogId=catalog_name)
    assert len(templates) == 1
    templateversions = \
        project_owner_client.list_template(projectCatalogId=catalog_name)
    assert len(templateversions) == 1

    project_member_client = user1.client
    # The project-member should now be able to access the project level
    # catalog and its templates
    cc = project_member_client.list_project_catalog()
    assert len(cc) == 1
    templates = \
        project_member_client.list_template(projectCatalogId=catalog_name)
    assert len(templates) == 1

    client.delete(project_catalog)
    wait_for_projectcatalog_template_to_be_deleted(client, catalog_name)


def test_user_addition_after_creating_project_catalog(admin_mc,
                                                      user_factory,
                                                      remove_resource,
                                                      admin_pc):
    # Create project-level catalog for this project as admin
    client = admin_mc.client
    name = random_str()
    project_name = str.lstrip(admin_pc.project.id, "local:")
    catalog_name = project_name + ":" + name
    url = "https://github.com/mrajashree/charts.git"

    project = admin_pc.project
    project_owner_client = client
    project_catalog = \
        project_owner_client.create_project_catalog(name=name,
                                                    branch="onlyOne",
                                                    url=url,
                                                    projectId=project.id,
                                                    )
    wait_for_projectcatalog_template_to_be_created(project_owner_client,
                                                   catalog_name)

    # The project-owner should now be able to access the project level
    # catalog and its templates
    cc = project_owner_client.list_project_catalog(name=name)
    assert len(cc) == 1
    templates = \
        project_owner_client.list_template(projectCatalogId=catalog_name)
    assert len(templates) == 1
    templateverions = \
        project_owner_client.list_template(projectCatalogId=catalog_name)
    assert len(templateverions) == 1

    # Create a new user
    user1 = user_factory()
    remove_resource(user1)
    project = admin_pc.project
    # Add this user as project-member
    prtb_member = client.create_project_role_template_binding(
        projectId=project.id,
        roleTemplateId="project-member",
        userId=user1.user.id)
    remove_resource(prtb_member)

    wait_until(prtb_cb(client, prtb_member))

    project_member_client = user1.client
    # The project-member should now be able to access the project level
    # catalog and its templates
    cc = project_member_client.list_project_catalog()
    assert len(cc) == 1
    templates = \
        project_member_client.list_template(projectCatalogId=catalog_name)
    assert len(templates) == 1
    templateversions = \
        project_member_client.list_template(projectCatalogId=catalog_name)
    assert len(templateversions) == 1

    client.delete(project_catalog)
    wait_for_projectcatalog_template_to_be_deleted(client, catalog_name)


def wait_for_projectcatalog_template_to_be_created(client, name, timeout=45):
    found = False
    start = time.time()
    interval = 0.5
    while not found:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for templates")
        templates = client.list_template(projectCatalogId=name)
        if len(templates) > 0:
            found = True
        time.sleep(interval)
        interval *= 2


def wait_for_projectcatalog_template_to_be_deleted(client, name, timeout=45):
    found = False
    start = time.time()
    interval = 0.5
    while not found:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for templates")
        templates = client.list_template(projectCatalogId=name)
        if len(templates) == 0:
            found = True
        time.sleep(interval)
        interval *= 2


def prtb_cb(client, prtb):
    """Wait for the crtb to have the userId populated"""
    def cb():
        p = client.reload(prtb)
        return p.userPrincipalId is not None
    return cb


def cr_rule_template(api_instance, cr_name, cr, resource):
    def cb():
        c = api_instance.read_cluster_role(cr_name)
        cr_rules = c.rules
        for i in range(0, len(cr_rules)):
            if cr_rules[i].resources[0] == resource:
                return cr_rules[i].resource_names is not None
    return cb
