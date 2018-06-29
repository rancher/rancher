from .common import random_str
from .test_secrets import CERT, KEY

UPDATED_CERT = """-----BEGIN CERTIFICATE-----
MIIDEDCCAfgCCQC+HwE8rpMN7jANBgkqhkiG9w0BAQUFADBKMQswCQYDVQQGEwJV
UzEQMA4GA1UECBMHQXJpem9uYTEVMBMGA1UEChMMUmFuY2hlciBMYWJzMRIwEAYD
VQQDEwlsb2NhbGhvc3QwHhcNMTYwNjMwMDExMzMyWhcNMjYwNjI4MDExMzMyWjBK
MQswCQYDVQQGEwJVUzEQMA4GA1UECBMHQXJpem9uYTEVMBMGA1UEChMMUmFuY2hl
ciBMYWJzMRIwEAYDVQQDEwlsb2NhbGhvc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQC1PR0EiJjM0wbFQmU/yKSb7AuQdzhdW02ya+RQe+31/B+sOTMr
z9b473KCKf8LiFKFOIQUhR5fPvwyrrIWKCEV9pCp/wM474fX32j0zYaH6ezZjL0r
L6hTeGFScGse3dk7ej2+6nNWexpujos0djFi9Gu11iVHIJyT2Sx66kPPPZVRkJO9
5Pfetm5SLIQtJHUwy5iWv5Br+AbdXlUAjTYUqS4mhKIIbblAPbOKrYRxGXX/6oDV
J5OGLle8Uvlb8poxqmy67FPyMObNHhjggKwboXhmNuuT2OGf/VeZANMYubs4JP2V
ZLs3U/1tFMAOaQM+PbT9JuwMSmGYFX0Qiuh/AgMBAAEwDQYJKoZIhvcNAQEFBQAD
ggEBACpkRCQpCn/zmTOwboBckkOFeqMVo9cvSu0Sez6EPED4WUv/6q5tlJeHekQm
6YVcsXeOMkpfZ7qtGmBDwR+ly7D43dCiPKplm0uApO1CkogG5ePv0agvKHEybd36
xu9pt0fnxDdrP2NrP6trHq1D+CzPZooLRfmYqbt1xmIb00GpnyiJIUNuMu7GUM3q
NxWGK3eq+1cyt6xr8nLOC5zaGeSyZikw4+9vqLudNSyYdnw9mdHtrYT0GlcEP1Vc
NK+yrhDCvEWH6+4+pp8Ve2P2Le5tvbA1m24AxyuC9wHS5bUmiNHweLXNpxLFTjK8
BBUi6y1Vm9jrDi/LiiHcN4sJEoP=
-----END CERTIFICATE-----"""


def test_namespaced_secrets(admin_pc, admin_cc_client):
    client = admin_pc.client

    ns = admin_cc_client.create_namespace(name=random_str(),
                                          projectId=admin_pc.project.id)

    name = random_str()
    secret = client.create_namespaced_secret(name=name, namespaceId=ns.id,
                                             stringData={
                                                 'foo': 'bar'
                                             })

    assert secret.baseType == 'namespacedSecret'
    assert secret.type == 'namespacedSecret'
    assert secret.kind == 'Opaque'
    assert secret.name == name
    assert secret.data.foo == 'YmFy'

    secret.data.baz = 'YmFy'
    secret = client.update(secret, data=secret.data)
    assert secret is not None
    secret = client.reload(secret)

    assert secret.baseType == 'namespacedSecret'
    assert secret.type == 'namespacedSecret'
    assert secret.kind == 'Opaque'
    assert secret.name == name
    assert secret.data.foo == 'YmFy'
    assert secret.data.baz == 'YmFy'
    assert secret.namespaceId == ns.id
    assert 'namespace' not in secret.data
    assert secret.projectId == admin_pc.project.id

    found = False
    for i in client.list_namespaced_secret():
        if i.id == secret.id:
            found = True
            break

    assert found

    client.delete(secret)


def test_namespaced_certificates(admin_pc, admin_cc_client):
    client = admin_pc.client

    ns = admin_cc_client.create_namespace(name=random_str(),
                                          projectId=admin_pc.project.id)

    name = random_str()
    cert = client.create_namespaced_certificate(name=name, key=KEY,
                                                namespaceId=ns.id,
                                                certs=CERT)
    assert cert.baseType == 'namespacedSecret'
    assert cert.type == 'namespacedCertificate'
    assert cert.name == name
    assert cert.certs == CERT
    assert cert.namespaceId == ns.id
    assert cert.projectId == admin_pc.project.id
    assert 'namespace' not in cert

    cert = client.update(cert, certs=UPDATED_CERT)
    assert cert.namespaceId == ns.id
    assert cert.projectId == admin_pc.project.id

    cert = client.reload(cert)

    assert cert.baseType == 'namespacedSecret'
    assert cert.type == 'namespacedCertificate'
    assert cert.name == name
    assert cert.certs == UPDATED_CERT
    assert cert.namespaceId == ns.id
    assert cert.projectId == admin_pc.project.id

    found = False
    for i in client.list_namespaced_certificate():
        if i.id == cert.id:
            found = True
            break

    assert found

    cert = client.by_id_namespaced_certificate(cert.id)
    assert cert is not None

    client.delete(cert)


