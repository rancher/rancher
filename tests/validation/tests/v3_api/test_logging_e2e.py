import time
import urllib

import pytest

from .common import CATTLE_TEST_URL
from .common import DEFAULT_TIMEOUT
from .common import USER_TOKEN
from .common import WebsocketLogParse
from .common import create_connection
from .common import create_kubeconfig
from .common import create_project_and_ns
from .common import get_project_client_for_token
from .common import get_user_client_and_cluster
from .common import random_test_name
from .common import wait_for_app_to_active

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None,
             "name_prefix": None, "admin_client": None, "sys_p_client": None,
             "pod": None}
fluentd_aggregator_answers = {"defaultImage": "true",
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

FLUENTD_AGGREGATOR_CATALOG_ID = "catalog://?catalog=library&template=fluentd-aggregator&version=0.3.1"


fluentd_app_name = "rancher-logging"
endpoint_port = "24224"
weight = 100


def test_fluentd_target_logs(setup_fluentd_aggregator, request):
    cluster_logging = create_cluster_logging(fluentd_target_without_ssl())
    request.addfinalizer(lambda: delete_logging(cluster_logging))
    wait_for_logging_app()

    # wait for config to sync
    time.sleep(90)
    validate_websocket_view_logs()


def test_project_fluentd_target_logs(setup_fluentd_aggregator, request):
    project_logging = create_project_logging(fluentd_target_without_ssl())
    request.addfinalizer(lambda: delete_logging(project_logging))
    wait_for_logging_app()

    # wait for config to sync
    # wait for project logs to start being forwarded
    time.sleep(90)
    validate_websocket_view_logs()


def wait_for_logging_app():
    sys_p_client = namespace["sys_p_client"]
    wait_for_app_to_active(sys_p_client, fluentd_app_name)


def fluentd_target_without_ssl():
    return {"compress": True,
            "enableTls": False,
            "sslVerify": False,
            "fluentServers": [
                {
                    "endpoint": namespace["hostname"]+ ":" + endpoint_port,
                    "weight": weight
                }
            ],
            }


def get_system_project_client():
    cluster = namespace["cluster"]
    admin_client = namespace["admin_client"]
    projects = admin_client.list_project(name="System",
                                         clusterId=cluster.id).data
    assert len(projects) == 1
    project = projects[0]
    sys_p_client = get_project_client_for_token(project, USER_TOKEN)
    return sys_p_client


def create_cluster_logging(config, json_parsing=False):
    cluster = namespace["cluster"]
    admin_client = namespace["admin_client"]
    name = random_test_name("fluentd")
    return admin_client.create_cluster_logging(name=name,
                                               clusterId=cluster.id,
                                               fluentForwarderConfig=config,
                                               enableJSONParsing=json_parsing,
                                               outputFlushInterval=5
                                               )


def create_project_logging(config, json_parsing=False):
    admin_client = namespace["admin_client"]
    cluster = namespace["cluster"]
    projects = admin_client.list_project(name="System",
                                         clusterId=cluster.id).data
    assert len(projects) == 1
    project = projects[0]
    name = random_test_name("project-fluentd")
    return admin_client.create_project_logging(name=name,
                                               projectId=project.id,
                                               fluentForwarderConfig=config,
                                               enableJSONParsing=json_parsing,
                                               outputFlushInterval=5
                                               )


def delete_logging(logging_project):
    admin_client = namespace["admin_client"]
    admin_client.delete(logging_project)


def validate_websocket_view_logs():
    url_base = 'wss://' + CATTLE_TEST_URL[8:] + \
               '/k8s/clusters/' + namespace["cluster"].id + \
               '/api/v1/namespaces/' + namespace["ns"].name + \
               '/pods/' + namespace["pod"].name + \
               '/log?container=' + namespace["pod"].containers[0].name
    params_dict = {
        "tailLines": 500,
        "follow": True,
        "timestamps": True,
        "previous": False,
    }
    params = urllib.parse.urlencode(params_dict, doseq=True,
                                    quote_via=urllib.parse.quote, safe='()')

    url = url_base + "&" + params
    wait_for_match(WebsocketLogParse(), url)


def wait_for_match(wslog, url, timeout=DEFAULT_TIMEOUT):
    start = time.time()
    found = False

    ws = create_connection(url, ["base64.binary.k8s.io"])
    assert ws.connected, "failed to build the websocket"
    wslog.start_thread(target=wslog.receiver, args=(ws, False))
    while not found:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for string to match in logs")
        time.sleep(1)
        print('shell command and output:\n' + wslog.last_message + '\n')
        if 'log_type' in wslog.last_message or '{"log"' in wslog.last_message:
            found = True
            wslog.last_message = ''
            break
    ws.close()
    assert found == True


@pytest.fixture(autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    project_name = random_test_name("testlogging")
    p, ns = create_project_and_ns(USER_TOKEN, cluster,
                                  project_name)
    p_client = get_project_client_for_token(p, USER_TOKEN)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["admin_client"] = client
    namespace["sys_p_client"] = get_system_project_client()

    def fin():
        client.delete(namespace["project"])
    request.addfinalizer(fin)


@pytest.fixture
def setup_fluentd_aggregator():
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
    namespace["hostname"] = namespace["name_prefix"] + \
                            "." + namespace["ns"].name + \
                            ".svc.cluster.local"

    wl = p_client.list_workload(name=name).data[0]
    pod = p_client.list_pod(workloadId=wl.id).data[0]
    namespace["pod"] = pod
