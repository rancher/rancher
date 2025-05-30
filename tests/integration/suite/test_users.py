import pytest
from rancher import ApiError

grbAnno = "cleanup.cattle.io/grbUpgradeCluster"
rtAnno = "cleanup.cattle.io/rtUpgradeCluster"


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


def test_user_cant_use_username_as_password(user_mc):
    client = user_mc.client
    with pytest.raises(ApiError) as e:
        client.create_user(username="administrator", password="administrator")

    assert e.value.error.status == 422


def test_password_too_short(user_mc):
    client = user_mc.client
    with pytest.raises(ApiError) as e:
        client.create_user(username="testuser", password="tooshort")

    assert e.value.error.status == 422
