from .common import random_str
from rancher import ApiError
import pytest
import time
import kubernetes


def test_dns_fqdn_unique(admin_mc):
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

    fqdn = random_str() + ".example.com"
    globaldns_entry = \
        client.create_global_dns(fqdn=fqdn, providerId=provider_name)

    with pytest.raises(ApiError) as e:
        client.create_global_dns(fqdn=fqdn, providerId=provider_name)
        assert e.value.error.status == 422

    client.delete(globaldns_entry)
    client.delete(globaldns_provider)


def test_dns_provider_deletion(admin_mc):
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

    fqdn = random_str() + ".example.com"
    provider_id = "cattle-global-data:"+provider_name
    globaldns_entry = \
        client.create_global_dns(fqdn=fqdn, providerId=provider_id)

    with pytest.raises(ApiError) as e:
        client.delete(globaldns_provider)
        assert e.value.error.status == 403

    client.delete(globaldns_entry)
    client.delete(globaldns_provider)


def test_share_globaldns_provider_entry(admin_mc, user_factory,
                                        remove_resource):
    client = admin_mc.client
    provider_name = random_str()
    access = random_str()
    secret = random_str()
    # Add regular user as member to gdns provider
    user_member = user_factory()
    remove_resource(user_member)
    user_client = user_member.client
    members = [{"userPrincipalId": "local://" + user_member.user.id,
                "accessType": "owner"}]
    globaldns_provider = \
        client.create_global_dns_provider(
            name=provider_name,
            rootDomain="example.com",
            route53ProviderConfig={
                'accessKey': access,
                'secretKey': secret},
            members=members)

    remove_resource(globaldns_provider)
    fqdn = random_str() + ".example.com"
    globaldns_entry = \
        client.create_global_dns(fqdn=fqdn, providerId=provider_name,
                                 members=members)
    remove_resource(globaldns_entry)
    # Make sure creator can access both, provider and entry
    gdns_provider_id = "cattle-global-data:" + provider_name
    gdns_provider = client.by_id_global_dns_provider(gdns_provider_id)
    assert gdns_provider is not None

    gdns_entry_id = "cattle-global-data:" + globaldns_entry.name
    gdns = client.by_id_global_dns(gdns_entry_id)
    assert gdns is not None
    # user should be able to list this gdns provider
    api_instance = kubernetes.client.RbacAuthorizationV1Api(
        admin_mc.k8s_client)
    provider_rb_name = provider_name + "-gp-a"
    wait_to_ensure_user_in_rb_subject(api_instance, provider_rb_name,
                                      user_member.user.id)
    gdns_provider = user_client.by_id_global_dns_provider(gdns_provider_id)
    assert gdns_provider is not None

    # user should be able to list this gdns entry
    entry_rb_name = globaldns_entry.name + "-g-a"
    wait_to_ensure_user_in_rb_subject(api_instance, entry_rb_name,
                                      user_member.user.id)
    gdns = user_client.by_id_global_dns(gdns_entry_id)
    assert gdns is not None


def test_user_access_global_dns(admin_mc, user_factory, remove_resource):
    user1 = user_factory()
    remove_resource(user1)
    user_client = user1.client
    provider_name = random_str()
    access = random_str()
    secret = random_str()
    globaldns_provider = \
        user_client.create_global_dns_provider(
            name=provider_name,
            rootDomain="example.com",
            route53ProviderConfig={
                'accessKey': access,
                'secretKey': secret})

    remove_resource(globaldns_provider)
    fqdn = random_str() + ".example.com"
    globaldns_entry = \
        user_client.create_global_dns(fqdn=fqdn, providerId=provider_name)

    remove_resource(globaldns_entry)
    # Make sure creator can access both, provider and entry
    api_instance = kubernetes.client.RbacAuthorizationV1Api(
        admin_mc.k8s_client)
    provider_rb_name = provider_name + "-gp-a"
    wait_to_ensure_user_in_rb_subject(api_instance, provider_rb_name,
                                      user1.user.id)

    gdns_provider_id = "cattle-global-data:" + provider_name
    gdns_provider = user_client.by_id_global_dns_provider(gdns_provider_id)
    assert gdns_provider is not None

    entry_rb_name = globaldns_entry.name + "-g-a"
    wait_to_ensure_user_in_rb_subject(api_instance, entry_rb_name,
                                      user1.user.id)
    gdns_entry_id = "cattle-global-data:" + globaldns_entry.name
    gdns = user_client.by_id_global_dns(gdns_entry_id)
    assert gdns is not None


