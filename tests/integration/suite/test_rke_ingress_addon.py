from .common import random_str


rke_config = {
        "type": "rancherKubernetesEngineConfig",
        "ingress": {
            "provider": "nginx",
            "extraEnvs": [{
                "name": "test",
                "value": "testval",
                "incorrectenvfield": "test",
            }],
            "extraVolumes": [{
                "name": "testvol",
                "mountPath": "test",
                "incorrectvolumesfield": "vol",
            }],
            "extraVolumeMounts": [{
                "name": "testvol",
                "emptyDir": {},
                "incorrectvolumemountfield": "volmount",
            }],
            "type": "ingressConfig"
        }
    }


def test_ingress_config_schema_validation(admin_mc, remove_resource):
    client = admin_mc.client
    cluster = client.create_cluster(
        name=random_str(), rancherKubernetesEngineConfig=rke_config)
    remove_resource(cluster)
    cluster = client.by_id_cluster(cluster.id)
    ingress = cluster['rancherKubernetesEngineConfig']['ingress']
    extra_envs = ingress['extraEnvs']
    for env in extra_envs:
        assert 'incorrectenvfield' not in env.keys()
    extra_vols = ingress['extraVolumes']
    for vol in extra_vols:
        assert 'incorrectvolumesfield' not in vol.keys()
    extra_vol_mount = ingress['extraVolumeMounts']
    for vol_mount in extra_vol_mount:
        assert 'incorrectvolumemountfield' not in vol_mount.keys()
