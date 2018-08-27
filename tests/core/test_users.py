import pytest
from rancher import ApiError


def test_user_cant_delete_self(admin_mc):
    client = admin_mc.client
    with pytest.raises(ApiError) as e:
        client.delete(admin_mc.user)

    assert e.value.error.status == 422


def test_user_cant_deactivate_self(admin_mc):
    client = admin_mc.client
    with pytest.raises(ApiError) as e:
        client.update(admin_mc.user, enabled=False)

    assert e.value.error.status == 422
