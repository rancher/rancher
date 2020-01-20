import pytest
import datetime
import time
from .common import get_user_client_and_cluster
from .common import validate_cluster_state
from .common import get_etcd_nodes
from .common import wait_for_nodes_to_become_active
from .common import wait_for_node_status

# Globals
# Master list of all certs
ALL_CERTS = ["kube-apiserver", "kube-controller-manager",
             "kube-node", "kube-proxy", "kube-scheduler",
             "kube-etcd", "kube-ca"]


def test_rotate_all_certs():
    changed = ALL_CERTS.copy()
    changed.remove("kube-ca")
    unchanged = ["kube-ca"]
    rotate_and_compare(unchanged, changed)


def test_rotate_kube_apiserver():
    changed = ["kube-apiserver"]
    unchanged = ALL_CERTS.copy()
    unchanged.remove("kube-apiserver")
    rotate_and_compare(unchanged, changed, "kube-apiserver")


def test_rotate_kube_controller_manager():
    changed = ["kube-controller-manager"]
    unchanged = ALL_CERTS.copy()
    unchanged.remove("kube-controller-manager")
    rotate_and_compare(unchanged, changed, "kube-controller-manager")


def test_rotate_kube_etcd():
    changed = ["kube-etcd"]
    unchanged = ALL_CERTS.copy()
    unchanged.remove("kube-etcd")
    rotate_and_compare(unchanged, changed, "etcd")


def test_rotate_kube_node():
    changed = ["kube-node"]
    unchanged = ALL_CERTS.copy()
    unchanged.remove("kube-node")
    rotate_and_compare(unchanged, changed, "kubelet")


def test_rotate_kube_proxy():
    changed = ["kube-proxy"]
    unchanged = ALL_CERTS.copy()
    unchanged.remove("kube-proxy")
    rotate_and_compare(unchanged, changed, "kube-proxy")


def test_rotate_kube_scheduler():
    changed = ["kube-scheduler"]
    unchanged = ALL_CERTS.copy()
    unchanged.remove("kube-scheduler")
    rotate_and_compare(unchanged, changed, "kube-scheduler")


def test_rotate_kube_ca():
    changed = ALL_CERTS
    unchanged = []
    rotate_and_compare(unchanged, changed, "kube-ca")


# Gets the certificate expiration date and cert name. Stores them in a dict.
def get_certs():
    certs = {}
    client, cluster = get_user_client_and_cluster()
    for key in cluster.certificatesExpiration:
        if "kube-etcd" not in key:
            certs[key] = parse_datetime(cluster.certificatesExpiration[key]
                                        ["expirationDate"])

    # Get etcd node certs from node IP
    nodes = get_etcd_nodes(cluster)
    for node in nodes:
        if node["labels"]["node-role.kubernetes.io/etcd"] == "true":
            ipKey = "kube-etcd-"+node["ipAddress"].replace(".", "-")
            certs[ipKey] = parse_datetime(cluster.certificatesExpiration[ipKey]
                                          ["expirationDate"])
    return certs


# Turn expiration string into datetime
def parse_datetime(expiration_string):
    return datetime.datetime.strptime(expiration_string, '%Y-%m-%dT%H:%M:%SZ')


def compare_changed(certs2, time_now, changed):
    if "kube-etcd" in changed:
        for key in certs2:
            if "kube-etcd" in key:
                changed.append(key)
        changed.remove("kube-etcd")
    for i in changed:
        assert(certs2[i] > (time_now + datetime.timedelta(days=3650)))


def compare_unchanged(certs1, certs2, unchanged):
    if "kube-etcd" in unchanged:
        for key in certs2:
            if "kube-etcd" in key:
                unchanged.append(key)
        unchanged.remove("kube-etcd")
    for i in unchanged:
        assert(certs1[i] == certs2[i])


def rotate_certs(service=""):
    client, cluster = get_user_client_and_cluster()
    if service:
        if service == "kube-ca":
            rotate = cluster.rotateCertificates(caCertificates=True)
        else:
            rotate = cluster.rotateCertificates(services=service)
    else:
        rotate = cluster.rotateCertificates()


def rotate_and_compare(unchanged, changed, service=""):
    client, cluster = get_user_client_and_cluster()
    # Grab certs before rotation
    certs1 = get_certs()
    now = datetime.datetime.now()
    # Rotate certs
    rotate_certs(service)
    # wait for cluster to update
    cluster = validate_cluster_state(client, cluster,
                                     intermediate_state="updating")
    if service == "kube-ca":
        time.sleep(60)
    # Grab certs after rotate
    certs2 = get_certs()
    # Checks the new certs against old certs.
    compare_changed(certs2, now, changed)
    compare_unchanged(certs1, certs2, unchanged)
    time.sleep(120)
    # get all nodes and assert status
    nodes = client.list_node(clusterId=cluster.id).data
    for node in nodes:
        if node["state"] != "active":
            raise AssertionError(
                "Timed out waiting for state to get to active")
