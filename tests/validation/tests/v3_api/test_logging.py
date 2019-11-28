import pytest
import requests
import base64
import time
from .common import random_test_name, get_user_client
from .common import get_user_client_and_cluster
from .common import create_project_and_ns
from .common import get_project_client_for_token
from .common import create_kubeconfig
from .common import wait_for_app_to_active
from .common import CATTLE_TEST_URL
from .common import USER_TOKEN

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None,
             "name_prefix": None, "admin_client": None, "sys_p_client": None}
fluentd_aggregator_answers = {"defaultImage": "true",
                              "image.repository": "guangbo/fluentd",
                              "image.tag": "dev",
                              "replicas": "1",
                              "service.type": "ClusterIP",
                              "persistence.enabled": "false",
                              "extraPersistence.enabled": "false",
                              "extraPersistence.size": "10Gi",
                              "extraPersistence.mountPath": "/extra",
                              "extraPersistence.storageClass": "",
                              "output.type": "custom",
                              "output.flushInterval": "5s",
                              "output.customConf": "<match **.**>\n  @type stdout\n</match>"}

CATTLE_ClUSTER_LOGGING_FLUENTD_TEST = \
    CATTLE_TEST_URL + "/v3/clusterloggings?action=test"
CATTLE_PROJECT_LOGGING_FLUENTD_TEST = \
    CATTLE_TEST_URL + "/v3/projectloggings?action=test"
FLUENTD_AGGREGATOR_CATALOG_ID = "catalog://?catalog=library&template=fluentd-aggregator&version=0.3.0"


def test_send_log_to_fluentd(setup_fluentd_aggregator):
    cluster = namespace["cluster"]
    project = namespace["project"]

    valid_endpoint = namespace["name_prefix"] + "-fluentd-aggregator" + \
        "." + namespace["ns"].name + ".svc.cluster.local:24224"
    print("fluentd aggregator endpoint:" + valid_endpoint)
    send_log_to_fluentd_aggregator(CATTLE_ClUSTER_LOGGING_FLUENTD_TEST, valid_endpoint, cluster.id, project.id, USER_TOKEN)
    send_log_to_fluentd_aggregator(CATTLE_PROJECT_LOGGING_FLUENTD_TEST, valid_endpoint, cluster.id, project.id, USER_TOKEN)

    bad_format_endpoint = "http://fluentd.com:9092"
    print("fluentd aggregator endpoint:" + bad_format_endpoint)
    send_log_to_fluentd_aggregator(CATTLE_ClUSTER_LOGGING_FLUENTD_TEST, bad_format_endpoint, cluster.id, project.id, USER_TOKEN, expected_status=500)
    send_log_to_fluentd_aggregator(CATTLE_PROJECT_LOGGING_FLUENTD_TEST, bad_format_endpoint, cluster.id, project.id, USER_TOKEN, expected_status=500)


def send_log_to_fluentd_aggregator(url, endpoint, clusterId, projectId, token, expected_status=204):
    headers = {'Authorization': 'Bearer ' + token}
    fluentdConfig = {
        "fluentServers": [
            {
                "endpoint": endpoint,
                "weight": 100
            }
        ],
        "enableTls": False,
        "compress": True
    }

    r = requests.post(url,
                      json={"fluentForwarderConfig": fluentdConfig,
                            "clusterId": clusterId,
                            "projectId": projectId},
                      verify=False, headers=headers)
    if len(r.content) != 0:
        print(r.content)
    assert r.status_code == expected_status


@pytest.fixture(scope='function')
def setup_fluentd_aggregator(request):
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    name = random_test_name("fluentd-aggregator")
    namespace["name_prefix"] = name
    app = p_client.create_app(name=name,
                              answers=fluentd_aggregator_answers,
                              targetNamespace=ns.name,
                              externalId=FLUENTD_AGGREGATOR_CATALOG_ID,
                              namespaceId=ns.id)
    wait_for_app_to_active(p_client, app.name)


fluentd_app_name = "rancher-logging"
fluentd_secret_name = "rancher-logging-fluentd"
endpoint_host = "rancher.com"
endpoint_port = "24224"
username = "user1"
password = "my_password"
shared_key = "my_shared_key"
weight = 100


def test_fluentd_target(request):
    cluster = namespace["cluster"]
    cluster_logging = create_cluster_logging(fluentd_target_without_ssl())
    request.addfinalizer(lambda: delete_cluster_logging(cluster_logging))
    wait_for_logging_app()

    # wait for config to sync
    time.sleep(120)

    config = get_fluentd_config("cluster.conf")
    assert fluentd_ssl_configure("cluster_" + namespace["cluster"].id) not in config
    assert fluentd_server_configure() in config

    ssl_config = fluentd_target_with_ssl()
    admin_client = namespace["admin_client"]
    admin_client.update_by_id_cluster_logging(id=cluster_logging.id,
                                              name=cluster_logging.name,
                                              clusterId=cluster.id,
                                              fluentForwarderConfig=ssl_config)
    # wait for config to sync
    time.sleep(60)
    config = get_fluentd_config("cluster.conf")
    assert fluentd_ssl_configure("cluster_" + namespace["cluster"].id) in config
    assert fluentd_server_configure() in config


