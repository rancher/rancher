from .common import *  # NOQA
from .test_secrets import (
    create_and_validate_workload_with_secret_as_env_variable, create_secret)
from .test_rke_cluster_provisioning import rke_config

secret_key = "RTczRjFDODMwQzAyMDVBREU4NDJBMUZFNDhCNzM5N0I="
SECRET_ENCRYPTION_CONFIG = {
    "customConfig": {
        "apiVersion": "apiserver.config.k8s.io/v1",
        "kind": "EncryptionConfiguration", "resources":
            [{"resources":
                ["secrets"], "providers":
                    [{"aescbc": {"keys":
                                 [{"name":
                                  "k-fw5hn", "secret":
                                   secret_key}]}},
                     {"identity": {}}]}]},
    "enabled": True, "type": "/v3/schemas/secretsEncryptionConfig"}

namespace = {"p_client": None, "ns": None,
             "cluster": None, "project": None}


def test_create_custom_cluster_with_encryption():
    client = None
    cluster = None
    aws_nodes = None
    try:
        client, cluster, aws_nodes = create_custom_cluster_with_encryption()
        create_and_validate_secrets(cluster)
    finally:
        # Cleans cluster
        cluster_cleanup(client, cluster, aws_nodes)


def create_custom_cluster_with_encryption():
    # Adds custom encryption to the RKE config
    custom_encryption_rke_config = rke_config.copy()
    custom_encryption_rke_config["services"]["kubeApi"].update(
                        {"secretsEncryptionConfig": SECRET_ENCRYPTION_CONFIG})
    print(custom_encryption_rke_config)

    # Creates custom cluster
    aws_nodes = \
        AmazonWebServices().create_multiple_nodes(
            3, random_test_name("jenkins-custom-secret-encryption"))
    node_roles = [["controlplane"], ["etcd"],
                  ["worker"]]
    client = get_user_client()
    cluster = client.create_cluster(
        name=random_name(),
        driver="rancherKubernetesEngine",
        rancherKubernetesEngineConfig=custom_encryption_rke_config)
    assert cluster.state == "provisioning"
    i = 0
    for aws_node in aws_nodes:
        docker_run_cmd = \
            get_custom_host_registration_cmd(
                client, cluster, node_roles[i], aws_node)
        aws_node.execute_command(docker_run_cmd)
        i += 1
    validate_cluster(client, cluster)
    create_kubeconfig(cluster)
    # Checks for key in encryption configuration
    # encryption.yaml is available only on the control plane node
    configuration_file = aws_nodes[0].execute_command(
        "sudo cat /etc/kubernetes/ssl/encryption.yaml")
    print(configuration_file)
    assert "aescbc" in configuration_file[0]
    assert "k-fw5hn" in configuration_file[0]
    assert secret_key in (
        configuration_file[0])
    return client, cluster, aws_nodes


def create_and_validate_secrets(cluster):
    # Validates the creation of secrets
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testsecret")
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)

    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["c_client"] = c_client

    p_client = namespace["p_client"]
    ns = namespace["ns"]
    # Value is base64 encoded
    value = base64.b64encode(b"valueall")
    keyvaluepair = {"testall": value.decode('utf-8')}
    cluster = namespace["cluster"]
    project = namespace["project"]
    c_client = namespace["c_client"]

    secret = create_secret(keyvaluepair, p_client=p_client, ns=ns)
    new_ns1 = create_ns(c_client, cluster, project)
    create_and_validate_workload_with_secret_as_env_variable(p_client,
                                                             secret,
                                                             new_ns1,
                                                             keyvaluepair)
