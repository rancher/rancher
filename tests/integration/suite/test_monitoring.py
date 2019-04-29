from .conftest import wait_for_condition
import time


def test_monitoring_version_upgrade(admin_mc):
    client = admin_mc.client
    catalogs = client.list_catalog(name="system-library")
    assert len(catalogs) == 1
    systemlibrary = catalogs.data[0]
    oldurl = systemlibrary.url
    oldbranch = systemlibrary.branch
    client.delete(systemlibrary)
    newlibrary = client.create_catalog(
                name="system-library",
                branch="master",
                url="https://github.com/orangedeng/test-charts",
                )

    newlibrary = client.reload(newlibrary)
    client.action(obj=systemlibrary, action_name="refresh")
    wait_for_condition('Refreshed', 'True', client, newlibrary)

    settings = client.list_setting(name="system-monitoring-catalog-id")
    assert len(settings) == 1
    value = 'catalog://?catalog=system-library' + \
        '&template=rancher-monitoring&version=9.9.9'
    wait_for_setting_update(settings.data[0], value, client)

    client.delete(newlibrary)
    systemlibrary = client.create_catalog(
            name="system-library",
            branch=oldbranch,
            url=oldurl,
            )
    systemlibrary = client.reload(systemlibrary)
    client.action(obj=systemlibrary, action_name="refresh")
    wait_for_condition('Refreshed', 'True', client, systemlibrary)


def wait_for_setting_update(obj, value, client, timeout=45):
    start = time.time()
    obj = client.reload(obj)
    sleep = 0.01
    while obj.default != value:
        time.sleep(sleep)
        sleep *= 2
        if sleep > 2:
            sleep = 2
        obj = client.reload(obj)
        delta = time.time() - start
        if delta > timeout:
            msg = 'Expected setting {} to have default {}\n'\
                'Timeout waiting for [{}:{}] for setting after {} ' \
                'seconds\n {}'.format(obj.name, value, obj.type, obj.id,
                                      delta, str(obj))
            raise Exception(msg)
