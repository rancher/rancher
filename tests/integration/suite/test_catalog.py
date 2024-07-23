import pytest
import time
from rancher import ApiError
from .common import wait_for_template_to_be_created, \
    wait_for_template_to_be_deleted, random_str, wait_for_atleast_workload
from .conftest import set_server_version, wait_for, DEFAULT_CATALOG


def test_catalog(admin_mc, remove_resource):
    client = admin_mc.client
    name1 = random_str()
    name2 = random_str()
    url1 = "https://github.com/StrongMonkey/charts-1.git"
    url2 = "HTTP://github.com/StrongMonkey/charts-1.git"
    catalog1 = client.create_catalog(name=name1,
                                     branch="test",
                                     url=url1,
                                     )
    remove_resource(catalog1)
    catalog2 = client.create_catalog(name=name2,
                                     branch="test",
                                     url=url2,
                                     )
    remove_resource(catalog2)
    wait_for_template_to_be_created(client, name1)
    wait_for_template_to_be_created(client, name2)
    client.delete(catalog1)
    client.delete(catalog2)
    wait_for_template_to_be_deleted(client, name1)
    wait_for_template_to_be_deleted(client, name2)


def test_invalid_catalog_chars(admin_mc, remove_resource):
    client = admin_mc.client
    name = random_str()
    url = "https://github.com/%0dStrongMonkey%0A/charts-1.git"
    with pytest.raises(ApiError) as e:
        catalog = client.create_catalog(name=name,
                                        branch="test",
                                        url=url,
                                        )
        remove_resource(catalog)
    assert e.value.error.status == 422
    assert e.value.error.message == "Invalid characters in catalog URL"
    url = "https://github.com/StrongMonkey\t/charts-1.git"
    with pytest.raises(ApiError) as e:
        catalog = client.create_catalog(name=name,
                                        branch="test",
                                        url=url,
                                        )
        remove_resource(catalog)
    assert e.value.error.status == 422
    assert e.value.error.message == "Invalid characters in catalog URL"


def test_global_catalog_template_access(admin_mc, user_factory,
                                        remove_resource):
    client = admin_mc.client
    user1 = user_factory()
    remove_resource(user1)
    name = random_str()

    # Get all templates from library catalog that is enabled by default
    updated = False
    start = time.time()
    interval = 0.5
    while not updated:
        time.sleep(interval)
        interval *= 2
        c = client.list_catalog(name="library").data[0]
        if c.transitioning == "no":
            updated = True
            continue
        if time.time() - start > 90:
            raise AssertionError(
                "Timed out waiting for catalog to stop transitioning")

    existing = client.list_template(catalogId="library").data
    templates = []
    for t in existing:
        templates.append("library-" + t.name)

    url = "https://github.com/mrajashree/charts.git"
    catalog = client.create_catalog(name=name,
                                    branch="onlyOne",
                                    url=url,
                                    )
    wait_for_template_to_be_created(client, name)
    updated = False
    start = time.time()
    interval = 0.5
    while not updated:
        time.sleep(interval)
        interval *= 2
        c = client.list_catalog(name=name).data[0]
        if c.transitioning == "no":
            updated = True
            continue
        if time.time() - start > 90:
            raise AssertionError(
                "Timed out waiting for catalog to stop transitioning")

    # Now list all templates of this catalog
    new_templates = client.list_template(catalogId=name).data
    for t in new_templates:
        templates.append(name + "-" + t.name)

    all_templates = existing + new_templates
    # User should be able to list all these templates
    user_client = user1.client
    user_lib_templates = user_client.list_template(catalogId="library").data
    user_new_templates = user_client.list_template(catalogId=name).data
    user_templates = user_lib_templates + user_new_templates
    assert len(user_templates) == len(all_templates)

    client.delete(catalog)
    wait_for_template_to_be_deleted(client, name)


def test_user_can_list_global_catalog(user_factory, remove_resource):
    user1 = user_factory()
    remove_resource(user1)
    user_client = user1.client
    c = user_client.list_catalog(name="library")
    assert len(c) == 1


