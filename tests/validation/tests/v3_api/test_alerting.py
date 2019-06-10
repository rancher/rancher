import pytest
import requests
import time
import os
import yaml
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
namespace = {"admin_client": None, "p_client": None, "ns": None, "cluster": None, "project": None, "pod": None,
             "slack_notifier": None, "wechat_notifier": None, "pagerduty_notifier": None, "webhook_notifier": None,
             "cluster_alert_group": None, "project_alert_group": None,
             "cluster_alert_rule": None, "project_alert_rule": None}

CATTLE_ClUSTER_NOTIFIER_TEST = \
    CATTLE_TEST_URL + "/v3/notifiers?action=send"

ONLINE_PROXY = os.environ.get('ONLINE_PROXY', "http://www.angolatelecom.com:8080")
WECHAT_AGENT = os.environ.get('NOTIFIER_WECHAT_AGENT', "None")
WECHAT_CORP = os.environ.get('NOTIFIER_WECHAT_CORP', "None")
WECHAT_SECRET = os.environ.get('NOTIFIER_WECHAT_SECRET', "None")
WECHAT_RECIPIENT = os.environ.get('NOTIFIER_WECHAT_RECIPIENT', "None")
SLACK_RECIPIENT = os.environ.get('NOTIFIER_SLACK_RECIPIENT', "None")
WEBHOOK_URL = os.environ.get('NOTIFIER_WEBHOOK_URL', "https://webhook.site/1dd8c55c-8628-4044-938f-42f0c73993c5")
PAGERDUTY_SERVICE_KEY = os.environ.get('NOTIFIER_PAGERDUTY_SERVICE_KEY', "None")

ALERT_TEST_IMAGE = os.environ.get('ALERT_TEST_IMAGE', "shashanktyagi/testing_restarts:v1")


def test_send_message():
    cluster = namespace["cluster"]

    # # webhook
    args = get_webhook_config(cluster.id)
    send_test_msg(CATTLE_ClUSTER_NOTIFIER_TEST, ADMIN_TOKEN, args)

    # slack
    args = get_slack_config(cluster.id)
    send_test_msg(CATTLE_ClUSTER_NOTIFIER_TEST, ADMIN_TOKEN, args)

    # # wechat
    args = get_wechat_config(cluster.id)
    send_test_msg(CATTLE_ClUSTER_NOTIFIER_TEST, ADMIN_TOKEN, args)

    # # pageduty
    args = get_pageduty_config(cluster.id)
    send_test_msg(CATTLE_ClUSTER_NOTIFIER_TEST, ADMIN_TOKEN, args)


def test_generated_alert_configure(setup):
    # wait for configure to sync
    time.sleep(60)
    cluster = namespace["cluster"]
    url = CATTLE_TEST_URL + "/k8s/clusters/" + cluster.id + "/api/v1/namespaces/cattle-prometheus/services/http:access-alertmanager:80/proxy/api/v2/status"
    alert_status = get_alert_status(url, ADMIN_TOKEN)
    config_original = alert_status["config"]["original"]
    result = yaml.load(config_original)

    receivers = result["receivers"]
    assert len(receivers) > 0
    cluster_alert_group = namespace["cluster_alert_group"]
    project_alert_group = namespace["project_alert_group"]
    for r in receivers:
        if r["name"] == cluster_alert_group.id:
            assert len(r["slack_configs"]) > 0
            r["slack_configs"][0]["http_config"]["proxy_url"] = ONLINE_PROXY
            assert len(r["slack_configs"]) > 0
            r["wechat_configs"][0]["http_config"]["proxy_url"] = ONLINE_PROXY
            assert len(r["slack_configs"]) > 0
            r["webhook_configs"][0]["http_config"]["proxy_url"] = ONLINE_PROXY
            assert len(r["slack_configs"]) > 0
            r["pagerduty_configs"][0]["http_config"]["proxy_url"] = ONLINE_PROXY

        if r["name"] == project_alert_group.id:
            assert len(r["slack_configs"]) > 0
            r["slack_configs"][0]["http_config"]["proxy_url"] = ONLINE_PROXY
            assert len(r["slack_configs"]) > 0
            r["wechat_configs"][0]["http_config"]["proxy_url"] = ONLINE_PROXY
            assert len(r["slack_configs"]) > 0
            r["webhook_configs"][0]["http_config"]["proxy_url"] = ONLINE_PROXY
            assert len(r["slack_configs"]) > 0
            r["pagerduty_configs"][0]["http_config"]["proxy_url"] = ONLINE_PROXY


