from .common import random_str
from rancher import ApiError
import pytest


@pytest.mark.skip(reason='drone needs to change to run rancher in HA mode')
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


@pytest.mark.skip(reason='drone needs to change to run rancher in HA mode')
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
    globaldns_entry = \
        client.create_global_dns(fqdn=fqdn, providerId=provider_name)

    with pytest.raises(ApiError) as e:
        client.delete(globaldns_provider)
        assert e.value.error.status == 403

    client.delete(globaldns_entry)
    client.delete(globaldns_provider)
