from kubernetes.client import CustomObjectsApi
from .common import random_str


def test_notifier_smtp_password(admin_mc, remove_resource):
    client = admin_mc.client
    name = random_str()
    password = random_str()
    notifier = client.create_notifier(clusterId="local",
                                      name=name,
                                      smtpConfig={
                                          "defaultRecipient": "test",
                                          "host": "test",
                                          "port": "587",
                                          "sender": "test",
                                          "tls": "true",
                                          "username": "test",
                                          "password": password
                                      })
    remove_resource(notifier)
    assert notifier is not None

    # Test password not present in api
    assert notifier['smtpConfig'].get('password') is None

    crd_client = get_crd_client(admin_mc)
    ns, name = notifier["id"].split(":")
    # Test password is in k8s  after creation
    verify_smtp_password(crd_client, ns, name, password)
    # Test noop, password field should be as it is
    notifier = client.update(notifier, smtpConfig=notifier['smtpConfig'])
    verify_smtp_password(crd_client, ns, name, password)
    # Test updating password
    new_password = random_str()
    notifier = client.update(notifier, smtpConfig={
        "password": new_password})
    verify_smtp_password(crd_client, ns, name, new_password)
    # Test updating field non-password related
    notifier = client.update(notifier, smtpConfig={"username": "test2"})
    notifier = client.reload(notifier)
    assert notifier["smtpConfig"]["username"] == "test2"
    # Test the password in crd remains the same value after updating username
    verify_smtp_password(crd_client, ns, name, new_password)


def verify_smtp_password(crd_client, ns, name, password):
    crd_dict = {
        'group': 'management.cattle.io',
        'version': 'v3',
        'namespace': 'local',
        'plural': 'notifiers',
        'name': name,
    }

    k8s_notifier = crd_client.get_namespaced_custom_object(**crd_dict)
    smtp_password = k8s_notifier['spec']['smtpConfig']['password']

    assert smtp_password == password


def get_crd_client(admin_mc):
    return CustomObjectsApi(admin_mc.k8s_client)
