import pytest
import time
import urllib
import re

from .common import CATTLE_TEST_URL
from .common import USER_TOKEN
from .common import DNS_REGEX
from .common import create_connection
from .common import create_kubeconfig
from .common import create_project_and_ns
from .common import get_cluster_client_for_token
from .common import get_global_admin_client_and_cluster
from .common import get_project_client_for_token
from .common import random_str
from .common import random_test_name
from .common import wait_for_condition
from .common import WebsocketLogParse

pipeline_details = {"p_client": None, "ns": None, "cluster": None,
             "project": None, "pipeline": None, "pipeline_run": None}
PIPELINE_TIMEOUT = 600
PIPELINE_REPO_URL = "https://github.com/rancher/pipeline-example-go.git"


def test_pipeline():
    pipeline = create_example_pipeline()
    assert len(pipeline_details["p_client"].list_pipeline(
        projectId=pipeline_details["project"].id).data) > 0
    print("Created Pipeline, running example...")
    pipeline_details["pipeline"] = pipeline
    pipeline_details["pipeline_run"] = pipeline_details["p_client"].action(
                                                             obj=pipeline,
                                                             action_name='run',
                                                             branch="master")
    wait_for_condition(
        pipeline_details["p_client"], pipeline, 
        check_last_run_state("Success"), timeout=PIPELINE_TIMEOUT)
    assert len(pipeline_view_logs()) > 1
    print("Cleaning up...")
    pipeline_details["p_client"].delete(pipeline)
    assert len(pipeline_details["p_client"].list_pipeline(
        projectId=pipeline_details["project"].id).data) == 0
    print("\nDeleted Pipeline")


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_global_admin_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(
        USER_TOKEN, cluster, random_test_name("testworkload"))
    p_client = get_project_client_for_token(p, USER_TOKEN)
    pipeline_details["p_client"] = p_client
    pipeline_details["ns"] = ns
    pipeline_details["cluster"] = cluster
    pipeline_details["project"] = p

    def fin():
        # project delete doesn't remove all ns in test, removing manually
        client = get_cluster_client_for_token(pipeline_details["cluster"], 
                                              USER_TOKEN)
        for ns_x in client.list_namespace(
                        projectId=pipeline_details["project"].id):
            client.delete(ns_x)
        client.delete(pipeline_details["project"])
    request.addfinalizer(fin)


def check_last_run_state(status):
    def _find_condition(resource):
        if not hasattr(resource, "lastRunState"):
            return False

        if resource.lastRunState is None:
            return False

        if resource.lastRunState == status:
            return True
        if resource.lastRunState == "Failed":
            return False
        return False
    return _find_condition


def create_example_pipeline():
    return pipeline_details["p_client"].create_pipeline(
        name="test-" + random_str(),
        repositoryUrl=PIPELINE_REPO_URL,
        triggerWebhookPr=False,
        triggerWebhookPush=False,
        triggerWebhookTag=False)


def pipeline_view_logs():
    # using a regex to get the dns from the CATTLE_TEST_URL
    search_result = re.search(DNS_REGEX, CATTLE_TEST_URL)
    dns = search_result.group(2)

    url_base = 'wss://' + dns + \
               '/v3/projects/' + pipeline_details["project"].id + \
               '/pipelineExecutions/' + pipeline_details["pipeline_run"].id + \
               '/log?'
    params_dict = {
        "stage": 1,
        "step": 0
    }
    params = urllib.parse.urlencode(params_dict, doseq=True,
                                    quote_via=urllib.parse.quote, safe='()')
    url = url_base + "&" + params
    ws = create_connection(url, None)
    logparse = WebsocketLogParse()
    logparse.start_thread(target=logparse.receiver, args=(ws, False, False))
    # wait on thread to report any logs.
    while len(logparse.last_message) < 1:
        time.sleep(2)
    logs = '\noutput:\n' + logparse.last_message + '\n'
    print(logs)
    # + is on every line of any given log
    assert '+' in logparse.last_message, \
        "failed to view logs"
    logparse.last_message = ''
    ws.close()
    return logs
