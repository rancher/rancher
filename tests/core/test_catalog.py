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
