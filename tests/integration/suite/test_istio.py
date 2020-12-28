import os
import pytest
import subprocess
from .common import random_str
from .conftest import cluster_and_client, ClusterContext

kube_fname = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                          "k8s_kube_config")
istio_crd_url = "https://raw.githubusercontent.com/istio/istio/1.1.5" \
                "/install/kubernetes/helm/istio-init/files/crd-10.yaml"


@pytest.mark.nonparallel
def test_virtual_service(admin_pc):
    client = admin_pc.client
    ns = admin_pc.cluster.client.create_namespace(
        name=random_str(),
        projectId=admin_pc.project.id)
    name = random_str()
    client.create_virtualService(
        name=name,
        namespaceId=ns.id,
        hosts=["test"],
        http=[{
            "route": [
                {
                    "destination": {
                        "host": "test",
                        "subset": "v1"
                    }
                }
            ]
        }],
    )
    virtualServices = client.list_virtualService(
        namespaceId=ns.id
    )
    assert len(virtualServices) == 1
    client.delete(virtualServices.data[0])
    client.delete(ns)


@pytest.mark.nonparallel
def test_destination_rule(admin_pc):
    client = admin_pc.client
    ns = admin_pc.cluster.client.create_namespace(
        name=random_str(),
        projectId=admin_pc.project.id)
    name = random_str()
    client.create_destinationRule(
        name=name,
        namespaceId=ns.id,
        host="test",
        subsets=[{
            "name": "v1",
            "labels": {
                "version": "v1",
            }
        }],
    )
    destinationRules = client.list_destinationRule(
        namespaceId=ns.id
    )
    assert len(destinationRules) == 1
    client.delete(destinationRules.data[0])
    client.delete(ns)


# consistentHash has a "oneOf" only openAPI validation on it,
# and our types were passing multiple options which failed.
# This test ensures you can pass a single option.
# See: https://github.com/rancher/rancher/issues/25515
@pytest.mark.nonparallel
def test_destination_rule_on_cookie(admin_pc, remove_resource):
    client = admin_pc.client
    ns = admin_pc.cluster.client.create_namespace(
        name=random_str(),
        projectId=admin_pc.project.id)
    remove_resource(ns)
    name = random_str()
    cookie_name = name + "_cookie"
    dr = client.create_destinationRule(
        name=name,
        namespaceId=ns.id,
        host="test",
        subsets=[{
            "name": "v1",
            "labels": {
                "version": "v1",
            }
        }],
        trafficPolicy={
            "loadBalancer": {
                "consistentHash": {
                    "httpCookie": {
                        "ttl": "0s",
                        "name": cookie_name,
                    }
                }
            }
        }
    )
    remove_resource(dr)
    destinationRules = client.list_destinationRule(
        namespaceId=ns.id
    )
    assert len(destinationRules) == 1
    assert destinationRules.data[0].trafficPolicy.loadBalancer\
        .consistentHash.httpCookie.name == cookie_name


@pytest.mark.nonparallel
def test_gateway(admin_pc):
    client = admin_pc.client
    ns = admin_pc.cluster.client.create_namespace(
        name=random_str(),
        projectId=admin_pc.project.id)
    name = random_str()
    client.create_gateway(
        name=name,
        namespaceId=ns.id,
        servers=[{
            "hosts": [
                "*",
            ],
            "port": {
                "number": 443,
                "name": "https",
                "protocol": "HTTPS",
            },
            "tls": {
                "mode": "SIMPLE",
                "serverCertificate": "/etc/certs/server.pem",
                "privateKey": "/etc/certs/privatekey.pem",
            }
        }],
    )
    gateways = client.list_gateway(
        namespaceId=ns.id
    )
    assert len(gateways) == 1
    client.delete(gateways.data[0])
    client.delete(ns)


@pytest.fixture(scope='module', autouse="True")
def install_crd(admin_mc):
    cluster, client = cluster_and_client('local', admin_mc.client)
    cc = ClusterContext(admin_mc, cluster, client)
    create_kubeconfig(cc.cluster)
    try:
        return subprocess.check_output(
            'kubectl apply ' +
            ' --kubeconfig ' + kube_fname +
            ' -f ' + istio_crd_url,
            stderr=subprocess.STDOUT, shell=True,
        )
    except subprocess.CalledProcessError as err:
        print('kubectl error: ' + str(err.output))
        raise err


def teardown_module(module):
    try:
        return subprocess.check_output(
            'kubectl delete ' +
            ' --kubeconfig ' + kube_fname +
            ' -f ' + istio_crd_url,
            stderr=subprocess.STDOUT, shell=True,
        )
    except subprocess.CalledProcessError as err:
        print('kubectl error: ' + str(err.output))
        raise err


def create_kubeconfig(cluster):
    generateKubeConfigOutput = cluster.generateKubeconfig()
    print(generateKubeConfigOutput.config)
    file = open(kube_fname, "w")
    file.write(generateKubeConfigOutput.config)
    file.close()
