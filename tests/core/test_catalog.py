from .common import wait_for_template_to_be_created, \
    wait_for_template_to_be_deleted, random_str


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
