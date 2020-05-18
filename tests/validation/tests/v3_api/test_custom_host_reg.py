from .test_auth import enable_ad, load_setup_data
from .common import *  # NOQA
import ast

AGENT_REG_CMD = os.environ.get('RANCHER_AGENT_REG_CMD', "")
HOST_COUNT = int(os.environ.get('RANCHER_HOST_COUNT', 1))
HOST_NAME = os.environ.get('RANCHER_HOST_NAME', "testsa")
RANCHER_SERVER_VERSION = os.environ.get('RANCHER_SERVER_VERSION',
                                        "master-head")
rke_config = {"authentication": {"type": "authnConfig", "strategy": "x509"},
              "ignoreDockerVersion": False,
              "network": {"type": "networkConfig", "plugin": "canal"},
              "type": "rancherKubernetesEngineConfig"
              }
AUTO_DEPLOY_CUSTOM_CLUSTER = ast.literal_eval(
    os.environ.get('RANCHER_AUTO_DEPLOY_CUSTOM_CLUSTER', "True"))
KEYPAIR_NAME_PREFIX = os.environ.get('RANCHER_KEYPAIR_NAME_PREFIX', "")
RANCHER_CLUSTER_NAME = os.environ.get('RANCHER_CLUSTER_NAME', "")
RANCHER_ELASTIC_SEARCH_ENDPOINT = os.environ.get(
    'RANCHER_ELASTIC_SEARCH_ENDPOINT', "")
K8S_VERSION = os.environ.get('RANCHER_K8S_VERSION', "")

def test_add_custom_host():
    aws_nodes = AmazonWebServices().create_multiple_nodes(
        HOST_COUNT, random_test_name("testsa"+HOST_NAME))
    if AGENT_REG_CMD != "":
        for aws_node in aws_nodes:
            additional_options = " --address " + aws_node.public_ip_address + \
                                 " --internal-address " + \
                                 aws_node.private_ip_address
            agent_cmd = AGENT_REG_CMD + additional_options
            aws_node.execute_command(agent_cmd)
            print("Nodes: " + aws_node.public_ip_address)


def test_delete_keypair():
    AmazonWebServices().delete_keypairs(KEYPAIR_NAME_PREFIX)


def test_deploy_rancher_server():
    RANCHER_SERVER_CMD = \
        'sudo docker run -d --name="rancher-server" ' \
        '--restart=unless-stopped -p 80:80 -p 443:443  ' \
        'rancher/rancher'
    RANCHER_SERVER_CMD += ":" + RANCHER_SERVER_VERSION
    aws_nodes = AmazonWebServices().create_multiple_nodes(
        1, random_test_name("testsa"+HOST_NAME))
    aws_nodes[0].execute_command(RANCHER_SERVER_CMD)
    time.sleep(120)
    RANCHER_SERVER_URL = "https://" + aws_nodes[0].public_ip_address
    print(RANCHER_SERVER_URL)
    wait_until_active(RANCHER_SERVER_URL, timeout=300)

    RANCHER_SET_DEBUG_CMD = \
        "sudo docker exec rancher-server loglevel --set debug"
    aws_nodes[0].execute_command(RANCHER_SET_DEBUG_CMD)

    token = set_url_password_token(RANCHER_SERVER_URL)
    admin_client = rancher.Client(url=RANCHER_SERVER_URL + "/v3",
                                  token=token, verify=False)

    if AUTH_PROVIDER == "activeDirectory":
        enable_url = RANCHER_SERVER_URL + "/v3/" + AUTH_PROVIDER + \
            "Configs/" + AUTH_PROVIDER.lower() + "?action=testAndApply"
        auth_admin_user = load_setup_data()["admin_user"]
        enable_ad(auth_admin_user, token, enable_url=enable_url,
                  password=AUTH_USER_PASSWORD, nested=NESTED_GROUP_ENABLED)

        auth_user_login_url = RANCHER_SERVER_URL + "/v3-public/" \
            + AUTH_PROVIDER + "Providers/" \
            + AUTH_PROVIDER.lower() + "?action=login"
        user_token = login_as_auth_user(load_setup_data()["standard_user"],
                                        AUTH_USER_PASSWORD,
                                        login_url=auth_user_login_url)["token"]
    else:
        AUTH_URL = \
            RANCHER_SERVER_URL + "/v3-public/localproviders/local?action=login"
        user, user_token = create_user(admin_client, AUTH_URL)

    env_details = "env.CATTLE_TEST_URL='" + RANCHER_SERVER_URL + "'\n"
    env_details += "env.ADMIN_TOKEN='" + token + "'\n"
    env_details += "env.USER_TOKEN='" + user_token + "'\n"

    if AUTO_DEPLOY_CUSTOM_CLUSTER:
        aws_nodes = \
            AmazonWebServices().create_multiple_nodes(
                5, random_test_name("testcustom"))
        node_roles = [["controlplane"], ["etcd"],
                      ["worker"], ["worker"], ["worker"]]
        client = rancher.Client(url=RANCHER_SERVER_URL+"/v3",
                                token=user_token, verify=False)
        if K8S_VERSION != "":
            rke_config["kubernetesVersion"] = K8S_VERSION
        print("the rke config for creating the cluster:")
        print(rke_config)
        cluster = client.create_cluster(
            name=random_name(),
            driver="rancherKubernetesEngine",
            rancherKubernetesEngineConfig=rke_config)
        assert cluster.state == "provisioning"
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
    AmazonWebServices().delete_nodes(aws_nodes, wait_for_deleted=True)


def test_cluster_enable_logging_elasticsearch():
    client = get_user_client()
    cluster = get_cluster_by_name(client, RANCHER_CLUSTER_NAME)
    cluster_name = cluster.name
    client.create_cluster_logging(name=random_test_name("elasticsearch"),
                                  clusterId=cluster.id,
                                  elasticsearchConfig={
                                      "dateFormat": "YYYY-MM-DD",
                                      "sslVerify": False,
                                      "sslVersion": "TLSv1_2",
                                      "indexPrefix": cluster_name,
                                      "endpoint":
                                          RANCHER_ELASTIC_SEARCH_ENDPOINT}
                                  )
    projects = client.list_project(name="System",
                                   clusterId=cluster.id).data
    assert len(projects) == 1
    project = projects[0]
    p_client = get_project_client_for_token(project, USER_TOKEN)
    wait_for_app_to_active(p_client, "rancher-logging")
