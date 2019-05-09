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
answers = {
    "defaultImage": "true",
    "image": "confluentinc/cp-kafka",
    "imageTag": "4.0.1-1",
    "zookeeper.image.repository": "confluentinc/cp-zookeeper",
    "zookeeper.image.tag": "4.1.1",
    "schema-registry.enabled": "false",
    "kafka-rest.enabled": "false",
    "replicas": "3",
    "persistence.enabled": "false",
    "zookeeper.persistence.enabled": "false",
    "kafka-topics-ui.enabled": "false",
    "kafka-topics-ui.ingress.enabled": "false"}

CATTLE_ClUSTER_LOGGING_KAFKA_TEST = \
    CATTLE_TEST_URL + "/v3/clusterloggings?action=test"
CATTLE_PROJECT_LOGGING_KAFKA_TEST = \
    CATTLE_TEST_URL + "/v3/projectloggings?action=test"
KAFKA_CATALOG_ID = "catalog://?catalog=library&template=kafka&version=0.7.3"


def test_send_log_to_kafka(setup_app):
    cluster = namespace["cluster"]
    project = namespace["project"]

    valid_endpoint = "http://" + namespace["name_prefix"] + "-kafka" + \
        "." + namespace["ns"].name + ".svc.cluster.local:9092"
    print("kafka endpoint:" + valid_endpoint)
    send_log_to_kafka(CATTLE_ClUSTER_LOGGING_KAFKA_TEST, valid_endpoint,
                      cluster.id, project.id, ADMIN_TOKEN)
    send_log_to_kafka(CATTLE_PROJECT_LOGGING_KAFKA_TEST, valid_endpoint,
                      cluster.id, project.id, ADMIN_TOKEN)

    unreachable_endpoint = "http://kafka.com:9092"
    print("kafka endpoint:" + unreachable_endpoint)
    send_log_to_kafka(CATTLE_ClUSTER_LOGGING_KAFKA_TEST, unreachable_endpoint,
                      cluster.id, project.id, ADMIN_TOKEN, expected_status=500)
    send_log_to_kafka(CATTLE_PROJECT_LOGGING_KAFKA_TEST, unreachable_endpoint,
                      cluster.id, project.id, ADMIN_TOKEN, expected_status=500)

    bad_format_endpoint = "kafka.com:9092"
    print("kafka endpoint:" + bad_format_endpoint)
    send_log_to_kafka(CATTLE_ClUSTER_LOGGING_KAFKA_TEST, bad_format_endpoint,
                      cluster.id, project.id, ADMIN_TOKEN, expected_status=500)
    send_log_to_kafka(CATTLE_PROJECT_LOGGING_KAFKA_TEST, bad_format_endpoint,
                      cluster.id, project.id, ADMIN_TOKEN, expected_status=500)


def send_log_to_kafka(url, endpoint, clusterId, projectId, token,
                      expected_status=204):
    headers = {'Authorization': 'Bearer ' + token}
    kafkaConfig = {
        "brokerEndpoints": [endpoint],
        "topic": "test",
    }

    r = requests.post(url,
                      json={"kafkaConfig": kafkaConfig,
                            "clusterId": clusterId,
                            "projectId": projectId},
                      verify=False, headers=headers)
    if len(r.content) != 0:
        print(r.content)
    assert r.status_code == expected_status


@pytest.fixture(scope='function')
def setup_app(request):
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    name = random_test_name("kafka")
    namespace["name_prefix"] = name
    app = p_client.create_app(name=name,
                              answers=answers,
                              targetNamespace=ns.name,
                              externalId=KAFKA_CATALOG_ID,
                              namespaceId=ns.id)
    wait_for_app_to_active(p_client, app.name)
    # wait for kafka broker to sync
    time.sleep(80)


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
