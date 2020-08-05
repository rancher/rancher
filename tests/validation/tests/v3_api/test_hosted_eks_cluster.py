import os
from .common import get_user_client, validate_cluster, \
    random_test_name, AWS_SSH_KEY_NAME
from .test_create_ha import resource_prefix
from lib.aws import AmazonWebServices
import pytest

EKS_ACCESS_KEY = os.environ.get('RANCHER_EKS_ACCESS_KEY', "")
EKS_SECRET_KEY = os.environ.get('RANCHER_EKS_SECRET_KEY', "")
EKS_REGION = os.environ.get('RANCHER_EKS_REGION', "us-east-2")
EKS_K8S_VERSION = os.environ.get('RANCHER_EKS_K8S_VERSION', "1.17")
EKS_NODESIZE = os.environ.get('RANCHER_EKS_NODESIZE', 2)
KMS_KEY = os.environ.get('RANCHER_KMS_KEY', None)
SECRET_ENCRYPTION = os.environ.get('RANCHER_SECRET_ENCRYPTION', False)
LOGGING_TYPES = os.environ.get('RANCHER_LOGGING_TYPES', None)
EKS_SERVICE_ROLE = os.environ.get('RANCHER_EKS_SERVICE_ROLE', None)
EKS_SUBNETS = os.environ.get('RANCHER_EKS_SUBNETS', None)
EKS_SECURITYGROUP = os.environ.get('RANCHER_EKS_SECURITYGROUP', None)
EKS_PUBLIC_ACCESS_SOURCES = \
    os.environ.get('RANCHER_EKS_PUBLIC_ACCESS_SOURCES', None)
ekscredential = pytest.mark.skipif(not (EKS_ACCESS_KEY and EKS_SECRET_KEY),
                                   reason='EKS Credentials not provided, '
                                          'cannot create cluster')
DEFAULT_TIMEOUT_EKS = 1200
IMPORTED_EKS_CLUSTERS = []

cluster_details = {}

eks_config = {
        "imported": False,
        "kubernetesVersion": EKS_K8S_VERSION,
        "privateAccess": False,
        "publicAccess": True,
        "region": EKS_REGION,
        "type": "eksclusterconfigspec",
        "nodeGroups": [{
            "desiredSize": EKS_NODESIZE,
            "diskSize": 20,
            "gpu": False,
            "instanceType": "t3.medium",
            "maxSize": EKS_NODESIZE,
            "minSize": EKS_NODESIZE,
            "nodegroupName": random_test_name("test-ng"),
            "type": "nodeGroup"
        }]
    }


@ekscredential
def test_eks_v2_create_hosted_cluster_1():
    ec2_cloud_credential = get_aws_cloud_credential()
    cluster_name = random_test_name("test-auto-eks")
    global eks_config
    eks_config_temp = eks_config.copy()
    eks_config_temp["displayName"] = cluster_name
    eks_config_temp["amazonCredentialSecret"] = ec2_cloud_credential.id
    cluster_config = {
        "eksConfig": eks_config_temp,
        "name": cluster_name,
        "type": "cluster",
        "dockerRootDir": "/var/lib/docker",
        "enableNetworkPolicy": False,
        "enableClusterAlerting": False,
        "enableClusterMonitoring": False
    }
    create_and_validate_eks_cluster(cluster_config)


