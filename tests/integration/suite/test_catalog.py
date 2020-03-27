import pytest
import time

from rancher import ApiError
from .common import wait_for_template_to_be_created, \
    wait_for_template_to_be_deleted, random_str


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
        templates.append("library-"+t.name)

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
        templates.append(name+"-"+t.name)

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


def test_catalog_refresh(admin_mc):
    """Test that on refresh the response includes the names of the catalogs
    that are being refreshed"""
    catalog = admin_mc.client.by_id_catalog("library")
    try:
        admin_mc.client.action(obj=catalog, action_name="refresh")
    except ApiError as e:
        assert e is None


def test_refresh_catalog_access(admin_mc, user_mc):
    """Tests that a user with standard access is not
    able to refresh a catalog.
    """
    catalog = admin_mc.client.by_id_catalog("library")
    with pytest.raises(ApiError) as e:
        # use catalog obj from admin client to get action not available to user
        user_mc.client.action(obj=catalog, action_name="refresh")
    assert e.value.error.status == 404
