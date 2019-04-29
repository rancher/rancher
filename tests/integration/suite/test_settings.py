import pytest
from rancher import ApiError


# cacerts is readOnly, and should not be able to be set through the API
def test_create_read_only(admin_mc, remove_resource):
    client = admin_mc.client

    with pytest.raises(ApiError) as e:
        client.create_setting(name="cacerts", value="a")

    assert e.value.error.status == 405
    assert "readOnly" in e.value.error.message


# cacerts is readOnly setting, and should not be able to be updated through API
def test_update_read_only(admin_mc, remove_resource):
    client = admin_mc.client

    with pytest.raises(ApiError) as e:
        client.update_by_id_setting(id="cacerts", value="b")

    assert e.value.error.status == 405
    assert "readOnly" in e.value.error.message


# cacerts is readOnly, and should be able to be read
def test_get_read_only(admin_mc, remove_resource):
    client = admin_mc.client
    client.by_id_setting(id="cacerts")


# cacerts is readOnly, and user should not be able to delete
def test_delete_read_only(admin_mc, remove_resource):
    client = admin_mc.client
    setting = client.by_id_setting(id="cacerts")

    with pytest.raises(ApiError) as e:
        client.delete(setting)

    assert e.value.error.status == 405
    assert "readOnly" in e.value.error.message


# user should be able to create a setting that does not exist yet
def test_create(admin_mc, remove_resource):
    client = admin_mc.client
    setting = client.create_setting(name="samplesetting1", value="a")
    remove_resource(setting)

    assert setting.value == "a"


# user should not be able to create a setting if it already exists
def test_create_existing(admin_mc, remove_resource):
    client = admin_mc.client
    setting = client.create_setting(name="samplefsetting2", value="a")
    remove_resource(setting)

    with pytest.raises(ApiError) as e:
        setting2 = client.create_setting(name="samplefsetting2", value="a")
        remove_resource(setting2)

    assert e.value.error.status == 409
    assert e.value.error.code == "AlreadyExists"


# user should be able to update a setting if it exists
def test_update(admin_mc, remove_resource):
    client = admin_mc.client
    setting = client.create_setting(name="samplesetting3", value="a")
    remove_resource(setting)

    setting = client.update_by_id_setting(id="samplesetting3", value="b")
    assert setting.value == "b"


# user should not be able to update a setting if it does not exists
def test_update_nonexisting(admin_mc, remove_resource):
    client = admin_mc.client

    with pytest.raises(ApiError) as e:
        setting = client.update_by_id_setting(id="samplesetting4", value="a")
        remove_resource(setting)

    assert e.value.error.status == 404
    assert e.value.error.code == "NotFound"
