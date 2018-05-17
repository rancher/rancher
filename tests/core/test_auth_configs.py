import pytest
from cattle import ApiError


def test_auth_configs(mc):
    client = mc.client

    with pytest.raises(AttributeError) as e:
        client.list_github_config()

    with pytest.raises(AttributeError) as e:
        client.create_auth_config({})

    configs = client.list_auth_config()
    assert len(configs) == 4
    gh = None
    local = None
    ad = None
    azuread = None
    for c in configs:
        if c.type == "githubConfig":
            gh = c
        elif c.type == "localConfig":
            local = c
        elif c.type == "activeDirectoryConfig":
            ad = c
        elif c.type == "azureADConfig":
            azuread = c

    for x in [gh, local, ad, azuread]:
        assert x is not None
        config = client.by_id_auth_config(x.id)
        with pytest.raises(ApiError) as e:
            client.delete(config)
        assert e.value.error.status == 405

    assert gh.actions['testAndApply']
    assert gh.actions['configureTest']

    assert ad.actions['testAndApply']

    assert azuread.actions['testAndApply']
