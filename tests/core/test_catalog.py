from .common import random_str

import pytest
import time
from rancher import ApiError


def test_catalog(admin_mc):
    client = admin_mc.client
    name = random_str()
    url = "https://github.com/StrongMonkey/charts-1.git"
    catalog = client.create_catalog(name=name,
                                    branch="test",
                                    url=url,
                                    )
    wait_for_template_to_be_created(client, name)
    client.delete(catalog)
    wait_for_template_to_be_deleted(client, name)


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
    assert catalog.get("url") == correct_url


def wait_for_template_to_be_created(client, name, timeout=45):
    found = False
    start = time.time()
    interval = 0.5
    while not found:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for templates")
        templates = client.list_template(catalogId=name)
        if len(templates) > 0:
            found = True
        time.sleep(interval)
        interval *= 2


def wait_for_template_to_be_deleted(client, name, timeout=45):
    found = False
    start = time.time()
    interval = 0.5
    while not found:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for templates")
        templates = client.list_template(catalogId=name)
        if len(templates) == 0:
            found = True
        time.sleep(interval)
        interval *= 2
