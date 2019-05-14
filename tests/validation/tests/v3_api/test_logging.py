import pytest
import requests
import time
from .common import random_test_name
from .common import get_admin_client_and_cluster
from .common import create_project_and_ns
from .common import get_project_client_for_token
from .common import create_kubeconfig
from .common import get_admin_client
from .common import wait_for_app_to_active
from .common import CATTLE_TEST_URL
from .common import ADMIN_TOKEN

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None,
             "name_prefix": None}
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
    send_log_to_fluentd_aggregator(CATTLE_ClUSTER_LOGGING_FLUENTD_TEST, valid_endpoint, cluster.id, project.id, ADMIN_TOKEN)
    send_log_to_fluentd_aggregator(CATTLE_PROJECT_LOGGING_FLUENTD_TEST, valid_endpoint, cluster.id, project.id, ADMIN_TOKEN)

    bad_format_endpoint = "http://fluentd.com:9092"
    print("fluentd aggregator endpoint:" + bad_format_endpoint)
    send_log_to_fluentd_aggregator(CATTLE_ClUSTER_LOGGING_FLUENTD_TEST, bad_format_endpoint, cluster.id, project.id, ADMIN_TOKEN, expected_status=500)
    send_log_to_fluentd_aggregator(CATTLE_PROJECT_LOGGING_FLUENTD_TEST, bad_format_endpoint, cluster.id, project.id, ADMIN_TOKEN, expected_status=500)


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


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_admin_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(ADMIN_TOKEN, cluster,
                                  random_test_name("testlogging"))
    p_client = get_project_client_for_token(p, ADMIN_TOKEN)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p

    def fin():
        client = get_admin_client()
        client.delete(namespace["project"])
    request.addfinalizer(fin)
