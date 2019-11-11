import platform
import pytest
import sys
import requests
from rancher import ApiError

from .conftest import wait_for_condition, wait_until, random_str, \
                    wait_for, BASE_URL

NEW_DRIVER_URL = "https://github.com/rancher/kontainer-engine-driver-" \
                 "example/releases/download/v0.2.2/kontainer-engine-" \
                 "driver-example-" + sys.platform + "-amd64"
NEW_DRIVER_ARM64_URL = "https://github.com/rancher/kontainer-engine-driver-" \
                       "example/releases/download/v0.2.2/kontainer-engine-" \
                       "driver-example-" + sys.platform + "-arm64"
DRIVER_AMD64_URL = "https://github.com/rancher/" \
                   "kontainer-engine-driver-example/" \
                   "releases/download/v0.2.1/kontainer-engine-driver-example-"\
                   + sys.platform
DRIVER_ARM64_URL = "https://github.com/jianghang8421/" \
                   "kontainer-engine-driver-example/" \
                   "releases/download/v0.2.1-multiarch/" \
                   "kontainer-engine-driver-example-" \
                   + sys.platform + "-arm64"


def test_builtin_drivers_are_present(admin_mc):
    """Test if builtin kd are present and cannot be deleted via API or UI"""
    admin_mc.client.reload_schema()
    types = admin_mc.client.schema.types

    for name in ['azureKubernetesService',
                 'googleKubernetesEngine',
                 'amazonElasticContainerService']:
        kd = admin_mc.client.list_kontainerDriver(
            name=name,
        ).data[0]
        wait_for_condition('Active', "True", admin_mc.client, kd, timeout=90)
        # check in schema
        assert name + "Config" in types

        # verify has no delete link because its built in
        kd = admin_mc.client.by_id_kontainer_driver(name.lower())
        assert not hasattr(kd.links, 'remove')
        # assert cannot delete it via API
        with pytest.raises(ApiError) as e:
            admin_mc.client.delete(kd)

        assert e.value.error.status == 405


@pytest.mark.skip
@pytest.mark.nonparallel
def test_kontainer_driver_lifecycle(admin_mc, list_remove_resource):
    URL = DRIVER_AMD64_URL
    if platform.machine() == "aarch64":
        URL = DRIVER_ARM64_URL
    kd = admin_mc.client.create_kontainerDriver(
        createDynamicSchema=True,
        active=True,
        url=URL
    )
    remove_list = [kd]
    list_remove_resource(remove_list)

    # Test that it is in downloading state while downloading
    kd = wait_for_condition('Downloaded', 'Unknown', admin_mc.client, kd)
    assert "downloading" == kd.state

    # no actions should be present while downloading/installing
    assert not hasattr(kd, 'actions')

    # test driver goes active and appears in schema
    kd = wait_for_condition('Active', 'True', admin_mc.client, kd,
                            timeout=90)
    verify_driver_in_types(admin_mc.client, kd)
    # verify the leading kontainer driver identifier and trailing system
    # type are removed from the name
    assert kd.name == "example"

    # verify the kontainer driver has activate and no deactivate links
    assert not hasattr(kd.actions, "activate")
    assert hasattr(kd.actions, "deactivate")
    assert kd.actions.deactivate != ""

    # verify driver has delete link
    assert kd.links.remove != ""

    # associate driver with a cluster
    cluster = admin_mc.client.create_cluster(
        name=random_str(),
        exampleEngineConfig={
            "credentials": "bad credentials",
            "nodeCount": 3
        })
    # order matters here, need to remove cluster before kontainer driver
    remove_list.insert(0, cluster)

    def check_remove_link(kod):
        kod = admin_mc.client.reload(kod)
        if hasattr(kod.links, "remove"):
            return False
        return True

    wait_for(lambda: check_remove_link(kd))
    with pytest.raises(ApiError) as e:
        admin_mc.client.delete(kd)

    assert e.value.error.status == 405

    # cleanup local cluster, note this depends on a force delete of the cluster
    # within rancher since this cluster is not a "true" cluster

    def cluster_steady_state(clus):
        clus = admin_mc.client.reload(clus)
        if "lifecycle.cattle.io/" \
           "create.mgmt-cluster-rbac-remove" in clus.annotations:
            return True
        return False

    # this typically takes at least 45 seconds
    wait_for(lambda: cluster_steady_state(cluster), timeout=90)
    admin_mc.client.delete(cluster)
    # wait for removal link to return
    wait_for(lambda: not (check_remove_link(kd)), timeout=90)
    admin_mc.client.delete(kd)
    # test driver is removed from schema after deletion
    verify_driver_not_in_types(admin_mc.client, kd)


