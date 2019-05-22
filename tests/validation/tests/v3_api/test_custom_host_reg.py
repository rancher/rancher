from .common import *  # NOQA
import ast

AGENT_REG_CMD = os.environ.get('RANCHER_AGENT_REG_CMD', "")
HOST_COUNT = int(os.environ.get('RANCHER_HOST_COUNT', 1))
HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "testsa")
RANCHER_VERSION = os.environ.get('RANCHER_SERVER_VERSION', "master")
RANCHER_IMAGE = os.environ.get('RANCHER_SERVER_IMAGE', "rancher/rancher")
RANCHER_ENV = os.environ.get('RANCHER_SERVER_ENV', "")
ADMIN_PASSWORD = os.environ.get('ADMIN_PASSWORD', "None")
rke_config = {"authentication": {"type": "authnConfig", "strategy": "x509"},
              "ignoreDockerVersion": False,
              "network": {"type": "networkConfig", "plugin": "canal"},
              "type": "rancherKubernetesEngineConfig"
              }
AUTO_DEPLOY_CUSTOM_CLUSTER = ast.literal_eval(
    os.environ.get('RANCHER_AUTO_DEPLOY_CUSTOM_CLUSTER', "True"))
KEYPAIR_NAME_PREFIX = os.environ.get('RANCHER_KEYPAIR_NAME_PREFIX', "")


def test_add_custom_host():
    aws_nodes = AmazonWebServices().create_multiple_nodes(
        HOST_COUNT, random_test_name("testsa"+HOST_NAME))
    if AGENT_REG_CMD != "":
        for aws_node in aws_nodes:
            aws_node.execute_command(AGENT_REG_CMD)


def test_delete_keypair():
    AmazonWebServices().delete_keypairs(KEYPAIR_NAME_PREFIX)


def test_deploy_rancher_server():
    assert ADMIN_PASSWORD != "None", "Need to set admin passsword"
    RANCHER_SERVER_CMD = \
        'docker run -d --name="rancher-server" ' \
        '--restart=unless-stopped -p 80:80 -p 443:443  '
    if len(RANCHER_ENV) > 0:
        RANCHER_SERVER_CMD += " -e " + RANCHER_ENV + " "
    RANCHER_SERVER_CMD += RANCHER_IMAGE + ":" + RANCHER_VERSION
    aws_nodes = AmazonWebServices().create_multiple_nodes(
        1, random_test_name("testsa"+HOST_NAME))
    aws_nodes[0].execute_command(RANCHER_SERVER_CMD)
    time.sleep(120)
    RANCHER_SERVER_URL = "https://" + aws_nodes[0].public_ip_address
    print(RANCHER_SERVER_URL)
    wait_until_active(RANCHER_SERVER_URL)

    RANCHER_SET_DEBUG_CMD = "docker exec rancher-server loglevel --set debug"
    aws_nodes[0].execute_command(RANCHER_SET_DEBUG_CMD)

    token = get_admin_token(RANCHER_SERVER_URL)
    env_details = "env.CATTLE_TEST_URL='" + RANCHER_SERVER_URL + "'\n"
    env_details += "env.ADMIN_TOKEN='" + token + "'\n"

    if AUTO_DEPLOY_CUSTOM_CLUSTER:
        aws_nodes = \
            AmazonWebServices().create_multiple_nodes(
                5, random_test_name("testcustom"))
        node_roles = [["controlplane"], ["etcd"],
                      ["worker"], ["worker"], ["worker"]]
        client = rancher.Client(url=RANCHER_SERVER_URL+"/v3",
                                token=token, verify=False)
        cluster = client.create_cluster(
            name=random_name(),
            driver="rancherKubernetesEngine",
            rancherKubernetesEngineConfig=rke_config)
        assert cluster.state == "active"
        i = 0
        for aws_node in aws_nodes:
            docker_run_cmd = \
                get_custom_host_registration_cmd(
                    client, cluster, node_roles[i], aws_node)
            aws_node.execute_command(docker_run_cmd)
            i += 1
        validate_cluster_state(client, cluster)
        env_details += "env.CLUSTER_NAME='" + cluster.name + "'\n"
    create_config_file(env_details)


def test_delete_rancher_server():
    client = get_admin_client()
    clusters = client.list_cluster().data
    for cluster in clusters:
        delete_cluster(client, cluster)
    clusters = client.list_cluster().data
    start = time.time()
    while len(clusters) > 0:
        time.sleep(30)
        clusters = client.list_cluster().data
        if time.time() - start > MACHINE_TIMEOUT:
            exceptionMsg = 'Timeout waiting for clusters to be removed'
            raise Exception(exceptionMsg)
    ip_address = CATTLE_TEST_URL[8:]
    print("Ip Address:" + ip_address)
    filters = [
        {'Name': 'network-interface.addresses.association.public-ip',
         'Values': [ip_address]}]
    aws_nodes = AmazonWebServices().get_nodes(filters)
    assert len(aws_nodes) == 1
    AmazonWebServices().delete_nodes(aws_nodes)


def get_admin_token(RANCHER_SERVER_URL):
    """Returns a ManagementContext for the default global admin user."""
    CATTLE_AUTH_URL = \
        RANCHER_SERVER_URL + "/v3-public/localproviders/local?action=login"
    r = requests.post(CATTLE_AUTH_URL, json={
        'username': 'admin',
        'password': 'admin',
        'responseType': 'json',
    }, verify=False)
    print(r.json())
    token = r.json()['token']
    print(token)
    # Change admin password
    client = rancher.Client(url=RANCHER_SERVER_URL+"/v3",
                            token=token, verify=False)
    admin_user = client.list_user(username="admin").data
    admin_user[0].setpassword(newPassword=ADMIN_PASSWORD)

    # Set server-url settings
    serverurl = client.list_setting(name="server-url").data
    client.update(serverurl[0], value=RANCHER_SERVER_URL)
    return token


def wait_until_active(rancher_url, timeout=120):
    start = time.time()
    while check_for_no_access(rancher_url):
        time.sleep(.5)
        print("No access yet")
        if time.time() - start > timeout:
            raise Exception('Timed out waiting for Rancher server '
                            'to become active')
    return


def check_for_no_access(rancher_url):
    try:
        requests.get(rancher_url, verify=False)
        return False
    except requests.ConnectionError:
        return True
