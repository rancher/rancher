def test_multi_user(admin_mc, user_mc):
    """Tests a bug in the python client where multiple clients would not
    work properly. All clients would get the auth header of the last  client"""
    # Original admin client should be able to get auth configs
    ac = admin_mc.client.list_auth_config()
    assert len(ac) > 0

    # User client should not. We currently dont 404 on this, which would be
    # more correct. Instead, list gets filtered to zero
    ac = user_mc.client.list_auth_config()
    assert len(ac) == 0
