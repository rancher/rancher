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

CATTLE_PROJECT_LOGGING_FLUENTD_DRY_RUN_TEST = \
    CATTLE_TEST_URL + "/v3/projectloggings?action=dryRun"


def test_dry_run_with_fluentd_configure():
    cluster = namespace["cluster"]
    project = namespace["project"]

    valid_content = """
    @type forward
    compress gzip
    <server>
        name rancher.com
        host 192.168.1.42
        port 24224
        weight  100
    </server>
    """

    custom_target_config = {
        "clientKey": "",
        "clientCert": "",
        "certificate": "",
        "content": valid_content
    }

    send_custom_target_config(CATTLE_PROJECT_LOGGING_FLUENTD_DRY_RUN_TEST, cluster.id, project.id, ADMIN_TOKEN, custom_target_config)
    send_custom_target_config(CATTLE_PROJECT_LOGGING_FLUENTD_DRY_RUN_TEST, cluster.id, project.id, ADMIN_TOKEN, custom_target_config)

    invalid_content = """
    @type invalid
    compress gzip
    <server>
        name rancher.com
        host 192.168.1.42
        port 24224
        weight  100
    </server>
    """
    custom_target_config["content"] = invalid_content
    send_custom_target_config(CATTLE_PROJECT_LOGGING_FLUENTD_DRY_RUN_TEST, cluster.id, project.id, ADMIN_TOKEN, custom_target_config, expected_status=500)
    send_custom_target_config(CATTLE_PROJECT_LOGGING_FLUENTD_DRY_RUN_TEST, cluster.id, project.id, ADMIN_TOKEN, custom_target_config, expected_status=500)


def send_custom_target_config(url, clusterId, projectId, token, custom_target_config, expected_status=204):
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.post(url,
                      json={"customTargetConfig": custom_target_config,
                            "clusterId": clusterId,
                            "projectId": projectId},
                      verify=False, headers=headers)
    if len(r.content) != 0:
        print(r.content)
    assert r.status_code == expected_status


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
