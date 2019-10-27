from .common import *  # NOQA
import pytest

EKS_ACCESS_KEY = os.environ.get('RANCHER_EKS_ACCESS_KEY', "")
EKS_SECRET_KEY = os.environ.get('RANCHER_EKS_SECRET_KEY', "")
EKS_AMI = os.environ.get('RANCHER_EKS_AMI', "")
EKS_REGION = os.environ.get('RANCHER_EKS_REGION', "us-west-2")
EKS_K8S_VERSION = os.environ.get('RANCHER_EKS_K8S_VERSION', "1.14")

ekscredential = pytest.mark.skipif(not (EKS_ACCESS_KEY and EKS_SECRET_KEY),
                                   reason='EKS Credentials not provided, '
                                          'cannot create cluster')


@ekscredential
def test_create_eks_cluster():

    client = get_admin_client()
    eksConfig = get_eks_config()

    print("Cluster creation")
    cluster = client.create_cluster(eksConfig)
    print(cluster)
    cluster = validate_cluster(client, cluster, check_intermediate_state=True,
                               skipIngresscheck=True)

    cluster_cleanup(client, cluster)


def get_eks_config():

    amazonConfig = {
        "accessKey": EKS_ACCESS_KEY,
        "secretKey": EKS_SECRET_KEY,
        "instanceType": "m4.large",
        "maximumNodes": 3,
        "minimumNodes": 1,
        "kubernetesVersion": EKS_K8S_VERSION,
        "region": EKS_REGION,
        "subnets": [],
        "type": "amazonElasticContainerServiceConfig",
        "virtualNetwork": None,
        "dockerRootDir": "/var/lib/docker",
        "enableNetworkPolicy": False,
    }

    if EKS_AMI is not None:
        amazonConfig.update({"ami": EKS_AMI})

    # Generate the config for EKS cluster
    eksConfig = {

        "amazonElasticContainerServiceConfig": amazonConfig,
        "name": random_test_name("test-auto-eks"),
        "type": "cluster"
    }
    print("\nEKS Configuration")
    print(eksConfig)

    return eksConfig