def test_namespaced_docker_credential(admin_pc, admin_cc_client):
    client = admin_pc.client

    ns = admin_cc_client.create_namespace(name=random_str(),
                                          projectId=admin_pc.project.id)

    name = random_str()
    registries = {'index.docker.io': {
        'username': 'foo',
        'password': 'bar',
    }}
    cert = client.create_namespaced_docker_credential(name=name,
                                                      namespaceId=ns.id,
                                                      registries=registries)
    assert cert.baseType == 'namespacedSecret'
    assert cert.type == 'namespacedDockerCredential'
    assert cert.name == name
    assert cert.registries.data_dict()['index.docker.io'].username == 'foo'
    assert 'password' in cert.registries.data_dict()['index.docker.io']
    assert cert.namespaceId == ns.id
    assert cert.projectId == admin_pc.project.id

    registries['two'] = {
        'username': 'blah'
    }

    cert = client.update(cert, registries=registries)
    cert = client.reload(cert)

    assert cert.baseType == 'namespacedSecret'
    assert cert.type == 'namespacedDockerCredential'
    assert cert.name == name
    assert cert.registries.data_dict()['index.docker.io'].username == 'foo'
    assert cert.registries.two.username == 'blah'
    assert 'password' not in cert.registries.data_dict()['index.docker.io']
    assert cert.namespaceId == ns.id
    assert 'namespace' not in cert
    assert cert.projectId == admin_pc.project.id

    found = False
    for i in client.list_namespaced_docker_credential():
        if i.id == cert.id:
            found = True
            break

    assert found

    cert = client.by_id_namespaced_docker_credential(cert.id)
    assert cert is not None

    client.delete(cert)


def test_namespaced_basic_auth(admin_pc, admin_cc_client):
    client = admin_pc.client

    ns = admin_cc_client.create_namespace(name=random_str(),
                                          projectId=admin_pc.project.id)

    name = random_str()
    cert = client.create_namespaced_basic_auth(name=name,
                                               namespaceId=ns.id,
                                               username='foo',
                                               password='bar')
    assert cert.baseType == 'namespacedSecret'
    assert cert.type == 'namespacedBasicAuth'
    assert cert.name == name
    assert cert.username == 'foo'
    assert 'password' in cert
    assert cert.namespaceId == ns.id
    assert 'namespace' not in cert
    assert cert.projectId == admin_pc.project.id

    cert = client.update(cert, username='foo2')
    cert = client.reload(cert)

    assert cert.baseType == 'namespacedSecret'
    assert cert.type == 'namespacedBasicAuth'
    assert cert.name == name
    assert cert.username == 'foo2'
    assert 'password' not in cert
    assert cert.namespaceId == ns.id
    assert 'namespace' not in cert
    assert cert.projectId == admin_pc.project.id

    found = False
    for i in client.list_namespaced_basic_auth():
        if i.id == cert.id:
            found = True
            break

    assert found

    cert = client.by_id_namespaced_basic_auth(cert.id)
    assert cert is not None

    client.delete(cert)


def test_namespaced_ssh_auth(admin_pc, admin_cc_client):
    client = admin_pc.client

    ns = admin_cc_client.create_namespace(name=random_str(),
                                          projectId=admin_pc.project.id)

    name = random_str()
    cert = client.create_namespaced_ssh_auth(name=name,
                                             namespaceId=ns.id,
                                             privateKey='foo')
    assert cert.baseType == 'namespacedSecret'
    assert cert.type == 'namespacedSshAuth'
    assert cert.name == name
    assert 'privateKey' in cert
    assert cert.namespaceId == ns.id
    assert 'namespace' not in cert
    assert cert.projectId == admin_pc.project.id

    cert = client.update(cert, privateKey='foo2')
    cert = client.reload(cert)
    assert cert.baseType == 'namespacedSecret'
    assert cert.type == 'namespacedSshAuth'
    assert cert.name == name
    assert 'privateKey' not in cert
    assert cert.namespaceId == ns.id
    assert 'namespace' not in cert
    assert cert.projectId == admin_pc.project.id

    found = False
    for i in client.list_namespaced_ssh_auth():
        if i.id == cert.id:
            found = True
            break

    assert found

    cert = client.by_id_namespaced_ssh_auth(cert.id)
    assert cert is not None

    client.delete(cert)
