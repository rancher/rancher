from .common import get_global_admin_client_and_cluster, create_kubeconfig, \
    random_test_name, create_project_and_ns, get_project_client_for_token, \
    random_str, wait_for_condition, get_cluster_client_for_token, \
    create_connection, WebsocketLogParse, CATTLE_TEST_URL, USER_TOKEN
import pytest
import urllib
import time

namespace = {"p_client": None, "ns": None, "cluster": None,
             "project": None, "pipeline": None, "pipeline_run": None}
PIPELINE_TIMEOUT =  600
PIPELINE_REPO_URL = "https://github.com/rancher/pipeline-example-go.git"


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_global_admin_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(
        USER_TOKEN, cluster, random_test_name("testworkload"))
    p_client = get_project_client_for_token(p, USER_TOKEN)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p

    def fin():
        # project delete doesn't remove all ns in test, removing manually
        client = get_cluster_client_for_token(namespace["cluster"], USER_TOKEN)
        for ns_x in client.list_namespace(projectId=namespace["project"].id):
            client.delete(ns_x)
        client.delete(namespace["project"])
    request.addfinalizer(fin)


def test_pipeline():
    pipeline = create_example_pipeline()
    assert len(namespace["p_client"].list_pipeline(
        projectId=namespace["project"].id).data) > 0
    print("Created Pipeline, running example...")
    namespace["pipeline"] = pipeline
    namespace["pipeline_run"] = namespace["p_client"].action(obj=pipeline,
                                                             action_name='run',
                                                             branch="master")
    wait_for_condition(
        namespace["p_client"], pipeline, check_last_run_state("Success"),
        timeout=PIPELINE_TIMEOUT)
    assert len(pipeline_view_logs()) > 1
    print("Cleaning up...")
    namespace["p_client"].delete(pipeline)
    assert len(namespace["p_client"].list_pipeline(
        projectId=namespace["project"].id).data) == 0
    print("\nDeleted Pipeline")


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
    return namespace["p_client"].create_pipeline(
        name="test-" + random_str(),
        repositoryUrl=PIPELINE_REPO_URL,
        triggerWebhookPr=False,
        triggerWebhookPush=False,
        triggerWebhookTag=False)



def pipeline_view_logs():
    url_base = 'wss://' + CATTLE_TEST_URL[7:] + \
               '/v3/projects/' + namespace["project"].id + \
               '/pipelineExecutions/' + namespace["pipeline_run"].id + \
               '/log?'
    params_dict = {
        "stage": 1,
        "step": 0
    }
    params = urllib.parse.urlencode(params_dict, doseq=True,
                                    quote_via=urllib.parse.quote, safe='()')
    url = url_base + "&" + params
    print(url)
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
