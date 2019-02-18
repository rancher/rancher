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
                                         route53ProviderConfig={
                                             'accessKey': access,
                                             'secretKey': secret,
                                             'rootDomain': "example.com"})

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
                                         route53ProviderConfig={
                                             'accessKey': access,
                                             'secretKey': secret,
                                             'rootDomain': "example.com"})

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
            route53ProviderConfig={
                'accessKey': access,
                'secretKey': secret,
                'rootDomain': "example.com"},
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
            route53ProviderConfig={
                'accessKey': access,
                'secretKey': secret,
                'rootDomain': "example.com"})

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
