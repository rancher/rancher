import pytest
from rancher import ApiError


def test_auth_configs(admin_mc):
    client = admin_mc.client

    with pytest.raises(AttributeError) as e:
        client.list_github_config()

    with pytest.raises(AttributeError) as e:
        client.create_auth_config({})

    configs = client.list_auth_config()

    assert configs.pagination.total == 8

    gh = None
    local = None
    ad = None
    azure = None
    openldap = None
    freeIpa = None
    ping = None
    adfs = None

    for c in configs:
        if c.type == "githubConfig":
            gh = c
        elif c.type == "localConfig":
            local = c
        elif c.type == "activeDirectoryConfig":
            ad = c
        elif c.type == "azureADConfig":
            azure = c
        elif c.type == "openLdapConfig":
            openldap = c
        elif c.type == "freeIpaConfig":
            freeIpa = c
        elif c.type == "pingConfig":
            ping = c
        elif c.type == "adfsConfig":
            adfs = c

    for x in [gh, local, ad, azure, openldap, freeIpa, ping, adfs]:
        assert x is not None
        config = client.by_id_auth_config(x.id)
        with pytest.raises(ApiError) as e:
            client.delete(config)
        assert e.value.error.status == 405

    assert gh.actions.testAndApply
    assert gh.actions.configureTest

    assert ad.actions.testAndApply

    assert azure.actions.testAndApply
    assert azure.actions.configureTest

    assert openldap.actions.testAndApply

    assert freeIpa.actions.testAndApply

    assert ping.actions.testAndEnable

    assert adfs.actions.testAndEnable
