from .common import random_str

import time


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