@ekscredential
def test_eks_v2_create_hosted_cluster_2():
    ec2_cloud_credential = get_aws_cloud_credential()
    cluster_name = random_test_name("test-auto-eks")
    global eks_config
    public_access = [] if EKS_PUBLIC_ACCESS_SOURCES \
                          is None else EKS_PUBLIC_ACCESS_SOURCES.split(",")
    eks_config_temp = eks_config.copy()
    eks_config_temp["displayName"] = cluster_name
    eks_config_temp["amazonCredentialSecret"] = ec2_cloud_credential.id
    eks_config_temp["kmsKey"] = KMS_KEY
    eks_config_temp["secretsEncryption"] = SECRET_ENCRYPTION
    eks_config_temp["subnets"] = [] \
        if EKS_SUBNETS is None else EKS_SUBNETS.split(",")
    eks_config_temp["securityGroups"] = [] \
        if EKS_SECURITYGROUP is None else EKS_SECURITYGROUP.split(",")
    eks_config_temp["publicAccessSources"] = public_access
    eks_config_temp["tags"] = {"cluster-level": "tag1"}
    eks_config_temp["nodeGroups"][0]["tags"] = \
        {"nodegroup-level": "tag1", "nodegroup-level": "tag2"}
    eks_config_temp["nodeGroups"][0]["labels"] = {"label1": "value1"}
    eks_config_temp["loggingTypes"] = get_logging_types()
    eks_config_temp["serviceRole"] = EKS_SERVICE_ROLE
    eks_config_temp["ec2SshKey"] = AWS_SSH_KEY_NAME
    cluster_config = {
        "eksConfig": eks_config_temp,
        "name": cluster_name,
        "type": "cluster",
        "dockerRootDir": "/var/lib/docker",
        "enableNetworkPolicy": False,
        "enableClusterAlerting": False,
        "enableClusterMonitoring": False
    }
    create_and_validate_eks_cluster(cluster_config)


@ekscredential
def test_eks_v2_create_import_cluster():
    ec2_cloud_credential = get_aws_cloud_credential()
    display_name = create_resources_eks()
    cluster_name = random_test_name("test-auto-eks")

    eks_config_temp = {
        "amazonCredentialSecret": ec2_cloud_credential.id,
        "displayName": display_name,
        "imported": True,
        "privateAccess": False,
        "publicAccess": False,
        "region": EKS_REGION,
        "secretsEncryption": False,
        "type": "eksclusterconfigspec"
    }

    cluster_config = {
        "eksConfig": eks_config_temp,
        "name": cluster_name,
        "type": "cluster",
        "dockerRootDir": "/var/lib/docker",
        "enableNetworkPolicy": False,
        "enableClusterAlerting": False,
        "enableClusterMonitoring": False
    }
    create_and_validate_eks_cluster(cluster_config,
                                    imported=True)


def create_resources_eks():
    cluster_name = resource_prefix + "-ekscluster"
    AmazonWebServices().create_eks_cluster(cluster_name)
    IMPORTED_EKS_CLUSTERS.append(cluster_name)
    AmazonWebServices().wait_for_eks_cluster_state(cluster_name, "ACTIVE")
    return cluster_name


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):

    def fin():
        client = get_user_client()
        for name, cluster in cluster_details.items():
            client.delete(cluster)
        for display_name in IMPORTED_EKS_CLUSTERS:
            AmazonWebServices().delete_eks_cluster(cluster_name=display_name)

    request.addfinalizer(fin)


def create_and_validate_eks_cluster(cluster_config, imported=False):
    client = get_user_client()
    print("Creating EKS cluster")
    print("\nEKS Configuration: {}".format(cluster_config))
    cluster = client.create_cluster(cluster_config)
    print(cluster)
    cluster_details[cluster["name"]] = cluster
    intermediate_state = False if imported else True
    cluster = validate_cluster(client, cluster,
                               check_intermediate_state=intermediate_state,
                               skipIngresscheck=True,
                               timeout=DEFAULT_TIMEOUT_EKS)
    return client, cluster


def get_aws_cloud_credential():
    client = get_user_client()
    ec2_cloud_credential_config = {
        "accessKey": EKS_ACCESS_KEY,
        "secretKey": EKS_SECRET_KEY
    }
    ec2_cloud_credential = client.create_cloud_credential(
        amazonec2credentialConfig=ec2_cloud_credential_config
    )
    return ec2_cloud_credential


def get_logging_types():
    logging_types = []
    if LOGGING_TYPES is not None:
        temp = LOGGING_TYPES.split(",")
        for logging in temp:
            logging_types.append(logging)
    return logging_types
