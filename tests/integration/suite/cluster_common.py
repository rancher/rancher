import subprocess
import os
import re
import yaml
from .common import random_str
from jinja2 import Template


def generate_cluster_config(request, dind_rke_node_num):
    # generate a random and kube_config file
    dind_name = 'dind-' + random_str()
    dind_cluster_config_file = dind_name + '.yml'
    dind_kube_config_file = 'kube_config_' + dind_name + '.yml'
    cluster_config_tmpl = get_rke_config_template()
    # generate nodes
    random_nodes = [
            'node-' +
            random_str()
            for x in range(dind_rke_node_num)]
    rke_config_template = Template(cluster_config_tmpl)
    rendered_tmpl = rke_config_template.render(
        random_nodes=random_nodes)
    # write config file on disk
    cluster_config_file = open(dind_cluster_config_file, 'w')
    cluster_config_file.write(rendered_tmpl)
    cluster_config_file.close()

    request.addfinalizer(lambda: cleanup_dind(
        dind_cluster_config_file,
        dind_name + '.rkestate'
    ))

    return \
        dind_name, \
        yaml.safe_load(rendered_tmpl), \
        dind_cluster_config_file, \
        dind_kube_config_file


def cleanup_dind(cluster_file, state_file):
    remove_cluster(cluster_file)
    os.remove(cluster_file)
    os.remove(state_file)


def get_rke_config_template():
    dind_cluster_config_j2 = """
---
authentication:
    strategy: "x509|webhook"
nodes:{% for node in random_nodes %}
  - address: {{ node }}
    user: docker
    role:
    - controlplane
    - worker
    - etcd{% endfor %}
"""
    return dind_cluster_config_j2


def create_cluster(cluster_config_file):
    raise Exception('cluster creation needs refactor')
    # attempt to resolve unknown random rke up errors
    for _ in range(3):
        try:
            return subprocess.check_output(
                'rke up --dind --config ' +
                cluster_config_file,
                stderr=subprocess.STDOUT, shell=True
            )
        except subprocess.CalledProcessError as err:
            print('RKE up error: ' + str(err.output))
    raise Exception('rke up failure')


def remove_cluster(cluster_config_file):
    try:
        return subprocess.check_output(
            'rke remove --force --dind --config ' +
            cluster_config_file,
            stderr=subprocess.STDOUT, shell=True
        )
    except subprocess.CalledProcessError as err:
        print('RKE down error: ' + str(err.output))
        raise err


def import_cluster(admin_mc, kube_config_file, cluster_name):
    client = admin_mc.client

    imported_cluster = client.create_cluster(
                            replace=True,
                            name=cluster_name,
                            localClusterAuthEndpoint={
                                'enabled': True,
                            },
                            rancherKubernetesEngineConfig={},
                        )
    reg_token = client.create_cluster_registration_token(
                    clusterId=imported_cluster.id
                )

    # modify import command to add auth image
    match = r'\.yaml \|'
    replace = '.yaml?authImage=fixed |'
    insecure_command = re.sub(match, replace, reg_token.insecureCommand)

    # run kubectl command
    os_env = os.environ.copy()
    os_env['KUBECONFIG'] = kube_config_file
    subprocess.check_output(insecure_command, env=os_env, shell=True)
    return imported_cluster
