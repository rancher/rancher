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
from .common import wait_for_wl_to_active

default_group_interval_seconds = 180
default_group_wait_seconds = 30
default_repeat_interval_seconds = 3600
namespace = {"admin_client": None, "p_client": None, "ns": None, "cluster": None, "project": None,
             "pod": None, "notifier": None, "pod_restart_group": None, "pod_restart_rule": None}

def test_pod_restart_alert(create_webhook_notifier):
    admin_client = namespace["admin_client"]
    p_client = namespace["p_client"]
    cluster = namespace["cluster"]
    project = namespace["project"]
    ns = namespace["ns"]

    create_auto_restart_workload(p_client, ns.id)
    create_project_alert_pod_restart(admin_client, project.id)

    # wait for configure to sync and alert be trigger
    time.sleep(180)

    alert_group = namespace["pod_restart_group"]
    alert_rule = namespace["pod_restart_rule"]
    url = CATTLE_TEST_URL + "/k8s/clusters/" + cluster.id + "/api/v1/namespaces/cattle-prometheus/services/http:access-alertmanager:80/proxy/api/v1/alerts"
    alert_list = get_alert_list(url, ADMIN_TOKEN)
    for a in alert_list:
        if a["labels"]["alert_type"] == "podRestarts" and a["labels"]["rule_id"] == alert_group.id + "_" + alert_rule.id.split(':')[1]:
            return

    raise AssertionError("Couldn't get pod restart alert")


def create_auto_restart_workload(p_client, ns):
    con = [{"name": "test1",
            "image": "shashanktyagi/testing_restarts:v1"}]
    wl_name = random_test_name("auto-restart-workload")
    wl = p_client.create_workload(name=wl_name,
                                  containers=con,
                                  namespaceId=ns)
    wait_for_wl_to_active(p_client, wl, timeout=90)
    wl = p_client.reload(wl)

    pods = p_client.list_pod(workloadId=wl.id).data
    assert len(pods) == 1
    namespace["pod"] = pods[0]


def create_project_alert_pod_restart(admin_client, project_id):
    notifier = namespace["notifier"]
    pod = namespace["pod"]
    recipier = {"notifierId": notifier.id,
                "notifierType": "webhook",
                "recipient": "https://webhook.site/1dd8c55c-8628-4044-938f-42f0c73993c5"}
    pod_rule = {"condition": "restarts",
                "podId": pod.id,
                "restartIntervalSeconds": 180,
                "restartTimes": 1}
    name = random_test_name("pod_restart")

    alert_group = admin_client.create_project_alert_group(name=name,
                                                          projectId=project_id,
                                                          groupIntervalSeconds=180,
                                                          groupWaitSeconds=30,
                                                          repeatIntervalSeconds=3600,
                                                          recipients=[recipier])

    alert_rule = admin_client.create_project_alert_rule(name=name,
                                                        projectId=project_id,
                                                        groupId=alert_group.id,
                                                        groupIntervalSeconds=180,
                                                        groupWaitSeconds=30,
                                                        inherited=True,
                                                        repeatIntervalSeconds=3600,
                                                        podRule=pod_rule,
                                                        severity="critical",
                                                        recipients=[recipier])
    namespace["pod_restart_group"] = alert_group
    namespace["pod_restart_rule"] = alert_rule

def get_alert_list(url, token, expected_status=200):
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.get(url, verify=False, headers=headers)
    assert r.status_code == expected_status
    res = r.json()
    assert res["status"] == "success"
    assert len(res["data"]) > 0
    return res["data"]


@pytest.fixture(scope='function')
def create_webhook_notifier(request):
    admin_client = namespace["admin_client"]
    cluster = namespace["cluster"]
    name = random_test_name("webhook")
    webhook_config = {"type": "webhookConfig",
                      "url": "https://webhook.site/1dd8c55c-8628-4044-938f-42f0c73993c5"}
    notifier = admin_client.create_notifier(name=name,
                                            clusterId=cluster.id,
                                            webhookConfig=webhook_config)
    namespace["notifier"] = notifier

    def fin():
        client = get_admin_client()
        client.delete(namespace["notifier"])
    request.addfinalizer(fin)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_admin_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(ADMIN_TOKEN, cluster,
                                  random_test_name("testalerting"))
    p_client = get_project_client_for_token(p, ADMIN_TOKEN)
    namespace["admin_client"] = client
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p

    def fin():
        client = get_admin_client()
        client.delete(namespace["project"])
    request.addfinalizer(fin)
