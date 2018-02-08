from common import random_str


def test_namespaced_secrets(pc):
    client = pc.client

    ns = pc.cluster.client.create_namespace(name=random_str(),
                                            projectId=pc.project.id)

    name = random_str()
    secret = client.create_namespaced_secret(name=name, namespaceId=ns.id,
                                             stringData={
                                                 'foo': 'bar'
                                             })

    assert secret.baseType == 'namespacedSecret'
    assert secret.type == 'namespacedSecret'
    assert secret.kind == 'Opaque'
    assert secret.name == name
    assert secret.data['foo'] == 'YmFy'

    secret.data['baz'] = 'YmFy'
    secret = client.update(secret, data=secret.data)
    assert secret is not None
    secret = client.reload(secret)

    assert secret.baseType == 'namespacedSecret'
    assert secret.type == 'namespacedSecret'
    assert secret.kind == 'Opaque'
    assert secret.name == name
    assert secret.data['foo'] == 'YmFy'
    assert secret.data['baz'] == 'YmFy'
    assert secret.namespaceId == ns.id
    assert 'namespace' not in secret
    assert secret.projectId == pc.project.id

    found = False
    for i in client.list_namespaced_secret():
        if i.id == secret.id:
            found = True
            break

    assert found

    client.delete(secret)


def test_namespaced_certificates(pc):
    client = pc.client

    ns = pc.cluster.client.create_namespace(name=random_str(),
                                            projectId=pc.project.id)

    name = random_str()
    cert = client.create_namespaced_certificate(name=name, key='keydata',
                                                namespaceId=ns.id,
                                                certs='certdata')
    assert cert.baseType == 'namespacedSecret'
    assert cert.type == 'namespacedCertificate'
    assert cert.name == name
    assert cert.certs == 'certdata'
    assert cert.namespaceId == ns.id
    assert cert.projectId == pc.project.id
    assert 'namespace' not in cert

    cert = client.update(cert, certs='certdata2')
    assert cert.namespaceId == ns.id
    assert cert.projectId == pc.project.id

    cert = client.reload(cert)

    assert cert.baseType == 'namespacedSecret'
    assert cert.type == 'namespacedCertificate'
    assert cert.name == name
    assert cert.certs == 'certdata2'
    assert cert.namespaceId == ns.id
    assert cert.projectId == pc.project.id

    found = False
    for i in client.list_namespaced_certificate():
        if i.id == cert.id:
            found = True
            break

    assert found

    cert = client.by_id_namespaced_certificate(cert.id)
    assert cert is not None

    client.delete(cert)


def test_namespaced_docker_credential(pc):
    client = pc.client

    ns = pc.cluster.client.create_namespace(name=random_str(),
                                            projectId=pc.project.id)

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
    assert cert.registries['index.docker.io']['username'] == 'foo'
    assert 'password' in cert.registries['index.docker.io']
    assert cert.namespaceId == ns.id
    assert cert.projectId == pc.project.id

    registries['two'] = {
        'username': 'blah'
    }

    cert = client.update(cert, registries=registries)
    cert = client.reload(cert)

    assert cert.baseType == 'namespacedSecret'
    assert cert.type == 'namespacedDockerCredential'
    assert cert.name == name
    assert cert.registries['index.docker.io']['username'] == 'foo'
    assert cert.registries['two']['username'] == 'blah'
    assert 'password' not in cert.registries['index.docker.io']
    assert cert.namespaceId == ns.id
    assert 'namespace' not in cert
    assert cert.projectId == pc.project.id

    found = False
    for i in client.list_namespaced_docker_credential():
        if i.id == cert.id:
            found = True
            break

    assert found

    cert = client.by_id_namespaced_docker_credential(cert.id)
    assert cert is not None

    client.delete(cert)


def test_namespaced_basic_auth(pc):
    client = pc.client

    ns = pc.cluster.client.create_namespace(name=random_str(),
                                            projectId=pc.project.id)

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
    assert cert.projectId == pc.project.id

    cert = client.update(cert, username='foo2')
    cert = client.reload(cert)

    assert cert.baseType == 'namespacedSecret'
    assert cert.type == 'namespacedBasicAuth'
    assert cert.name == name
    assert cert.username == 'foo2'
    assert 'password' not in cert
    assert cert.namespaceId == ns.id
    assert 'namespace' not in cert
    assert cert.projectId == pc.project.id

    found = False
    for i in client.list_namespaced_basic_auth():
        if i.id == cert.id:
            found = True
            break

    assert found

    cert = client.by_id_namespaced_basic_auth(cert.id)
    assert cert is not None

    client.delete(cert)


def test_namespaced_ssh_auth(pc):
    client = pc.client

    ns = pc.cluster.client.create_namespace(name=random_str(),
                                            projectId=pc.project.id)

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
    assert cert.projectId == pc.project.id

    cert = client.update(cert, privateKey='foo2')
    cert = client.reload(cert)
    assert cert.baseType == 'namespacedSecret'
    assert cert.type == 'namespacedSshAuth'
    assert cert.name == name
    assert 'privateKey' not in cert
    assert cert.namespaceId == ns.id
    assert 'namespace' not in cert
    assert cert.projectId == pc.project.id

    found = False
    for i in client.list_namespaced_ssh_auth():
        if i.id == cert.id:
            found = True
            break

    assert found

    cert = client.by_id_namespaced_ssh_auth(cert.id)
    assert cert is not None

    client.delete(cert)