def test_project_fluentd_target(request):
    project = namespace["project"]
    wrap_project_name = project.id.replace(':', '_')
    project_logging = create_project_logging(fluentd_target_without_ssl())
    request.addfinalizer(lambda: delete_project_logging(project_logging))
    wait_for_logging_app()

    # wait for config to sync
    time.sleep(60)
    config = get_fluentd_config("project.conf")
    assert fluentd_ssl_configure("project_" + wrap_project_name) not in config
    assert fluentd_server_configure() in config

    ssl_config = fluentd_target_with_ssl()
    admin_client = namespace["admin_client"]
    admin_client.update_by_id_project_logging(id=project_logging.id,
                                              name=project_logging.name,
                                              projectId=project.id,
                                              fluentForwarderConfig=ssl_config)
    # wait for config to sync
    time.sleep(60)
    config = get_fluentd_config("project.conf")
    assert fluentd_ssl_configure("project_" + wrap_project_name) in config
    assert fluentd_server_configure() in config


def wait_for_logging_app():
    sys_p_client = namespace["sys_p_client"]
    wait_for_app_to_active(sys_p_client, fluentd_app_name)


def fluentd_target_with_ssl():
    return {"certificate": "-----BEGIN CERTIFICATE-----\
                    ----END CERTIFICATE-----",
            "clientCert": "-----BEGIN CERTIFICATE-----\
                    ----END CERTIFICATE-----",
            "clientKey": "-----BEGIN PRIVATE KEY-----\
                    ----END PRIVATE KEY-----",
            "compress": True,
            "enableTls": True,
            "fluentServers": [
                {
                    "endpoint": endpoint_host + ":" + endpoint_port,
                    "username": username,
                    "password": password,
                    "sharedKey": shared_key,
                    "weight": weight
                }
            ],
            }


def fluentd_target_without_ssl():
    return {"compress": True,
            "enableTls": True,
            "fluentServers": [
                {
                    "endpoint": endpoint_host + ":" + endpoint_port,
                    "username": username,
                    "password": password,
                    "sharedKey": shared_key,
                    "weight": weight
                }
            ],
            }


def fluentd_ssl_configure(name):
    return f"""tls_cert_path /fluentd/etc/config/ssl/{name}_ca.pem
tls_client_cert_path /fluentd/etc/config/ssl/{name}_client-cert.pem
tls_client_private_key_path /fluentd/etc/config/ssl/{name}_client-key.pem"""


def fluentd_server_configure():
    return f"""<server>
host {endpoint_host}
port {endpoint_port}
shared_key {shared_key}
username  {username}
password  {password}
weight  {weight}
</server>"""


def get_system_project_client():
    cluster = namespace["cluster"]
    admin_client = namespace["admin_client"]
    projects = admin_client.list_project(name="System",
                                         clusterId=cluster.id).data
    assert len(projects) == 1
    project = projects[0]
    sys_p_client = get_project_client_for_token(project, USER_TOKEN)
    return sys_p_client


def get_namespaced_secret(name):
    sys_p_client = namespace["sys_p_client"]
    secres = sys_p_client.list_namespaced_secret(name=name)
    assert len(secres.data) == 1
    return secres.data[0]


def get_fluentd_config(key):
    secret = get_namespaced_secret(fluentd_secret_name)
    base64_cluster_conf = secret.data[key]
    tmp = base64.b64decode(base64_cluster_conf).decode("utf-8")
    return strip_whitespace(tmp)


def strip_whitespace(ws):
    new_str = []
    for s in ws.strip().splitlines(True):
        if s.strip():
            new_str.append(s.strip())
    return "\n".join(new_str)


def create_cluster_logging(config):
    cluster = namespace["cluster"]
    admin_client = namespace["admin_client"]
    name = random_test_name("fluentd")
    return admin_client.create_cluster_logging(name=name,
                                               clusterId=cluster.id,
                                               fluentForwarderConfig=config
                                               )


def delete_cluster_logging(cluster_logging):
    admin_client = namespace["admin_client"]
    admin_client.delete(cluster_logging)


def create_project_logging(config):
    project = namespace["project"]
    admin_client = namespace["admin_client"]
    name = random_test_name("fluentd")
    return admin_client.create_project_logging(name=name,
                                               projectId=project.id,
                                               fluentForwarderConfig=config
                                               )


def delete_project_logging(project_logging):
    admin_client = namespace["admin_client"]
    admin_client.delete(project_logging)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(USER_TOKEN, cluster,
                                  random_test_name("testlogging"))
    p_client = get_project_client_for_token(p, USER_TOKEN)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["admin_client"] = client
    namespace["sys_p_client"] = get_system_project_client()

    def fin():
        client = get_user_client()
        client.delete(namespace["project"])
    request.addfinalizer(fin)
