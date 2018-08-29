import json


def test_initial_defaults(admin_mc):
    cclient = admin_mc.client
    schema_defaults = {}
    setting_defaults = {}

    data = cclient.schema.types['cluster'].resourceFields.data_dict()
    default = data["enableNetworkPolicy"].data_dict()["default"]

    for name in cclient.schema.types['cluster'].resourceFields.keys():
        if name == "enableNetworkPolicy":
            schema_defaults["enableNetworkPolicy"] = default

    for name in cclient.schema.types['rancherKubernetesEngineConfig']\
            .resourceFields.keys():
        if name == "ignoreDockerVersion":
            schema_defaults["ignoreDockerVersion"] = cclient.schema.\
                types["rancherKubernetesEngineConfig"].\
                resourceFields.data_dict()["ignoreDockerVersion"].\
                data_dict()["default"]

    setting = cclient.list_setting(name="cluster-defaults")
    data = json.loads(setting.data_dict()['data'][0].data_dict()['default'])

    setting_defaults["enableNetworkPolicy"] = data["enableNetworkPolicy"]
    setting_defaults["ignoreDockerVersion"] = \
        data["rancherKubernetesEngineConfig"]["ignoreDockerVersion"]

    assert schema_defaults == setting_defaults
