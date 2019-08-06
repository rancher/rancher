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