@pytest.mark.skip
@pytest.mark.nonparallel
def test_enabling_driver_exposes_schema(admin_mc, wait_remove_resource):
    """ Test if enabling driver exposes its dynamic schema, drivers are
    downloaded / installed once they are active, and if re-activating a
    driver exposes its schema again"""
    URL = DRIVER_AMD64_URL
    if platform.machine() == "aarch64":
        URL = DRIVER_ARM64_URL
    kd = admin_mc.client.create_kontainerDriver(
        createDynamicSchema=True,
        active=False,
        url=URL
    )
    wait_remove_resource(kd)

    kd = wait_for_condition('Inactive', 'True', admin_mc.client, kd,
                            timeout=90)

    # verify the kontainer driver has no activate and a deactivate link
    assert hasattr(kd.actions, "activate")
    assert kd.actions.activate != ""
    assert not hasattr(kd.actions, "deactivate")

    verify_driver_not_in_types(admin_mc.client, kd)

    kd.active = True  # driver should begin downloading / installing
    admin_mc.client.update_by_id_kontainerDriver(kd.id, kd)

    kd = wait_for_condition('Active', 'True', admin_mc.client, kd,
                            timeout=90)

    verify_driver_in_types(admin_mc.client, kd)

    kd.active = False
    admin_mc.client.update_by_id_kontainerDriver(kd.id, kd)

    verify_driver_not_in_types(admin_mc.client, kd)

    # test re-activation flow
    kd.active = True
    admin_mc.client.update_by_id_kontainerDriver(kd.id, kd)
    verify_driver_in_types(admin_mc.client, kd)


@pytest.mark.skip
@pytest.mark.nonparallel
def test_upgrade_changes_schema(admin_mc, wait_remove_resource):
    client = admin_mc.client
    URL = DRIVER_AMD64_URL
    if platform.machine() == "aarch64":
        URL = DRIVER_ARM64_URL
    kd = client.create_kontainerDriver(
        createDynamicSchema=True,
        active=True,
        url=URL
    )
    wait_remove_resource(kd)

    kd = wait_for_condition('Active', 'True', admin_mc.client, kd,
                            timeout=90)

    verify_driver_in_types(client, kd)
    kdSchema = client.schema.types[kd.name + 'EngineConfig']
    assert 'specialTestingField' not in kdSchema.resourceFields

    NEW_URL = NEW_DRIVER_URL
    if platform.machine() == "aarch64":
        NEW_URL = NEW_DRIVER_ARM64_URL
    kd.url = NEW_URL
    kd = client.update_by_id_kontainerDriver(kd.id, kd)

    def schema_updated():
        client.reload_schema()
        kdSchema = client.schema.types[kd.name + 'EngineConfig']
        return 'specialTestingField' in kdSchema.resourceFields

    wait_until(schema_updated)

    kdSchema = client.schema.types[kd.name + 'EngineConfig']
    assert 'specialTestingField' in kdSchema.resourceFields


@pytest.mark.skip
@pytest.mark.nonparallel
def test_create_duplicate_driver_conflict(admin_mc, wait_remove_resource):
    """ Test if adding a driver with a pre-existing driver's URL
    returns a conflict error"""
    URL = DRIVER_AMD64_URL
    if platform.machine() == "aarch64":
        URL = DRIVER_ARM64_URL
    kd = admin_mc.client.create_kontainerDriver(
        createDynamicSchema=True,
        active=True,
        url=URL
    )
    wait_remove_resource(kd)
    kd = wait_for_condition('Active', 'True', admin_mc.client, kd, timeout=90)

    try:
        kd2 = admin_mc.client.create_kontainerDriver(
            createDynamicSchema=True,
            active=True,
            url=URL
        )
        wait_remove_resource(kd2)
        pytest.fail("Failed to catch duplicate driver URL on create")
    except ApiError as e:
        assert e.error.status == 409
        assert "Driver URL already in use:" in e.error.message


