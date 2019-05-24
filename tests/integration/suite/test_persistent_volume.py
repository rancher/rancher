from .common import random_str


def test_persistent_volume_update(admin_cc, remove_resource):
    client = admin_cc.client
    name = random_str()
    pv = client.create_persistent_volume(
        clusterId="local",
        name=name,
        accessModes=["ReadWriteOnce"],
        capacity={"storage": "10Gi"},
        cinder={"readOnly": "false",
                "secretRef": {"name": "fss",
                              "namespace": "fsf"},
                "volumeID": "fss",
                "fsType": "fss"})
    remove_resource(pv)
    assert pv is not None
    # fields shouldn't get updated
    toUpdate = {"readOnly": "true"}
    pv = client.update(pv, cinder=toUpdate)
    assert (pv["cinder"]["readOnly"]) is False
    # persistentVolumeSource type cannot be changed
    pv = client.update(pv, azureFile={"readOnly": "true",
                                      "shareName": "abc"}, cinder={})
    assert "azureFile" not in pv
