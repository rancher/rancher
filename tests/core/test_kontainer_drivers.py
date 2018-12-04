import sys

import pytest

from .conftest import wait_for_condition, wait_until

DRIVER_URL = "https://github.com/rancher/kontainer-engine-driver-example/" \
             "releases/download/v0.2.0/kontainer-engine-driver-example-" \
             + sys.platform


def test_builtin_drivers_are_present(admin_mc):
    admin_mc.client.reload_schema()
    types = admin_mc.client.schema.types

    assert 'azureKubernetesServiceConfig' in types
    assert 'googleKubernetesEngineConfig' in types
    assert 'amazonElasticContainerServiceConfig' in types


@pytest.mark.nonparallel
def test_kontainer_driver_lifecycle(admin_mc, remove_resource):
    kd = admin_mc.client.create_kontainerDriver(
        createDynamicSchema=True,
        active=True,
        url=DRIVER_URL
    )
    remove_resource(kd)

    # Test that it is in downloading state while downloading
    wait_for_condition('Downloaded', 'Unknown', admin_mc.client, kd)
    kd = admin_mc.client.reload(kd)
    assert "downloading" == kd.state

    # test driver goes active and appears in schema
    wait_for_condition('Active', 'True', admin_mc.client, kd,
                       timeout=90)
    kd = admin_mc.client.reload(kd)
    verify_driver_in_types(admin_mc.client, kd)

    # verify the leading kontainer driver identifier and trailing system
    # type are removed from the name
    assert kd.name == "example"

    # test driver is removed from schema after deletion
    admin_mc.client.delete(kd)
    verify_driver_not_in_types(admin_mc.client, kd)


@pytest.mark.nonparallel
def test_enabling_driver_exposes_schema(admin_mc, remove_resource):
    kd = admin_mc.client.create_kontainerDriver(
        createDynamicSchema=True,
        active=False,
        url=DRIVER_URL
    )
    remove_resource(kd)

    wait_for_condition('Downloaded', 'Unknown', admin_mc.client, kd,
                       timeout=90)
    kd = admin_mc.client.reload(kd)

    verify_driver_not_in_types(admin_mc.client, kd)

    kd.active = True
    admin_mc.client.update_by_id_kontainerDriver(kd.id, kd)

    wait_for_condition('Active', 'True', admin_mc.client, kd,
                       timeout=90)
    kd = admin_mc.client.reload(kd)

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