@pytest.mark.nonparallel
@pytest.mark.skip
def test_template_version_links(admin_mc, admin_pc, custom_catalog,
                                remove_resource, restore_rancher_version):
    """Test that template versionLinks are being updated based off the rancher
    version set on the server and the query paramater 'rancherVersion' being
    set.
    """
    # 1.6.0 uses 2.0.0-2.2.0
    # 1.6.2 uses 2.1.0-2.3.0
    client = admin_mc.client

    c_name = random_str()
    custom_catalog(name=c_name)

    # Set the server expecting both versions
    set_server_version(client, "2.1.0")

    templates = client.list_template(
        rancherVersion='2.1.0', catalogId=c_name)

    assert len(templates.data[0]['versionLinks']) == 2
    assert '1.6.0' in templates.data[0]['versionLinks']
    assert '1.6.2' in templates.data[0]['versionLinks']

    # Set the server expecting only the older version
    set_server_version(client, "2.0.0")

    templates = client.list_template(
        rancherVersion='2.0.0', catalogId=c_name)

    assert len(templates.data[0]['versionLinks']) == 1
    assert '1.6.0' in templates.data[0]['versionLinks']

    # Set the server expecting only the newer version
    set_server_version(client, "2.3.0")

    templates = client.list_template(
        rancherVersion='2.3.0', catalogId=c_name)

    assert len(templates.data[0]['versionLinks']) == 1
    assert '1.6.2' in templates.data[0]['versionLinks']

    # Set the server expecting no versions, this should be outside both
    # versions acceptable ranges
    set_server_version(client, "2.4.0")

    templates = client.list_template(
        rancherVersion='2.4.0', catalogId=c_name)

    assert len(templates.data[0]['versionLinks']) == 0


def test_relative_paths(admin_mc, admin_pc, remove_resource):
    """ This test adds a catalog's index.yaml with a relative chart url
    and ensures that rancher can resolve the relative url"""

    client = admin_mc.client
    catalogname = "cat-" + random_str()
    url = "https://raw.githubusercontent.com/rancher/integration-test-charts"\
        "/relative-path"
    catalog = client.create_catalog(catalogName=catalogname, branch="master",
                                    url=url)
    remove_resource(catalog)

    catalog = client.reload(catalog)
    assert catalog['url'] == url

    # now deploy the app in the catalog to ensure we can resolve the tarball
    ns = admin_pc.cluster.client.create_namespace(
        catalogName="ns-" + random_str(),
        projectId=admin_pc.project.id)
    remove_resource(ns)

    wait_for_template_to_be_created(client, catalog.id)
    mysqlha = admin_pc.client.create_app(name="app-" + random_str(),
                                         externalId="catalog://?catalog=" +
                                                    catalog.id +
                                                    "&template=mysql"
                                                    "&version=1.6.2",
                                         targetNamespace=ns.name,
                                         projectId=admin_pc.project.id)
    remove_resource(mysqlha)
    wait_for_atleast_workload(pclient=admin_pc.client, nsid=ns.id, timeout=60,
                              count=1)


def test_cannot_delete_system_catalog(admin_mc):
    """This test asserts that the system catalog cannot be delete"""
    client = admin_mc.client

    system_catalog = client.by_id_catalog("system-library")

    with pytest.raises(ApiError) as e:
        client.delete(system_catalog)

    assert e.value.error.status == 422
    assert e.value.error.message == 'not allowed to delete system-library' \
                                    ' catalog'


def test_system_catalog_missing_remove_link(admin_mc):
    """This test asserts that the remove link is missing from system-catalog's
    links"""
    client = admin_mc.client

    system_catalog = client.by_id_catalog("system-library")

    assert "remove" not in system_catalog.links


def test_cannot_update_system_if_embedded(admin_mc):
    """This test asserts that the system catalog cannot be updated if
    system-catalog setting is set to 'bundled'"""
    client = admin_mc.client

    system_catalog_setting = client.by_id_setting("system-catalog")
    # this could potentially interfere with other tests if they were to rely
    # on system-catalog setting
    client.update_by_id_setting(id=system_catalog_setting.id, value="bundled")

    system_catalog = client.by_id_catalog("system-library")

    with pytest.raises(ApiError) as e:
        client.update_by_id_catalog(id=system_catalog.id, branch="asd")

    assert e.value.error.status == 422
    assert e.value.error.message == 'not allowed to edit system-library' \
                                    ' catalog'


def test_embedded_system_catalog_missing_edit_link(admin_mc):
    """This test asserts that the system catalog is missing the 'update' link
    if system-catalog setting is set to 'bundled'"""
    client = admin_mc.client

    system_catalog_setting = client.by_id_setting("system-catalog")
    # this could potentially interfere with other tests if they were to rely
    # on system-catalog setting
    client.update_by_id_setting(id=system_catalog_setting.id, value="bundled")

    system_catalog = client.by_id_catalog("system-library")

    assert "update" not in system_catalog.links


@pytest.mark.nonparallel
def test_catalog_refresh(admin_mc):
    """Test that on refresh the response includes the names of the catalogs
    that are being refreshed"""
    client = admin_mc.client
    catalog = client.by_id_catalog("library")
    out = client.action(obj=catalog, action_name="refresh")
    assert out['catalogs'][0] == "library"

    catalogs = client.list_catalog()
    out = client.action(obj=catalogs, action_name="refresh")
    # It just needs to be more than none, other test can add/remove catalogs
    # so a hard count will break
    assert len(out['catalogs']) > 0, 'no catalogs in response'