def create_auto_restart_workload(p_client, ns):
    con = [{"name": "test1",
            "image": ALERT_TEST_IMAGE}]
    wl_name = random_test_name("auto-restart-workload")
    wl = p_client.create_workload(name=wl_name,
                                  containers=con,
                                  namespaceId=ns)
    wait_for_wl_to_active(p_client, wl, timeout=90)
    wl = p_client.reload(wl)

    pods = p_client.list_pod(workloadId=wl.id).data
    assert len(pods) == 1
    namespace["pod"] = pods[0]


def create_project_alert(admin_client, project_id):
    slack_recipier, wechat_recipier, pageduty_recipier, webhook_recipier = get_recipiers()
    pod = namespace["pod"]
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
                                                          recipients=[slack_recipier, wechat_recipier, pageduty_recipier, webhook_recipier])

    alert_rule = admin_client.create_project_alert_rule(name=name,
                                                        projectId=project_id,
                                                        groupId=alert_group.id,
                                                        groupIntervalSeconds=180,
                                                        groupWaitSeconds=30,
                                                        inherited=True,
                                                        repeatIntervalSeconds=3600,
                                                        podRule=pod_rule,
                                                        severity="critical")
    namespace["project_alert_group"] = alert_group
    namespace["project_alert_rule"] = alert_rule


def create_cluster_alert(admin_client, cluster_id):
    slack_recipier, wechat_recipier, pageduty_recipier, webhook_recipier = get_recipiers()
    event_rule = {"eventType": "Normal",
                  "resourceKind": "Deployment",
                  "type": "/v3/schemas/eventRule"}
    name = random_test_name("pod_restart")

    alert_group = admin_client.create_cluster_alert_group(name=name,
                                                          clusterId=cluster_id,
                                                          groupIntervalSeconds=180,
                                                          groupWaitSeconds=30,
                                                          repeatIntervalSeconds=3600,
                                                          recipients=[slack_recipier, wechat_recipier, pageduty_recipier, webhook_recipier])

    alert_rule = admin_client.create_cluster_alert_rule(name=name,
                                                        clusterId=cluster_id,
                                                        groupId=alert_group.id,
                                                        groupIntervalSeconds=180,
                                                        groupWaitSeconds=30,
                                                        inherited=True,
                                                        repeatIntervalSeconds=3600,
                                                        eventRule=event_rule,
                                                        severity="critical")
    namespace["cluster_alert_group"] = alert_group
    namespace["cluster_alert_rule"] = alert_rule


def get_recipiers():
    slack_recipier = {"notifierId": namespace["slack_notifier"].id,
                      "notifierType": "slack"}
    wechat_recipier = {"notifierId": namespace["wechat_notifier"].id,
                       "notifierType": "wechat"}
    pageduty_recipier = {"notifierId": namespace["pagerduty_notifier"].id,
                         "notifierType": "pagerduty"}
    webhook_recipier = {"notifierId": namespace["webhook_notifier"].id,
                        "notifierType": "webhook"}
    return slack_recipier, wechat_recipier, pageduty_recipier, webhook_recipier


def get_alert_status(url, token, expected_status=200):
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.get(url, verify=False, headers=headers)
    assert r.status_code == expected_status
    res = r.json()
    return res


