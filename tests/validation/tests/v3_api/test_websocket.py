import base64
import pytest
import time
import urllib
from .common import CATTLE_TEST_URL
from .common import USER_TOKEN
from .common import TEST_IMAGE
from .common import create_kubeconfig
from .common import create_connection
from .common import create_project_and_ns
from .common import get_user_client_and_cluster
from .common import get_project_client_for_token
from .common import random_test_name
from .common import validate_workload
from .common import WebsocketLogParse

namespace = {"cluster": None, "shell_url": None, "pod": None, "ns": ""}


def test_websocket_launch_kubectl():
    ws = create_connection(namespace["shell_url"], ["base64.channel.k8s.io"])
    logparse = WebsocketLogParse()
    logparse.start_thread(target=logparse.receiver, args=(ws, True))

    cmd = "kubectl version"
    checks = ["Client Version", "Server Version"]
    validate_command_execution(ws, cmd, logparse, checks)
    logparse.last_message = ''

    cmd = "kubectl get ns -o name"
    checks = ["namespace/kube-system"]
    validate_command_execution(ws, cmd, logparse, checks)
    logparse.last_message = ''

    ws.close()


def test_websocket_exec_shell():
    url_base = 'wss://' + CATTLE_TEST_URL[8:] + \
               '/k8s/clusters/' + namespace["cluster"].id + \
               '/api/v1/namespaces/' + namespace["ns"] + \
               '/pods/' + namespace["pod"].name + \
               '/exec?container=' + namespace["pod"].containers[0].name
    params_dict = {
        "stdout": 1,
        "stdin": 1,
        "stderr": 1,
        "tty": 1,
        "command": [
            '/bin/sh',
            '-c',
            'TERM=xterm-256color; export TERM; [ -x /bin/bash ] && ([ -x '
            '/usr/bin/script ] && /usr/bin/script -q -c "/bin/bash" '
            '/dev/null || exec /bin/bash) || exec /bin/sh '
        ]
    }
    params = urllib.parse.urlencode(params_dict, doseq=True,
                                    quote_via=urllib.parse.quote, safe='()')
    url = url_base + "&" + params
    ws = create_connection(url, ["base64.channel.k8s.io"])
    logparse = WebsocketLogParse()
    logparse.start_thread(target=logparse.receiver, args=(ws, True))

    cmd = "ls"
    checks = ["bin", "boot", "dev"]
    validate_command_execution(ws, cmd, logparse, checks)
    logparse.last_message = ''

    ws.close()


def test_websocket_view_logs():
    url_base = 'wss://' + CATTLE_TEST_URL[8:] + \
               '/k8s/clusters/' + namespace["cluster"].id + \
               '/api/v1/namespaces/' + namespace["ns"] + \
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
    ws = create_connection(url, ["base64.binary.k8s.io"])
    logparse = WebsocketLogParse()
    logparse.start_thread(target=logparse.receiver, args=(ws, False))

    print('\noutput:\n' + logparse.last_message + '\n')
    assert 'websocket' in logparse.last_message, \
        "failed to view logs"
    logparse.last_message = ''

    ws.close()


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    project, ns = create_project_and_ns(USER_TOKEN,
                                        cluster,
                                        random_test_name("websocket"))
    p_client = get_project_client_for_token(project, USER_TOKEN)
    con = [{"name": random_test_name(),
            "image": TEST_IMAGE,
            "entrypoint": ["/bin/sh"],
            "command": ["-c",
                        "while true; do echo websocket; sleep 1s; done;"
                        ],
            }]
    wl = p_client.create_workload(name=random_test_name(),
                                        containers=con,
                                        namespaceId=ns.id)
    validate_workload(p_client, wl, "deployment", ns.name)
    pod = p_client.list_pod(workloadId=wl.id).data[0]
    namespace["ns"] = ns.name
    namespace["pod"] = pod
    namespace["cluster"] = cluster
    namespace["shell_url"] = cluster.get("links").get("shell")

    def fin():
        client.delete(project)

    request.addfinalizer(fin)


def send_a_command(ws_connection, command):
    cmd_enc = base64.b64encode(command.encode('utf-8')).decode('utf-8')
    ws_connection.send('0' + cmd_enc)
    # sends the command to the webSocket
    ws_connection.send('0DQ==')
    time.sleep(5)


def validate_command_execution(websocket, command, log_obj, checking):
    """
    validate that a command is send via the websocket
    and the response contains expected results
    :param websocket: the websocket object
    :param command:  the command to run
    :param log_obj: the logparse object to receive the message
    :param checking: the list of string to be checked in the response message
    :return:
    """

    send_a_command(websocket, command)
    print('\nshell command and output:\n' + log_obj.last_message + '\n')
    for i in checking:
        assert i in log_obj.last_message, "failed to run the command"
