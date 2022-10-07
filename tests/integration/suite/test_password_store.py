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
