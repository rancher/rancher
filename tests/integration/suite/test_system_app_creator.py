from .common import random_str
import time
import pytest


@pytest.mark.skip
def test_system_app_creator(admin_mc, admin_system_pc, remove_resource):
    client = admin_mc.client
    provider_name = random_str()
    access = random_str()
    secret = random_str()
    globaldns_provider = \
        client.create_global_dns_provider(
                                         name=provider_name,
                                         rootDomain="example.com",
                                         route53ProviderConfig={
                                             'accessKey': access,
                                             'secretKey': secret})
    remove_resource(globaldns_provider)
    app = wait_for_system_app(
        admin_system_pc.client,
        "systemapp-"+globaldns_provider.name)
    # the creator id of system app won't be listed in api
    assert app.creatorId != globaldns_provider.creatorId


def wait_for_system_app(client, name, timeout=60):
    start = time.time()
    interval = 0.5
    apps = client.list_app(name=name)
    while len(apps.data) != 1:
        if time.time() - start > timeout:
            print(apps)
            raise Exception('Timeout waiting for workload service')
        time.sleep(interval)
        interval *= 2
        apps = client.list_app(name=name)
    return apps.data[0]
