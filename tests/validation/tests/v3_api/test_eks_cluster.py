from .common import *  # NOQA
import pytest

EKS_ACCESS_KEY = os.environ.get('RANCHER_EKS_ACCESS_KEY', "")
EKS_SECRET_KEY = os.environ.get('RANCHER_EKS_SECRET_KEY', "")
EKS_AMI = os.environ.get('RANCHER_EKS_AMI', "")
EKS_REGION = os.environ.get('RANCHER_EKS_REGION', "us-west-2")
EKS_K8S_VERSION = os.environ.get('RANCHER_EKS_K8S_VERSION', "1.17")

# Hardcoded to follow UI-style:
# https://github.com/rancher/ui/blob/master/lib/shared/addon/components/cluster-driver/driver-amazoneks/component.js
EKS_K8S_VERSIONS = os.environ.get('RANCHER_EKS_K8S_VERSIONS',
                                  "1.17,1.16,1.15").split(",")

ekscredential = pytest.mark.skipif(not (EKS_ACCESS_KEY and EKS_SECRET_KEY),
                                   reason='EKS Credentials not provided, '
                                          'cannot create cluster')


@ekscredential
def test_create_eks_cluster():
    client, cluster = create_and_validate_eks_cluster(EKS_K8S_VERSION)
    cluster_cleanup(client, cluster)


def create_and_validate_eks_cluster(k8s_version):
    client = get_user_client()
    eks_config = get_eks_config(k8s_version)

    print("Cluster creation")
    cluster = client.create_cluster(eks_config)
    print(cluster)
    cluster = validate_cluster(client, cluster, check_intermediate_state=True,
                               skipIngresscheck=True)
    return client, cluster


def get_eks_config(version):

    amazon_config = {
        "accessKey": EKS_ACCESS_KEY,
        "secretKey": EKS_SECRET_KEY,
        "instanceType": "m4.large",
        "maximumNodes": 3,
        "minimumNodes": 1,
        "kubernetesVersion": version,
        "region": EKS_REGION,
        "subnets": [],
        "type": "amazonElasticContainerServiceConfig",
        "virtualNetwork": None,
        "dockerRootDir": "/var/lib/docker",
        "enableNetworkPolicy": False,
    }

    if EKS_AMI is not None:
        amazon_config.update({"ami": EKS_AMI})

    # Generate the config for EKS cluster
    eks_config = {

        "amazonElasticContainerServiceConfig": amazon_config,
        "name": random_test_name("test-auto-eks"),
        "type": "cluster"
    }
    print("\nEKS Configuration")
    print(eks_config)

    return eks_config
