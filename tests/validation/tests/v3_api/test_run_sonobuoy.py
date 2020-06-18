import base64
import os
import time
from .common import *

RANCHER_SONOBUOY_VERSION = os.environ.get("RANCHER_SONOBUOY_VERSION", "0.18.2")
RANCHER_K8S_VERSION = os.environ.get("RANCHER_K8S_VERSION", "v1.18.2")
RANCHER_SONOBUOY_MODE = os.environ.get("RANCHER_SONOBUOY_MODE",
                                       "certified-conformance")
RANCHER_KUBECONFIG = os.environ.get("RANCHER_KUBECONFIG")
RANCHER_FAILED_TEST = os.environ.get("RANCHER_FAILED_TEST")
DATA_SUBDIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           'resource')

def test_sonobuoy_results():
    config = base64.b64decode(RANCHER_KUBECONFIG).decode("utf-8")
    kubeconfig = DATA_SUBDIR + "/config"
    print(kubeconfig)

    with open(kubeconfig, 'w') as f:
        f.write(config)

    run_sonobuoy_test(kubeconfig)
    get_sonobuoy_results(kubeconfig)
    delete_sonobuoy_from_cluster(kubeconfig)


def run_sonobuoy_test(kubeconfig):
    if not RANCHER_FAILED_TEST:
        cmd = "sonobuoy run --mode={0} --kube-conformance-image-version={1} --kubeconfig={2}".format(RANCHER_SONOBUOY_MODE, RANCHER_K8S_VERSION, kubeconfig)
    else:
       cmd = "sonobuoy run {0} --kube-conformance-image-version={1} --kubeconfig={2}".format(RANCHER_FAILED_TEST, RANCHER_K8S_VERSION, kubeconfig)
    status = run_command(cmd)
    time.sleep(60)


def get_sonobuoy_results(kubeconfig):
    cmd = "sonobuoy status --kubeconfig={0}".format(kubeconfig)
    status = run_command(cmd)
    print(status)
    while "running" in status or "Pending" in status:
        status = run_command(cmd, log_out=False)
        time.sleep(120)

    cmd = "sonobuoy status --kubeconfig={0}".format(kubeconfig)
    status = run_command(cmd)
    print(status)

    cmd = "sonobuoy retrieve --kubeconfig={0}".format(kubeconfig)
    result = run_command(cmd, log_out=False)

    cmd = "tar xzf {0}".format(result)
    status = run_command(cmd, log_out=False)

    filepath = "./plugins/e2e/results/global/e2e.log"
    is_file = os.path.isfile(filepath)
    assert is_file

    cmd = "sonobuoy results {0}".format(result)
    result = run_command(cmd)
    print(result)


def delete_sonobuoy_from_cluster(kubeconfig):
    cmd = "sonobuoy delete --all --wait --kubeconfig={0}".format(kubeconfig)
    result = run_command(cmd)
    print(result)
