from common import random_str

CERT = """-----BEGIN CERTIFICATE-----
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
BBUi6y1Vm9jrDi/LiiHcN4sJEoU=
-----END CERTIFICATE-----"""

KEY = """-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAtT0dBIiYzNMGxUJlP8ikm+wLkHc4XVtNsmvkUHvt9fwfrDkz
K8/W+O9ygin/C4hShTiEFIUeXz78Mq6yFighFfaQqf8DOO+H199o9M2Gh+ns2Yy9
Ky+oU3hhUnBrHt3ZO3o9vupzVnsabo6LNHYxYvRrtdYlRyCck9kseupDzz2VUZCT
veT33rZuUiyELSR1MMuYlr+Qa/gG3V5VAI02FKkuJoSiCG25QD2ziq2EcRl1/+qA
1SeThi5XvFL5W/KaMapsuuxT8jDmzR4Y4ICsG6F4Zjbrk9jhn/1XmQDTGLm7OCT9
lWS7N1P9bRTADmkDPj20/SbsDEphmBV9EIrofwIDAQABAoIBAGehHxN1i3EqhKeL
9FrJPh4NlPswwCDZUQ7hFDZU9lZ9qBqQxkqZ18CVIXN90eBlPVIBY7xb9Wbem9Pb
AecbYPeu+T7KmqwWgiUUEG5RikfyoMQv7gZghK3dmkBKGWYX0dtpZR7h7bsYPp/S
j5QatNhxC5l4be5CnmUHe6B4jPdUt8kRfTj0ukYGm/h3cOm/tEQeRYIIN/N6JN2Z
JWYzsyqGmlOTp7suczkRIUS0AjiljT1186bQSou62iMtMqEgArusFFb9m/dXCCYo
t/Q1SR4lRodDfzcF/CRbdR/ZC8gZlyCdbI4WHOw9IwwHnmrllx4MXFP/p6p+gEtl
cKMzHXECgYEA27KnkDnz338qKC2cCGkMf3ARfTX6gSlqmvgM9zOa8FLWp6GR6Rvo
NgVLUi63bQqv9D5qYSsweAp1QTvIxJffWMJDTWtxowOXVW5P8WJ8jp/pAXoWGRbd
pnavy6Ih0XT57huwT7fGGIikXYfw/kB85PPJL3FsT/b6G4ay2+Z7OGkCgYEA0y+d
bxUewYZkpNy7+kIh0x4vrJvNqSL9ZwiP2R159zu7zDwDph/fkhXej0FEtbXybt+O
4s9M3l4nNsY6AS9sIPCB5SxWguhx0z76U5cz1qFFZwIHtL8r1jHrl5iwkVyOAtVV
0BokmJG4Pn07yZo/iCmSTEfwcePvCMvOsPtcvKcCgYEAu5+SbKChfhBaz19MLv6P
ttHdjcIogl/9dAU9BWxj+LO2MAjS1HKJ2ICi97d/3LbQ19TqArvgs9OymZhV+Fb/
Xgzhb1+/94icmFASI8KJP0CfvCwobRrTBlO8BDsdiITO4SNyalI28kLXpCzxiiFG
yDzOZx8FcjEpHZLmctgeCWkCgYAO0rDCM0FNZBl8WOH41tt47g16mBT/Yi1XJgqy
upbs+4xa8XtwFZyjrFVKyNIBzxuNHLPyx4olsYYfGhrIKoP0a+0yIMKRva7/nNQF
Of+xePBeIo5X6XMyPZ7DrTv3d/+fw0maqbsX2mKMQE4KAIGlFQXnxMTjuZP1khiX
44zG0QKBgGwQ8T4DGZK5ukLQmhLi9npCaAW99s/uuKArMzAG9xd/I8YntM/kVY0V
VUi3lKqwXhtReYdrqVTPdjnyGIYIGGNRD7EKqQe15IRfbpy536DSN+LvL65Fdyis
iNITDKNP1H3hedFNFfbTGpueYdRX6QaptK4+NB4+dOm7hn8iqq7U
-----END RSA PRIVATE KEY-----"""


def test_secrets(pc):
    client = pc.client

    name = random_str()
    secret = client.create_secret(name=name, stringData={
        'foo': 'bar'
    })

    assert secret.type == 'secret'
    assert secret.kind == 'Opaque'
    assert secret.name == name
    assert secret.data['foo'] == 'YmFy'

    secret.data['baz'] = 'YmFy'
    secret = client.update(secret, data=secret.data)
    secret = client.reload(secret)

    assert secret.baseType == 'secret'
    assert secret.type == 'secret'
    assert secret.kind == 'Opaque'
    assert secret.name == name
    assert secret.data['foo'] == 'YmFy'
    assert secret.data['baz'] == 'YmFy'
    assert secret.namespaceId is None
    assert 'namespace' not in secret
    assert secret.projectId == pc.project.id

    found = False
    for i in client.list_secret():
        if i.id == secret.id:
            found = True
            break

    assert found

    client.delete(secret)


