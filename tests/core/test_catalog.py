from .common import wait_for_template_to_be_created, \
    wait_for_template_to_be_deleted, random_str
import time


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
    # Now list all templates of this catalog
    new_templates = client.list_template(catalogId=name).data
    for t in new_templates:
        templates.append(name+"-"+t.name)

    # Get the global role user, get all templates in it
    global_user_role = client.list_global_role(name="User").data[0]
    rules = global_user_role.rules
    found_templates = False
    # all_templates = existing_templates + new_templates
    # template_names
    for i in range(0, len(rules)):
        print("Pritning resource")
        print(rules[i].resources)
        if rules[i].resources[0] == "templates":
            found_templates = True
            all([z in templates for z in rules[i].resourceNames])

    assert found_templates is True

    client.delete(catalog)
    wait_for_template_to_be_deleted(client, name)
