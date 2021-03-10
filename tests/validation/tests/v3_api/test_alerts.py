import os

import pytest

from .common import get_user_client_and_cluster
from .common import execute_kubectl_cmd
from .common import create_kubeconfig
from .common import create_project_and_ns
from .common import get_user_client
from .common import wait_for
from .common import USER_TOKEN

ALERTS_PATH = os.path.join(
    os.path.dirname(os.path.realpath(__file__)), "resource/alerts"
)

namespace = {
    "client": None,
    "cluster": None,
    "project": None,
    "ns": None,
    "notifier": None,
    "alert_group": None,
    "alert_rule": None,
}

"""
Implementation of Minimum Alerting test
https://confluence.suse.com/display/EN/P0+Workflows+for+v1+charts

Uses cluster level approach for resources.
"""


def test_deploy_webhook():
    file_path = ALERTS_PATH + "/webhook.yaml"
    ns = namespace["ns"]
    print("The namespace name: " + ns.name)
    execute_kubectl_cmd("apply -f " + file_path + " -n " + ns.name, True)


def test_add_notifier():
    client = namespace["client"]
    cluster = namespace["cluster"]
    ns = namespace["ns"]
    notifier = client.create_notifier(
        name="webhook_test",
        clusterId=cluster.id,
        webhookConfig={
            "type": "webhookConfig",
            "url": "http://"
            + "webhook"
            + "."
            + ns.name
            + ".svc.cluster.local"
            + ":8080",
        },
    )
    # Debug
    print("The notifier id: " + notifier.id)
    print("The notifier url: " + notifier.webhookConfig.url)
    namespace["notifier"] = notifier
    assert len(notifier.name) > 0


def test_create_alertgroup():
    client = namespace["client"]
    cluster = namespace["cluster"]
    notifier = namespace["notifier"]

    alert_group = client.create_clusterAlertGroup(
        name="Test alert group",
        description="A description for test alert group",
        clusterId=cluster.id,
        recipients=[
            {
                "notifierType": "webhook",
                "notifierId": notifier.id,
                "recipient": notifier.webhookConfig.url,
            }
        ],
    )
    # Debug
    print("The alertgroup id: " + alert_group.id)
    print("The alertgroup recipient url: " + alert_group.recipients[0].recipient)
    namespace["alert_group"] = alert_group
    assert len(alert_group.name) > 0


def test_create_alertrule():
    client = namespace["client"]
    cluster = namespace["cluster"]
    alert_group = namespace["alert_group"]

    alert_rule = client.create_clusterAlertRule(
        name="test-watchpods",
        clusterId=cluster.id,
        groupId=alert_group.id,
        inherited="true",
        severity="critical",
        eventRule={"eventType": "Normal", "type": "eventRule", "resourceKind": "Pod"},
    )
    # Debug
    print("The alertrule id: " + alert_rule.id)
    print("The alertrule groupId: " + alert_rule.groupId)
    namespace["alert_rule"] = alert_rule
    assert len(alert_rule.name) > 0


# trigger alert event
def test_deploy_busybox():
    ns = namespace["ns"]

    # Execute nonexisting_command to make the pod restarting
    execute_kubectl_cmd(
        "run busybox --image=busybox -n " + ns.name + " -- nonexisting_command",
        False,
        False,
    )


def test_check_alert():
    ns = namespace["ns"]
    alert_rule = namespace["alert_rule"]

    def check_logs():
        logs = execute_kubectl_cmd(
            "logs -n " + ns.name + " -l app=webhook-serv", False, True
        )
        print(logs)
        alert_string = '"alert_name":"{}"'.format(alert_rule.name)
        return alert_string in logs.decode()

    try:
        wait_for(check_logs)
        assert True, "String found"
    except Exception as e:
        print("Error: {0}".format(e))
        assert False, "String not found"


@pytest.fixture(scope="module", autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    project, ns = create_project_and_ns(USER_TOKEN, cluster, "Test-alerts")
    namespace["client"] = client
    namespace["cluster"] = cluster
    namespace["ns"] = ns
    namespace["project"] = project

    def fin():
        client = get_user_client()
        notifier = namespace["notifier"]
        alert_group = namespace["alert_group"]
        alert_rule = namespace["alert_rule"]
        client.delete(project)
        client.delete(notifier)
        client.delete(alert_rule)
        client.delete(alert_group)

    request.addfinalizer(fin)
