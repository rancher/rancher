from jinja2 import Template
import random
import subprocess
import os


def generate_clsuter_config(dind_rke_node_num):
    # generate a radnom and kube_config file
    dind_random = random.randint(1, 100000)
    dind_cluster_config_file = "dind-" + str(dind_random) + ".yml"
    dind_kube_config_file = "kube_config_dind-" + str(dind_random) + ".yml"
    cluster_config_tmpl = get_rke_config_template()

    # generate nodes
    random_nodes = [
            "node-" +
            str(random.randint(10000, 99999))
            for x in range(dind_rke_node_num)]
    rke_config_template = Template(cluster_config_tmpl)
    rendered_tmpl = rke_config_template.render(
        random_nodes=random_nodes)

    # write config file on disk
    cluster_config_file = open(dind_cluster_config_file, "w")
    cluster_config_file.write(rendered_tmpl)
    cluster_config_file.close()
    return dind_cluster_config_file, dind_kube_config_file


def get_rke_config_template():
    dind_cluster_config_j2 = """
---
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
    subprocess.check_output("rke up --dind --config " +
                            cluster_config_file, shell=True)


def remove_cluster(cluster_config_file):
    subprocess.check_output("rke remove --force --dind --config " +
                            cluster_config_file, shell=True)


def import_cluster(admin_mc, kube_config_file, cluster_name):
    client = admin_mc.client
    imported_cluster = client.create_cluster(replace=True, name=cluster_name)
    reg_token = client.create_cluster_registration_token(
            clusterId=imported_cluster.id)
    insecure_command = reg_token.insecureCommand
    # run kubectl command
    os_env = os.environ.copy()
    os_env["KUBECONFIG"] = kube_config_file
    subprocess.check_output(insecure_command, env=os_env, shell=True)
