from kubernetes.client import CustomObjectsApi
from kubernetes.client import CoreV1Api
from kubernetes.client.rest import ApiException
from .common import random_str
import base64

group = 'management.cattle.io'
version = 'v3'
namespace = 'local'
plural = 'clusterloggings'
clusterId = "local"
globalNS = "cattle-global-data"


def test_cluster_logging_elasticsearch(admin_mc, remove_resource):
    client = admin_mc.client
    secretPassword = random_str()
    indexPrefix = "prefix"
    endpoint = "https://localhost:8443/"
    name = random_str()

    es = client.create_cluster_logging(
                                        name=name,
                                        clusterId=clusterId,
                                        elasticsearchConfig={
                                            'authPassword': secretPassword,
                                            'endpoint': endpoint,
                                            'indexPrefix': indexPrefix})

    remove_resource(es)

    # Test password not present in api
    assert es is not None
    assert es['elasticsearchConfig'].get('authPassword') is None

    crdClient, k8sclient = getClients(admin_mc)
    ns, name = es["id"].split(":")
    # Test password is in k8s secret after creation
    verifyPassword(crdClient, k8sclient, ns, name, secretPassword)

    # Test noop, password field should be as it is
    es = client.update(es, elasticsearchConfig=es['elasticsearchConfig'])
    verifyPassword(crdClient, k8sclient, ns, name, secretPassword)

    # Test updating password
    newSecretPassword = random_str()
    es = client.update(es, elasticsearchConfig={
                                    'endpoint': endpoint,
                                    'authPassword': newSecretPassword})
    verifyPassword(crdClient, k8sclient, ns, name, newSecretPassword)

    # Test secret doesn't exist after object deletion
    checkSecret(crdClient, k8sclient, ns, name, es, client, deleteFunc)


def test_cluster_logging_fluentd(admin_mc, remove_resource):
    client = admin_mc.client
    fluentdservers = getFluentdServers()
    name = random_str()

    fs = client.create_cluster_logging(
                                    name=name,
                                    clusterId=clusterId,
                                    fluentForwarderConfig={
                                        'compress': "true",
                                        'enableTls': "false",
                                        'fluentServers': fluentdservers})

    remove_resource(fs)
    assert fs is not None
    servers = fs['fluentForwarderConfig'].get('fluentServers')
    assert len(servers) == 3

    # Test password not present in api
    for server in servers:
        assert server.get('password') is None

    crdClient, k8sclient = getClients(admin_mc)

    ns, name = fs['id'].split(":")
    # Test password is in k8s secret after creation
    verifyPasswords(crdClient, k8sclient, ns, name, fluentdservers)

    # Test noop, password field should be as it is
    fs = client.update(fs, fluentForwarderConfig=fs['fluentForwarderConfig'])
    verifyPasswords(crdClient, k8sclient, ns, name, fluentdservers)

    # Test updating password of one of the entries, no password passed in rest
    newSecretPassword = random_str()
    fs['fluentForwarderConfig'].\
        get('fluentServers')[2].password = newSecretPassword
    fluentdservers[2]['password'] = newSecretPassword

    fs = client.update(fs, fluentForwarderConfig=fs['fluentForwarderConfig'])
    verifyPasswords(crdClient, k8sclient, ns, name, fluentdservers)

    # Change array order (delete middle entry from array)
    servers = fs['fluentForwarderConfig'].get('fluentServers')
    del servers[1]
    del fluentdservers[1]
    config = {'fluentServers': servers}

    fs = client.update(fs, fluentForwarderConfig=config)
    verifyPasswords(crdClient, k8sclient, ns, name, fluentdservers)

    # Test secrets doesn't exist after object deletion
    checkSecrets(crdClient, k8sclient, ns, name, fs, client, deleteFunc)