def test_update_gdns_entry(admin_mc, remove_resource):
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
    fqdn = random_str() + ".example.com"
    gdns_entry_name = random_str()
    globaldns_entry = \
        client.create_global_dns(name=gdns_entry_name,
                                 fqdn=fqdn, providerId=provider_name)
    remove_resource(globaldns_entry)
    new_fqdn = random_str()
    wait_for_gdns_entry_creation(admin_mc, gdns_entry_name)
    client.update(globaldns_entry, fqdn=new_fqdn)
    wait_for_gdns_update(admin_mc, gdns_entry_name, new_fqdn)


def test_create_globaldns_provider_regular_user(remove_resource,
                                                user_factory):
    provider_name = random_str()
    access = random_str()
    secret = random_str()
    user = user_factory()
    globaldns_provider = \
        user.client.create_global_dns_provider(
            name=provider_name,
            rootDomain="example.com",
            route53ProviderConfig={
                'accessKey': access,
                'secretKey': secret})
    remove_resource(globaldns_provider)


def wait_to_ensure_user_in_rb_subject(api, name,
                                      userId, timeout=60):
    found = False
    interval = 0.5
    start = time.time()
    while not found:
        time.sleep(interval)
        interval *= 2
        try:
            rb = api.read_namespaced_role_binding(name, "cattle-global-data")
            for i in range(0, len(rb.subjects)):
                if rb.subjects[i].name == userId:
                    found = True
        except kubernetes.client.rest.ApiException:
            found = False
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for user to get added to rb")


def wait_for_gdns_update(admin_mc, gdns_entry_name, new_fqdn, timeout=60):
    client = admin_mc.client
    updated = False
    interval = 0.5
    start = time.time()
    id = "cattle-global-data:" + gdns_entry_name
    while not updated:
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for gdns entry to update')
        gdns = client.by_id_global_dns(id)
        if gdns is not None and gdns.fqdn == new_fqdn:
            updated = True
        time.sleep(interval)
        interval *= 2


def wait_for_gdns_entry_creation(admin_mc, gdns_name, timeout=60):
    start = time.time()
    interval = 0.5
    client = admin_mc.client
    found = False
    while not found:
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for globalDNS entry creation')
        gdns = client.list_global_dns(name=gdns_name)
        if len(gdns) > 0:
            found = True
        time.sleep(interval)
        interval *= 2


def test_cloudflare_provider_proxy_setting(admin_mc, remove_resource):
    client = admin_mc.client
    provider_name = random_str()
    apiEmail = random_str()
    apiKey = random_str()
    globaldns_provider = \
        client.create_global_dns_provider(
                                         name=provider_name,
                                         rootDomain="example.com",
                                         cloudflareProviderConfig={
                                             'proxySetting': True,
                                             'apiEmail': apiEmail,
                                             'apiKey': apiKey})

    gdns_provider_id = "cattle-global-data:" + provider_name
    gdns_provider = client.by_id_global_dns_provider(gdns_provider_id)
    assert gdns_provider is not None
    assert gdns_provider.cloudflareProviderConfig.proxySetting is True

    remove_resource(globaldns_provider)


def test_dns_fqdn_hostname(admin_mc, remove_resource):
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

    fqdn = random_str() + ".example!!!*.com"
    with pytest.raises(ApiError) as e:
        client.create_global_dns(fqdn=fqdn, providerId=provider_name)
        assert e.value.error.status == 422
