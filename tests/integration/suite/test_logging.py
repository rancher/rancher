import time
from .common import random_str

endpoint_host = "rancher.com"
endpoint_port = "24224"
tls_config = {
    "endpoint": endpoint_host + ":" + endpoint_port,
    "protocol": "tcp",
    "enableTls": True,
    "sslVerify": True,
    "certificate": "-----BEGIN CERTIFICATE-----\
                            ----END CERTIFICATE-----",
    "clientCert": "-----BEGIN CERTIFICATE-----\
                            ----END CERTIFICATE-----",
    "clientKey": "-----BEGIN PRIVATE KEY-----\
                            ----END PRIVATE KEY-----",
}


def test_project_graylog_config(admin_mc, admin_pc, remove_resource):
    name = random_str()
    config = {
        "endpoint": endpoint_host + ":" + endpoint_port,
        "protocol": "udp"
    }
    client = admin_mc.client
    project = admin_pc.project
    project_logging = client.create_project_logging(name=name,
                                                    projectId=project.id,
                                                    graylogConfig=config)
    remove_resource(project_logging)

    project_logging = wait_for_project_logging(client, project.id)

    assert project_logging.graylogConfig[
               'endpoint'] == endpoint_host + ":" + endpoint_port
    assert project_logging.graylogConfig['protocol'] == 'udp'

    client.update_by_id_project_logging(id=project_logging.id,
                                        name=project_logging.name,
                                        projectId=project.id,
                                        graylogConfig=tls_config)
    generated_config = client.list_project_logging(projectId=project.id).data[
        0].graylogConfig

    # test whether config is successfully updated
    assert generated_config["protocol"] == "tcp"
    assert generated_config["enableTls"] is True
    assert generated_config["sslVerify"] is True


def test_cluster_graylog_config(admin_mc, remove_resource):
    name = random_str()
    config = {
        "endpoint": endpoint_host + ":" + endpoint_port,
        "protocol": "udp"
    }
    client = admin_mc.client
    cluster_logging = client.create_cluster_logging(name=name,
                                                    clusterId='local',
                                                    graylogConfig=config)
    remove_resource(cluster_logging)

    cluster_logging = wait_for_cluster_logging(client, 'local')

    assert cluster_logging.graylogConfig[
               'endpoint'] == endpoint_host + ":" + endpoint_port
    assert cluster_logging.graylogConfig['protocol'] == 'udp'

    client.update_by_id_cluster_logging(id=cluster_logging.id,
                                        name=cluster_logging.name,
                                        clusterId='local',
                                        graylogConfig=tls_config)

    generated_config = client.list_cluster_logging(clusterId='local').data[
        0].graylogConfig

    # test whether config is successfully updated
    assert generated_config["protocol"] == "tcp"
    assert generated_config["enableTls"] is True
    assert generated_config["sslVerify"] is True


def wait_for_project_logging(client, project_id, timeout=30):
    start = time.time()
    interval = 0.5
    project_loggings = client.list_project_logging(projectId=project_id).data

    while len(project_loggings) == 0:
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for logging created')
        time.sleep(interval)
        interval *= 2
        project_loggings = client.list_project_logging(
            projectId=project_id).data

    return project_loggings[0]


def wait_for_cluster_logging(client, cluster_id, timeout=30):
    start = time.time()
    interval = 0.5
    cluster_loggings = client.list_cluster_logging(clusterId=cluster_id).data

    while len(cluster_loggings) == 0:
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for logging created')
        time.sleep(interval)
        interval *= 2
        cluster_loggings = client.list_cluster_logging(
            clusterId=cluster_id).data

    return cluster_loggings[0]