def verifyPassword(crdClient, k8sclient, ns, name, secretPassword):
    k8es = crdClient.get_namespaced_custom_object(
            group, version, namespace, plural, name)

    secretName = k8es['spec']['elasticsearchConfig']['authPassword']
    ns, name = secretName.split(":")
    assert ns is not None
    assert name is not None

    secret = k8sclient.read_namespaced_secret(name, ns)
    assert base64.b64decode(secret.data[name]).\
        decode("utf-8") == secretPassword


def verifyPasswords(crdClient, k8sclient, ns, name, fluentdServers):
    k8fs = crdClient.get_namespaced_custom_object(
            group, version, namespace, plural, name)
    servers = k8fs['spec']['fluentForwarderConfig']['fluentServers']

    for ind, server in enumerate(fluentdServers):
        secretName = servers[ind]['password']
        ns, name = secretName.split(":")
        assert ns is not None
        assert name is not None

        secret = k8sclient.read_namespaced_secret(name, ns)
        assert base64.b64decode(secret.data[name]).\
            decode("utf-8") == server['password']


def checkSecret(crdClient, k8sclient, ns, name, es, client, func):
    k8es = crdClient.get_namespaced_custom_object(
            group, version, namespace, plural, name)
    secretName = k8es['spec']['elasticsearchConfig']['authPassword']
    ns, name = secretName.split(":")

    func(client, es)

    try:
        k8sclient.read_namespaced_secret(name, ns)
    except ApiException as e:
        assert e.status == 404


def checkSecrets(crdClient, k8sclient, ns, name, fs, client, func):
    k8fs = crdClient.get_namespaced_custom_object(
            group, version, namespace, plural, name)
    servers = k8fs['spec']['fluentForwarderConfig']['fluentServers']

    secretNames = []
    for ind, server in enumerate(servers):
        secretName = server['password']
        ns, name = secretName.split(":")
        secretNames.append(name)

    func(client, fs)

    for secretName in secretNames:
        try:
            k8sclient.read_namespaced_secret(name, globalNS)
        except ApiException as e:
            assert e.status == 404


def getClients(admin_mc):
    return CustomObjectsApi(admin_mc.k8s_client), \
        CoreV1Api(admin_mc.k8s_client)


def test_cluster_logging_null(admin_mc, remove_resource):
    client = admin_mc.client
    secretPassword = random_str()
    indexPrefix = "prefix"
    endpoint = "https://localhost:8443/"
    name = random_str()

    crdClient, k8sclient = getClients(admin_mc)

    es = client.create_cluster_logging(
                                        name=name,
                                        clusterId=clusterId,
                                        elasticsearchConfig={
                                            'authPassword': secretPassword,
                                            'endpoint': endpoint,
                                            'indexPrefix': indexPrefix})

    remove_resource(es)
    ns, name = es['id'].split(":")
    checkSecret(crdClient, k8sclient, ns, name, es, client, upFuncElastic)

    fluentdservers = getFluentdServers()
    name = random_str()

    fs = client.create_cluster_logging(
                                    name=name,
                                    clusterId=clusterId,
                                    fluentForwarderConfig={
                                        'compress': "true",
                                        'enableTls': "false",
                                        'fluentServers': fluentdservers})

    remove_resource(fs)

    ns, name = fs['id'].split(":")
    checkSecrets(crdClient, k8sclient, ns, name, fs, client, upFuncFluentd)


def upFuncFluentd(client, fs):
    try:
        fs = client.update(fs, fluentForwarderConfig=None)
    except ApiException as e:
        assert e is None


def upFuncElastic(client, es):
    try:
        es = client.update(es, elasticsearchConfig=None)
    except ApiException as e:
        assert e is None


def deleteFunc(client, obj):
    client.delete(obj)


def getFluentdServers():
    return [{
            "endpoint": "192.168.1.10:87",
            "standby": False,
            "username": random_str(),
            "weight": 100,
            "password": random_str()
            },
            {
            "endpoint": "192.168.1.10:89",
            "standby": False,
            "username": random_str(),
            "weight": 100,
            "password": random_str()
            },
            {
            "endpoint": "192.168.2.10:86",
            "standby": False,
            "username": random_str(),
            "weight": 100,
            "password": random_str()
            }]
