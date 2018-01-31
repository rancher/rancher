def test_auth_configs(cc):
    client = cc.client

    configs = client.list_auth_config()
    assert len(configs) == 2
    gh = None
    local = None
    for c in configs:
        if c.type == "githubConfig":
            gh = c
        elif c.type == "localConfig":
            local = c

    assert gh is not None
    assert local is not None
    gh_config = client.by_id_github_config(gh.id)
    assert gh_config is not None
    gh_config = client.by_id_github_config(gh.id)
    assert gh_config is not None