def test_invalid_catalog_chart_names(admin_mc, remove_resource):
    """Test chart with invalid name in catalog error properly
    and test that a chart names are truncated and processed without
    error"""
    client = admin_mc.client
    name = random_str()
    catalog = client.create_catalog(name=name,
                                    branch="broke-charts",
                                    url=DEFAULT_CATALOG,
                                    )
    remove_resource(catalog)
    wait_for_template_to_be_created(client, catalog.id)

    def get_errored_catalog(catalog):
        catalog = client.reload(catalog)
        if catalog.transitioning == "error":
            return catalog
        return None
    catalog = wait_for(lambda: get_errored_catalog(catalog),
                       fail_handler=lambda:
                       "catalog was not found in error state")
    templates = client.list_template(catalogId=catalog.id).data
    templatesString = ','.join([str(i) for i in templates])
    assert "areallylongname" not in templatesString
    assert "bad-chart_name" not in templatesString
    assert catalog.state == "processed"
    assert catalog.transitioning == "error"
    assert "Error in chart(s):" in catalog.transitioningMessage
    assert "bad-chart_name" in catalog.transitioningMessage
    assert "areallylongname" in catalog.transitioningMessage
    # this will break if github repo changes
    assert len(templates) == 6
    # checking that the errored catalog can be deleted successfully
    client.delete(catalog)
    wait_for_template_to_be_deleted(client, name)
    assert not client.list_catalog(name=name).data


def test_invalid_catalog_chart_urls(admin_mc, remove_resource):
    """Test chart with file:// and local:// url paths"""
    client = admin_mc.client
    name = random_str()
    catalog = client.create_catalog(name=name,
                                    branch="invalid-urls",
                                    url=DEFAULT_CATALOG,
                                    )
    remove_resource(catalog)
    wait_for_template_to_be_created(client, catalog.id)

    def get_errored_catalog(catalog):
        catalog = client.reload(catalog)
        if catalog.transitioning == "error":
            return catalog
        return None
    catalog = wait_for(lambda: get_errored_catalog(catalog),
                       fail_handler=lambda:
                       "catalog was not found in error state")
    templates = client.list_template(catalogId=catalog.id).data
    templatesString = ','.join([str(i) for i in templates])
    # url in index.yaml:
    # local://azure-samples.github.io/helm-charts/aks-helloworld-0.1.0.tgz
    assert "aks-goodbyeworld" not in templatesString
    # url in index.yaml:
    # file://azure-samples.github.io/helm-charts/aks-helloworld-0.1.0.tgz
    assert "aks-helloworld" not in templatesString
    assert catalog.state == "processed"
    assert catalog.transitioning == "error"
    assert "Error in chart(s):" in catalog.transitioningMessage
    assert "aks-goodbyeworld" in catalog.transitioningMessage
    assert "aks-helloworld" in catalog.transitioningMessage
    # this will break if github repo changes
    # valid url in index.yaml:
    # https://azure-samples.github.io/helm-charts/azure-vote-0.1.0.tgz
    assert len(templates) == 1
    # checking that the errored catalog can be deleted successfully
    client.delete(catalog)
    wait_for_template_to_be_deleted(client, name)
    assert not client.list_catalog(name=name).data


def test_catalog_has_helmversion(admin_mc, remove_resource):
    """Test to see that the helm version can be added to a catalog
    on create and that the value is passed to the template"""
    client = admin_mc.client
    name1 = random_str()
    name2 = random_str()
    catalog1 = client.create_catalog(name=name1,
                                     branch="master",
                                     url=DEFAULT_CATALOG,
                                     )
    remove_resource(catalog1)
    catalog2 = client.create_catalog(name=name2,
                                     branch="master",
                                     url=DEFAULT_CATALOG,
                                     helmVersion="helm_v3"
                                     )

    remove_resource(catalog2)
    wait_for_template_to_be_created(client, name1)
    wait_for_template_to_be_created(client, name2)
    assert "helm_v3" not in catalog1
    assert catalog2.helmVersion == "helm_v3"
    templates1 = client.list_template(catalogId=catalog1.id).data
    for template in templates1:
        assert "helmVersion" not in template.status
    templates2 = client.list_template(catalogId=catalog2.id).data
    for template in templates2:
        assert "helmVersion" in template.status
        assert template.status.helmVersion == "helm_v3"


def test_refresh_catalog_access(admin_mc, user_mc):
    """Tests that a user with standard access is not
    able to refresh a catalog.
    """
    catalog = admin_mc.client.by_id_catalog("library")
    out = admin_mc.client.action(obj=catalog, action_name="refresh")
    assert out['catalogs'][0] == "library"
    # use catalog obj from admin client to get action not available to user
    with pytest.raises(ApiError) as e:
        user_mc.client.action(obj=catalog, action_name="refresh")
    assert e.value.error.status == 404
