from .common import random_str


def test_user_input_capabilities_create(admin_mc, remove_resource):
    client = admin_mc.client
    name = random_str()
    # create cluster, set the capabilities in spec
    # only lb caps are allowed to be created/edited through API
    # rest should be set by the controller.
    cluster = client.\
        create_cluster(name=name,
                       rancherKubernetesEngineConfig={
                        "kubernetesVersion": "some-fake-version",
                        "cloudProvider": {
                            "name": "",
                        },
                        },
                       userInputCapabilities={
                        "loadBalancerCapabilities": {
                            "enabled": "true",
                        },
                        "ingressCapabilities": {
                            "ingressProvider": "test_provider",
                        },
                        "nodePoolScalingSupported": "true",
                        "nodePortRange": "10000-32000",
                        })
    remove_resource(cluster)
    cluster = client.reload(cluster)
    spec_capabilities = cluster['userInputCapabilities']
    # only lb capability can be set during creation,
    # so only that should be true
    assert spec_capabilities['loadBalancerCapabilities']['enabled'] is True
    # the other caps should not match the ones provided during create,
    # since these fields have the nocreate tag
    assert 'ingressCapabilities' not in spec_capabilities
    assert spec_capabilities['nodePoolScalingSupported'] is False
    assert 'nodePortRange' not in spec_capabilities


def test_user_input_capabilities_update(admin_mc, remove_resource):
    client = admin_mc.client
    name = random_str()
    # create cluster, set the capabilities in spec
    # only lb caps are allowed to be created/edited through API
    # rest should be set by the controller.
    cluster = client. \
        create_cluster(name=name,
                       rancherKubernetesEngineConfig={
                           "kubernetesVersion": "some-fake-version",
                           "cloudProvider": {
                               "name": "",
                           },
                       })
    remove_resource(cluster)
    cluster = client.reload(cluster)
    cluster = client.update(cluster, name=name,
                            userInputCapabilities={
                             "loadBalancerCapabilities": {
                                "enabled": "true",
                             },
                             "ingressCapabilities": {
                                "ingressProvider": "test_provider",
                             },
                             "nodePoolScalingSupported": "true",
                             "nodePortRange": "10000-32000",
                            })
    spec_capabilities = cluster['userInputCapabilities']
    # only lb capability can be set during update,
    # so only that should be true
    assert spec_capabilities['loadBalancerCapabilities']['enabled'] is True
    # the other caps should not match the ones provided during update,
    # since these fields have the noupdate tag
    assert 'ingressCapabilities' not in spec_capabilities
    assert spec_capabilities['nodePoolScalingSupported'] is False
    assert 'nodePortRange' not in spec_capabilities
