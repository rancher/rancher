import os
import time

from .conftest import *  # NOQA
from tests.rke.common import create_rke_cluster, delete_nodes


KUBE_CONFIG_PATH = os.environ.get(
    'KUBE_CONFIG_PATH', 'kube_config_cluster.yml')
CONFORMANCE_DONE = "no-exit was specified, sonobuoy is now blocking"


def extract_file_results_path(logs):
    log_lines = logs.splitlines()
    for line in log_lines:
        if "Results available at" in line:
            path_line = line.split(' ')
            abs_file_path = path_line[-1].replace('"', '')
            return abs_file_path
    else:
        raise Exception(
            "Unable to find test result file in logs: {0}".format(logs))


def delete_all_jobs(kubectl):
    namespaces = kubectl.list_namespaces()
    for namespace in namespaces:
        result = kubectl.delete_resourse("jobs", namespace=namespace, all=True)
        assert result.ok, "{}".format(result)


def run_conformance(kubectl, kube_config):
    kubectl.kube_config_path = kube_config
    delete_all_jobs(kubectl)
    kubectl.apply_conformance_tests()
    kubectl.wait_for_pod('sonobuoy', namespace='sonobuoy')
    conformance_tests_complete = False
    while not conformance_tests_complete:
        result = kubectl.logs(
            'sonobuoy', namespace='sonobuoy', tail=10)
        assert result.ok, (
            "Failed to read logs for conformance tests pod:\n{0}".format(
                result.stdout + result.stderr))
        if CONFORMANCE_DONE in result.stdout:
            break
        time.sleep(60)

    test_results_path = extract_file_results_path(result.stdout)
    result = kubectl.cp_from_pod('sonobuoy', 'sonobuoy', test_results_path,
                                 './conformance_results.tar.gz')
    assert result.ok, "{}".format(result)


def test_run_conformance_from_config(kubectl):
    """
        Runs conformance tests against an existing cluster
    """
    run_conformance(kubectl, KUBE_CONFIG_PATH)


def test_create_cluster_run_conformance(
        test_name, cloud_provider, rke_client, kubectl):
    """
        Creates an RKE cluster, runs conformance tests against that cluster
    """
    rke_template = 'cluster_install_config_1.yml.j2'
    nodes = cloud_provider.create_multiple_nodes(3, test_name)
    create_rke_cluster(rke_client, kubectl, nodes, rke_template)
    run_conformance(kubectl, rke_client.kube_config_path())
    delete_nodes(cloud_provider, nodes)
