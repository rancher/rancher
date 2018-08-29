def test_certificates(admin_mc):
    client = admin_mc.client

    tokens = client.list_token()

    currentCount = 0
    for t in tokens:
        if t.current:
            assert t.userId == admin_mc.user.id
            currentCount += 1

    assert currentCount == 1
