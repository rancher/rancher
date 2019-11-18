import pytest
from rancher import ApiError

from .common import random_str
from .conftest import wait_for


@pytest.mark.nonparallel
def test_builtin_default_can_be_edited(admin_mc, revert_gr):
    """Asserts admins can only edit a builtin global role's newUserDefault
    field"""
    admin_client = admin_mc.client

    gr = admin_client.by_id_global_role(id="admin")
    revert_gr(gr)
    assert gr.builtin is True
    assert "remove" not in gr.links.keys()
    assert gr.newUserDefault is False

    new_gr = admin_client.update_by_id_global_role(id=gr.id,
                                                   displayName="gr-test",
                                                   description="asdf",
                                                   rules=None,
                                                   newUserDefault=True,
                                                   builtin=True)
    assert new_gr.name == gr.name
    assert new_gr.get("description") == gr.description
    assert new_gr.rules is not None
    assert new_gr.get("builtin") is True

    # newUserDefault is the only field that should editable
    # for a builtin role
    assert new_gr.newUserDefault is True


def test_only_admin_can_crud_global_roles(admin_mc, user_mc, remove_resource):
    """Asserts that only admins can create, get, update, and delete
    non-builtin global roles"""
    admin_client = admin_mc.client
    user_client = user_mc.client

    gr = admin_client.create_global_role(name="gr-" + random_str())
    remove_resource(gr)

    gr.annotations = {"test": "asdf"}

    def try_gr_update():
        try:
            return admin_client.update_by_id_global_role(
                    id=gr.id,
                    value=gr)
        except ApiError as e:
            assert e.error.status == 404
            return False

    wait_for(try_gr_update)

    gr_list = admin_client.list_global_role()
    assert len(gr_list.data) > 0

    admin_client.delete(gr)

    with pytest.raises(ApiError) as e:
        gr2 = user_client.create_global_role(name="gr2-" + random_str())
        remove_resource(gr2)
    assert e.value.error.status == 403

    gr3 = admin_client.create_global_role(name="gr3-" + random_str())
    remove_resource(gr3)

    with pytest.raises(ApiError) as e:
        user_client.by_id_global_role(id=gr3.id)
    gr3.annotations = {"test2": "jkl"}

    def try_gr_unauth():
        with pytest.raises(ApiError) as e:
            user_client.update_by_id_global_role(id=gr3.id, value=gr3)
        if e.value.error.status == 404:
            return False
        assert e.value.error.status == 403
        return True

    wait_for(try_gr_unauth)

    gr_list = user_client.list_global_role()
    assert len(gr_list.data) == 0

    with pytest.raises(ApiError) as e:
        user_client.delete(gr3)
    assert e.value.error.status == 403


def test_admin_can_only_edit_builtin_global_roles(admin_mc, remove_resource):
    """Asserts admins can edit builtin global roles created by rancher but
    cannot delete them"""
    admin_client = admin_mc.client

    gr = admin_client.by_id_global_role(id="admin")
    assert gr.builtin is True
    assert "remove" not in gr.links.keys()

    gr2 = admin_client.create_global_role(name="gr2-" + random_str(),
                                          builtin=True)
    remove_resource(gr2)

    # assert that builtin cannot be set by admin and is false
    assert gr2.builtin is False

    admin_client.update_by_id_global_role(id=gr.id)

    with pytest.raises(ApiError) as e:
        admin_client.delete(gr)
    assert e.value.error.status == 403
    assert "cannot delete builtin global roles" in e.value.error.message


@pytest.fixture
def revert_gr(admin_mc, request):
    """Ensures gr was reverted to previous state, regardless of test results
    """
    def _cleanup(old_gr):
        def revert():
            reverted_gr = admin_mc.client.update_by_id_global_role(
                id=old_gr.id,
                displayName=old_gr.name,
                description=old_gr.description,
                rules=old_gr.rules,
                newUserDefault=old_gr.newUserDefault,
                builtin=old_gr.builtin)

            assert reverted_gr.name == old_gr.name
            assert reverted_gr.get("description") == old_gr.description
            assert reverted_gr.rules[0].data_dict() == old_gr.rules[0].\
                data_dict()
            assert reverted_gr.get("builtin") is old_gr.builtin
            assert reverted_gr.newUserDefault is old_gr.newUserDefault

        request.addfinalizer(revert)
    return _cleanup