def test_certificates(pc):
    client = pc.client

    name = random_str()
    cert = client.create_certificate(name=name, key=KEY, certs=CERT)
    assert cert.baseType == 'secret'
    assert cert.expiresAt == '2026-06-28T01:13:32Z'
    assert cert.type == 'certificate'
    assert cert.name == name
    assert cert.certs == CERT
    assert cert.namespaceId is None
    assert 'namespace' not in cert

    # cert = client.update(cert, certs='certdata2')
    # cert = client.reload(cert)
    #
    # assert cert.baseType == 'secret'
    # assert cert.type == 'certificate'
    # assert cert.name == name
    # assert cert.certs == 'certdata2'
    # assert cert.namespaceId is None
    # assert 'namespace' not in cert
    # assert cert.projectId == pc.project.id

    found = False
    for i in client.list_certificate():
        if i.id == cert.id:
            found = True
            break

    assert found

    cert = client.by_id_certificate(cert.id)
    assert cert is not None

    client.delete(cert)


def test_docker_credential(pc):
    client = pc.client

    name = random_str()
    registries = {'index.docker.io': {
        'username': 'foo',
        'password': 'bar',
    }}
    cert = client.create_docker_credential(name=name,
                                           registries=registries)
    assert cert.baseType == 'secret'
    assert cert.type == 'dockerCredential'
    assert cert.name == name
    assert cert.registries['index.docker.io']['username'] == 'foo'
    assert 'password' not in cert.registries['index.docker.io']
    assert cert.namespaceId is None
    assert 'namespace' not in cert
    assert cert.projectId == pc.project.id

    registries['two'] = {
        'username': 'blah'
    }

    cert = client.update(cert, registries=registries)
    cert = client.reload(cert)

    assert cert.baseType == 'secret'
    assert cert.type == 'dockerCredential'
    assert cert.name == name
    assert cert.registries['index.docker.io']['username'] == 'foo'
    assert cert.registries['two']['username'] == 'blah'
    assert 'password' not in cert.registries['index.docker.io']
    assert cert.namespaceId is None
    assert 'namespace' not in cert
    assert cert.projectId == pc.project.id

    found = False
    for i in client.list_docker_credential():
        if i.id == cert.id:
            found = True
            break

    assert found

    cert = client.by_id_docker_credential(cert.id)
    assert cert is not None

    client.delete(cert)


def test_basic_auth(pc):
    client = pc.client

    name = random_str()
    cert = client.create_basic_auth(name=name,
                                    username='foo',
                                    password='bar')
    assert cert.baseType == 'secret'
    assert cert.type == 'basicAuth'
    assert cert.name == name
    assert cert.username == 'foo'
    assert 'password' not in cert
    assert cert.namespaceId is None
    assert 'namespace' not in cert
    assert cert.projectId == pc.project.id

    cert = client.update(cert, username='foo2')
    cert = client.reload(cert)

    assert cert.baseType == 'secret'
    assert cert.type == 'basicAuth'
    assert cert.name == name
    assert cert.username == 'foo2'
    assert 'password' not in cert
    assert cert.namespaceId is None
    assert 'namespace' not in cert
    assert cert.projectId == pc.project.id

    found = False
    for i in client.list_basic_auth():
        if i.id == cert.id:
            found = True
            break

    assert found

    cert = client.by_id_basic_auth(cert.id)
    assert cert is not None

    client.delete(cert)


def test_ssh_auth(pc):
    client = pc.client

    name = random_str()
    cert = client.create_ssh_auth(name=name,
                                  privateKey='foo')
    assert cert.baseType == 'secret'
    assert cert.type == 'sshAuth'
    assert cert.name == name
    assert 'privateKey' not in cert
    assert cert.namespaceId is None
    assert 'namespace' not in cert
    assert cert.projectId == pc.project.id

    cert = client.update(cert, privateKey='foo2')
    cert = client.reload(cert)
    assert cert.baseType == 'secret'
    assert cert.type == 'sshAuth'
    assert cert.name == name
    assert 'privateKey' not in cert
    assert cert.namespaceId is None
    assert 'namespace' not in cert
    assert cert.projectId == pc.project.id

    found = False
    for i in client.list_ssh_auth():
        if i.id == cert.id:
            found = True
            break

    assert found

    cert = client.by_id_ssh_auth(cert.id)
    assert cert is not None

    client.delete(cert)
