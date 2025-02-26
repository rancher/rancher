import copy
import pytest

from .conftest import wait_for


@pytest.mark.nonparallel
def test_dynamic_schemas_update(request, admin_mc):
    assert not schema_has_field(admin_mc)

    eks_schema = admin_mc.client.by_id_dynamicSchema(
        'amazonelasticcontainerserviceconfig')

    new_field = copy.deepcopy(eks_schema.resourceFields['displayName'])
    new_field.description = 'My special field.'
    setattr(eks_schema.resourceFields, 'mySpecialField', new_field)

    admin_mc.client.update_by_id_dynamicSchema(eks_schema.id, eks_schema)
    request.addfinalizer(lambda: cleanup_extra_field(admin_mc))

    wait_for(lambda: schema_has_field(admin_mc),
             fail_handler=lambda: "could not add extra field",
             timeout=120)


def cleanup_extra_field(admin_mc):
    eks_schema = admin_mc.client.by_id_dynamicSchema(
        'amazonelasticcontainerserviceconfig')
    delattr(eks_schema.resourceFields, 'mySpecialField')
    admin_mc.client.delete(eks_schema)
    admin_mc.client.create_dynamicSchema(eks_schema)

    wait_for(lambda: not schema_has_field(admin_mc),
             fail_handler=lambda: "could not clean up extra field",
             timeout=120)


def schema_has_field(admin_mc):
    admin_mc.client.reload_schema()
    schemas = admin_mc.client.schema.types

    eks_schema = None
    for name, schema in schemas.items():
        if name == "amazonElasticContainerServiceConfig":
            eks_schema = schema

    return hasattr(eks_schema.resourceFields,
                   'mySpecialField') and eks_schema.resourceFields[
               'mySpecialField'] is not None