@pytest.mark.skip
@pytest.mark.nonparallel
def test_update_duplicate_driver_conflict(admin_mc, wait_remove_resource):
    """ Test if updating a driver's URL to a pre-existing driver's URL
    returns a conflict error"""
    URL = DRIVER_AMD64_URL
    if platform.machine() == "aarch64":
        URL = DRIVER_ARM64_URL
    kd1 = admin_mc.client.create_kontainerDriver(
        createDynamicSchema=True,
        active=True,
        url=URL
    )
    wait_remove_resource(kd1)
    kd1 = wait_for_condition('Active', 'True', admin_mc.client, kd1,
                             timeout=90)

    kd2 = admin_mc.client.create_kontainerDriver(
        createDynamicSchema=True,
        active=True,
        url=URL + "2"
    )
    wait_remove_resource(kd2)
    kd2.url = URL

    try:
        admin_mc.client.update_by_id_kontainerDriver(kd2.id, kd2)
        pytest.fail("Failed to catch duplicate driver URL on update")
    except ApiError as e:
        assert e.error.status == 409
        assert "Driver URL already in use:" in e.error.message


def test_kontainer_driver_links(admin_mc):
    client = admin_mc.client
    lister = client.list_kontainerDriver()
    assert 'rancher-images' in lister.links
    assert 'rancher-windows-images' in lister.links
    token = 'Bearer '+client.token
    url = BASE_URL + "/kontainerdrivers/rancher-images"
    images = get_images(url, token)
    assert "hyperkube" in images
    assert "rke-tools" in images
    assert "kubelet-pause" not in images
    # test windows link
    url = BASE_URL + "/kontainerdrivers/rancher-windows-images"
    images = get_images(url, token)
    assert "hyperkube" in images
    assert "rke-tools" in images
    assert "kubelet-pause" in images


def get_images(url, token):
    data = requests.get(
        url=url,
        verify=False,
        headers={'Accept': '*/*', 'Authorization': token})
    assert data is not None
    content = data.content.splitlines()
    assert len(content) > 0
    test = {}
    for line in content:
        if "rancher/hyperkube" in str(line):
            test["hyperkube"] = True
        elif "rancher/rke-tools" in str(line):
            test["rke-tools"] = True
        elif "rancher/kubelet-pause" in str(line):
            test["kubelet-pause"] = True
    return test


def verify_driver_in_types(client, kd):
    def check():
        client.reload_schema()
        types = client.schema.types
        return kd.name + 'EngineConfig' in types

    wait_until(check)
    client.reload_schema()
    assert kd.name + 'EngineConfig' in client.schema.types


def verify_driver_not_in_types(client, kd):
    def check():
        client.reload_schema()
        types = client.schema.types
        return kd.name + 'EngineConfig' not in types

    wait_until(check)
    client.reload_schema()
    assert kd.name + 'EngineConfig' not in client.schema.types


@pytest.mark.nonparallel
def test_user_update_settings(admin_mc):
    client = admin_mc.client
    k8s_version_setting = client.by_id_setting('k8s-version')
    default_k8s_version = k8s_version_setting["default"]
    k8s_versions_curr = client.by_id_setting(
        'k8s-versions-current')["value"].split(",")

    # user updates correct value
    user_value = k8s_versions_curr[0]
    updated_version = admin_mc.client.update_by_id_setting(
                        id='k8s-version', value=user_value)
    assert updated_version["default"] == default_k8s_version
    assert updated_version["value"] == user_value
    assert updated_version["labels"]["io.cattle.user.updated"] == "true"

    # assert refresh action doesn't override
    lister = client.list_kontainerDriver()

    try:
        client.action(obj=lister, action_name="refresh")
    except ApiError as e:
        assert e.value.error.status == 422

    new_k8s_version = client.by_id_setting('k8s-version')
    assert new_k8s_version["default"] == default_k8s_version
    assert new_k8s_version["value"] == user_value

    # user updates invalid value
    user_value = "v1.15.4-rancher13"
    try:
        updated_version = admin_mc.client.update_by_id_setting(
                            id='k8s-version', value=user_value)
    except ApiError as e:
        assert e.error.code == "MissingRequired"
        assert e.error.status == 422

    # bring back the default value, user updates with empty value
    user_value = ""
    updated_version = admin_mc.client.update_by_id_setting(
                        id='k8s-version', value=user_value)
    assert updated_version["default"] == default_k8s_version
    assert updated_version["value"] == default_k8s_version
    assert updated_version["labels"]["io.cattle.user.updated"] == "false"
