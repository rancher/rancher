import pytest
import json


@pytest.mark.skip(reason="cluster-defaults disabled")
def test_initial_defaults(admin_mc):
    cclient = admin_mc.client
    schema_defaults = {}
    setting_defaults = {}

    data = cclient.schema.types['cluster'].resourceFields
    default = data["enableNetworkPolicy"]["default"]

    for name in cclient.schema.types['cluster'].resourceFields.keys():
        if name == "enableNetworkPolicy":
            schema_defaults["enableNetworkPolicy"] = default

    for name in cclient.schema.types['rancherKubernetesEngineConfig']\
            .resourceFields.keys():
        if name == "ignoreDockerVersion":
            schema_defaults["ignoreDockerVersion"] = cclient.schema.\
                types["rancherKubernetesEngineConfig"].\
                resourceFields["ignoreDockerVersion"].\
                data_dict()["default"]

    setting = cclient.list_setting(name="cluster-defaults")
    data = json.loads(setting['data'][0]['default'])

    setting_defaults["enableNetworkPolicy"] = data["enableNetworkPolicy"]
    setting_defaults["ignoreDockerVersion"] = \
        data["rancherKubernetesEngineConfig"]["ignoreDockerVersion"]

    assert schema_defaults == setting_defaults
