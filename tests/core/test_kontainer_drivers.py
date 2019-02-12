import sys

import pytest

from .conftest import wait_for_condition, wait_until

DRIVER_URL = "https://github.com/rancher/kontainer-engine-driver-example/" \
             "releases/download/v0.2.1/kontainer-engine-driver-example-" \
             + sys.platform


def test_builtin_drivers_are_present(admin_mc):
    admin_mc.client.reload_schema()
    types = admin_mc.client.schema.types

    for name in ['azureKubernetesService',
                 'googleKubernetesEngine',
                 'amazonElasticContainerService']:
        # check in schema
        assert name + "Config" in types

        # verify has no delete link because its built in
        kd = admin_mc.client.by_id_kontainer_driver(name.lower())
        assert not hasattr(kd.links, 'remove')


@pytest.mark.nonparallel
def test_kontainer_driver_lifecycle(admin_mc, remove_resource):
    kd = admin_mc.client.create_kontainerDriver(
        createDynamicSchema=True,
        active=True,
        url=DRIVER_URL
    )
    remove_resource(kd)

    # Test that it is in downloading state while downloading
    kd = wait_for_condition('Downloaded', 'Unknown', admin_mc.client, kd)
    assert "downloading" == kd.state

    # no actions should be present while downloading/installing
    assert not hasattr(kd, 'actions')

    # test driver goes active and appears in schema
    kd = wait_for_condition('Active', 'True', admin_mc.client, kd,
                            timeout=90)
    verify_driver_in_types(admin_mc.client, kd)

    # verify the leading kontainer driver identifier and trailing system
    # type are removed from the name
    assert kd.name == "example"

    # verify the kontainer driver has activate and no deactivate links
    assert not hasattr(kd.actions, "activate")
    assert hasattr(kd.actions, "deactivate")
    assert kd.actions.deactivate != ""

    # verify driver has delete link
    assert kd.links.remove != ""

    # test driver is removed from schema after deletion
    admin_mc.client.delete(kd)
    verify_driver_not_in_types(admin_mc.client, kd)


@pytest.mark.nonparallel
def test_enabling_driver_exposes_schema(admin_mc, remove_resource):
    """ Test if enabling driver exposes its dynamic schema, drivers are
     downloaded / installed once they are active """
    kd = admin_mc.client.create_kontainerDriver(
        createDynamicSchema=True,
        active=False,
        url=DRIVER_URL
    )
    remove_resource(kd)

    kd = wait_for_condition('Inactive', 'True', admin_mc.client, kd,
                            timeout=90)

    # verify the kontainer driver has no activate and a deactivate link
    assert hasattr(kd.actions, "activate")
    assert kd.actions.activate != ""
    assert not hasattr(kd.actions, "deactivate")

    verify_driver_not_in_types(admin_mc.client, kd)

    kd.active = True  # driver should begin downloading / installing
    admin_mc.client.update_by_id_kontainerDriver(kd.id, kd)

    kd = wait_for_condition('Active', 'True', admin_mc.client, kd,
                            timeout=90)

    verify_driver_in_types(admin_mc.client, kd)

    kd.active = False
    admin_mc.client.update_by_id_kontainerDriver(kd.id, kd)

    verify_driver_not_in_types(admin_mc.client, kd)


def verify_driver_in_types(client, kd):
    def check():
        client.reload_schema()
        types = client.schema.types
        return kd.name + 'EngineConfig' in types

    wait_until(check)
    client.reload_schema()
    assert kd.name + 'EngineConfig' in client.schema.types


def verify_driver_not_in_types(client, kd):
    def check():
        client.reload_schema()
        types = client.schema.types
        return kd.name + 'EngineConfig' not in types

    wait_until(check)
    client.reload_schema()
    assert kd.name + 'EngineConfig' not in client.schema.types