def send_test_msg(url, token, args, expected_status=200):
    headers = {'Authorization': 'Bearer ' + token}
    args["message"] = "rancher test message"
    r = requests.post(url,
                      json=args,
                      verify=False, headers=headers)
    if len(r.content) != 0:
        print(r.content)
    assert r.status_code == expected_status


def create_notifiers(admin_client, cluster_id):
    namespace["slack_notifier"] = create_slack_notifier(admin_client, cluster_id)
    namespace["wechat_notifier"] = create_wechat_notifier(admin_client, cluster_id)
    namespace["pagerduty_notifier"] = create_pageduty_notifier(admin_client, cluster_id)
    namespace["webhook_notifier"] = create_webhook_notifier(admin_client, cluster_id)


def get_slack_config(cluster_id):
    name = random_test_name("slack")
    config = {
        "name": name,
        "clusterId": cluster_id,
        "slackConfig": {
            "defaultRecipient": SLACK_RECIPIENT,
            "proxyUrl": ONLINE_PROXY,
            "type": "/v3/schemas/slackConfig",
            "url": "https://slack.com/api/api.test"
        }
    }
    return config


def get_webhook_config(cluster_id):
    name = random_test_name("webhook")
    config = {
        "name": name,
        "clusterId": cluster_id,
        "webhookConfig": {
            "proxyUrl": ONLINE_PROXY,
            "type": "/v3/schemas/webhookConfig",
            "url": WEBHOOK_URL
        }
    }
    return config


def get_pageduty_config(cluster_id):
    name = random_test_name("pageduty")
    config = {
        "name": name,
        "clusterId": cluster_id,
        "pagerdutyConfig": {
            "proxyUrl": ONLINE_PROXY,
            "serviceKey": PAGERDUTY_SERVICE_KEY,
            "type": "/v3/schemas/pagerdutyConfig",
        }
    }
    return config


def get_wechat_config(cluster_id):
    name = random_test_name("wechat")
    config = {
        "name": name,
        "clusterId": cluster_id,
        "wechatConfig": {
            "agent": WECHAT_AGENT,
            "corp": WECHAT_CORP,
            "secret": WECHAT_SECRET,
            "defaultRecipient": WECHAT_RECIPIENT,
            "proxyUrl": ONLINE_PROXY,
            "recipientType": "user",
            "type": "/v3/schemas/wechatConfig"
        }
    }
    return config


def create_slack_notifier(admin_client, cluster_id):
    config = get_slack_config(cluster_id)
    return create_notifier(admin_client, config)


def create_wechat_notifier(admin_client, cluster_id):
    config = get_wechat_config(cluster_id)
    return create_notifier(admin_client, config)


def create_pageduty_notifier(admin_client, cluster_id):
    config = get_pageduty_config(cluster_id)
    return create_notifier(admin_client, config)


def create_webhook_notifier(admin_client, cluster_id):
    config = get_webhook_config(cluster_id)
    return create_notifier(admin_client, config)


def create_notifier(admin_client, args):
    notifier = admin_client.create_notifier(**args)
    return notifier


@pytest.fixture(scope='function')
def setup(request):
    admin_client = namespace["admin_client"]
    p_client = namespace["p_client"]
    cluster = namespace["cluster"]
    project = namespace["project"]
    ns = namespace["ns"]

    create_auto_restart_workload(p_client, ns.id)
    create_notifiers(admin_client, cluster.id)
    create_project_alert(admin_client, project.id)
    create_cluster_alert(admin_client, cluster.id)

    def fin():
        client = get_admin_client()
        client.delete(namespace["cluster_alert_group"])
        client.delete(namespace["cluster_alert_rule"])
        client.delete(namespace["project_alert_group"])
        client.delete(namespace["project_alert_rule"])

        client.delete(namespace["slack_notifier"])
        client.delete(namespace["wechat_notifier"])
        client.delete(namespace["pagerduty_notifier"])
        client.delete(namespace["webhook_notifier"])
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
