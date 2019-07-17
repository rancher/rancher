import pytest
import time
from rancher import ApiError
from .common import wait_for_template_to_be_created, \
    wait_for_template_to_be_deleted, random_str


def test_catalog(admin_mc):
    client = admin_mc.client
    name1 = random_str()
    name2 = random_str()
    url = "https://github.com/StrongMonkey/charts-1.git"
    catalog1 = client.create_catalog(name=name1,
                                     branch="test",
                                     url=url,
                                     )
    catalog2 = client.create_catalog(name=name2,
                                     branch="test",
                                     url=url,
                                     )
    wait_for_template_to_be_created(client, name1)
    wait_for_template_to_be_created(client, name2)
    client.delete(catalog1)
    client.delete(catalog2)
    wait_for_template_to_be_deleted(client, name1)
    wait_for_template_to_be_deleted(client, name2)


def test_invalid_catalog(admin_mc, remove_resource):
    client = admin_mc.client
    name = random_str()
    bad_url = "git://github.com/StrongMonkey/charts-1.git"
    # POST: Bad URL
    with pytest.raises(ApiError) as e:
        catalog = client.create_catalog(name=name,
                                        branch="test",
                                        url=bad_url,
                                        )
        remove_resource(catalog)
    assert e.value.error.status == 422
    # POST: No URL
    with pytest.raises(ApiError) as e:
        catalog = client.create_catalog(name=name,
                                        branch="test",
                                        url="",
                                        )
        remove_resource(catalog)
    assert e.value.error.status == 422
    # PUT: Bad URL
    good_url = "https://github.com/StrongMonkey/charts-1.git"
    catalog = client.create_catalog(name=name,
                                    branch="test",
                                    url=good_url,
                                    )
    remove_resource(catalog)
    wait_for_template_to_be_created(client, name)
    with pytest.raises(ApiError) as e:
        catalog.url = bad_url
        client.update_by_id_catalog(catalog.id, catalog)
    assert e.value.error.status == 422


def test_invalid_catalog_chars(admin_mc, remove_resource):
    client = admin_mc.client
    name = random_str()
    url = "https://github.com/%0dStrongMonkey%0A/charts-1.git"
    catalog = client.create_catalog(name=name,
                                    branch="test",
                                    url=url,
                                    )
    remove_resource(catalog)
    wait_for_template_to_be_created(client, name)
    catalog = client.reload(catalog)
    correct_url = "https://github.com/StrongMonkey/charts-1.git"
    assert catalog['url'] == correct_url


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
