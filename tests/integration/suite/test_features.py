import pytest
from rancher import ApiError


# no one should be able to create features via the api
def test_cannot_create(admin_mc, user_mc, remove_resource):
    admin_client = admin_mc.client
    user_client = user_mc.client

    with pytest.raises(ApiError) as e:
        admin_client.create_feature(name="testfeature", value=True)

    assert e.value.error.status == 405

    with pytest.raises(ApiError) as e:
        user_client.create_feature(name="testfeature", value=True)

    assert e.value.error.status == 405


# users and admins should be able to list features
def test_can_list(admin_mc, user_mc, remove_resource):
    user_client = user_mc.client
    user_client.list_feature()
    assert True

    admin_client = admin_mc.client
    admin_client.list_feature()
    assert True
