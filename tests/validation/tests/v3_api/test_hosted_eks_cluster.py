import os
from .common import get_user_client, validate_cluster, \
    random_test_name, AWS_SSH_KEY_NAME, wait_for_cluster_delete
from .test_create_ha import resource_prefix
import pytest
from lib.aws import AmazonWebServices

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
    cluster_name = random_test_name("test-auto-eks")
    eks_config_temp = get_eks_config_minimum(cluster_name)
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
    validate_eks_cluster(cluster_name, eks_config_temp)


@ekscredential
def test_create_hosted_eks_cluster_2():
    cluster_name = random_test_name("test-auto-eks")
    eks_config_temp = get_eks_config_all(cluster_name)
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
    validate_eks_cluster(cluster_name, eks_config_temp, all_parameters=True)


@ekscredential
def test_create_hosted_eks_cluster_3():
    cluster_name = random_test_name("test-auto-eks")
    eks_config_temp = get_eks_config_minimum(cluster_name)
    print(eks_config_temp)
    cluster_config = {
        "eksConfig": eks_config_temp,
        "name": cluster_name,
        "type": "cluster",
        "dockerRootDir": "/var/lib/docker",
        "enableNetworkPolicy": False,
        "enableClusterAlerting": False,
        "enableClusterMonitoring": False
    }
    client, cluster = create_and_validate_eks_cluster(cluster_config)
    # edit cluster
    cluster = edit_eks_cluster(cluster, eks_config_temp)


@ekscredential
def test_create_hosted_eks_cluster_4():
    cluster_name = random_test_name("test-auto-eks")
    eks_config_temp = get_eks_config_minimum(cluster_name)
    cluster_config = {
        "eksConfig": eks_config_temp,
        "name": cluster_name,
        "type": "cluster",
        "dockerRootDir": "/var/lib/docker",
        "enableNetworkPolicy": False,
        "enableClusterAlerting": False,
        "enableClusterMonitoring": False
    }
    client, cluster = create_and_validate_eks_cluster(cluster_config)
    # delete cluster
    client.delete(cluster)
    wait_for_cluster_delete(client, cluster)


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


def get_eks_config_minimum(cluster_name):
    ec2_cloud_credential = get_aws_cloud_credential()
    global eks_config
    nodegroup = get_new_node()
    eks_config_temp = eks_config.copy()
    eks_config_temp["displayName"] = cluster_name
    eks_config_temp["amazonCredentialSecret"] = ec2_cloud_credential.id
    eks_config_temp["nodeGroups"] = []
    eks_config_temp["nodeGroups"].append(nodegroup)
    return eks_config_temp


def get_eks_config_all(cluster_name):
    ec2_cloud_credential = get_aws_cloud_credential()
    global eks_config
    public_access = [] if EKS_PUBLIC_ACCESS_SOURCES \
                          is None else EKS_PUBLIC_ACCESS_SOURCES.split(",")
    eks_config_temp = eks_config.copy()
    eks_config_temp["displayName"] = cluster_name
    eks_config_temp["amazonCredentialSecret"] = ec2_cloud_credential.id
    if KMS_KEY is not None: eks_config_temp["kmsKey"] = KMS_KEY
    if SECRET_ENCRYPTION: eks_config_temp["secretsEncryption"] = \
        SECRET_ENCRYPTION
    eks_config_temp["subnets"] = [] \
        if EKS_SUBNETS is None else EKS_SUBNETS.split(",")
    eks_config_temp["securityGroups"] = [] \
        if EKS_SECURITYGROUP is None else EKS_SECURITYGROUP.split(",")
    eks_config_temp["publicAccessSources"] = public_access
    eks_config_temp["tags"] = {"cluster-level": "tag1"}
    eks_config_temp["nodeGroups"] = []
    eks_config_temp["nodeGroups"].append(get_new_node())
    eks_config_temp["nodeGroups"][0]["tags"] = \
        {"nodegroup-level": "tag1", "nodegroup-level": "tag2"}
    eks_config_temp["nodeGroups"][0]["labels"] = {"label1": "value1"}
    eks_config_temp["loggingTypes"] = get_logging_types()
    eks_config_temp["serviceRole"] = EKS_SERVICE_ROLE
    eks_config_temp["ec2SshKey"] = AWS_SSH_KEY_NAME
    return eks_config_temp


def get_new_node():
    new_nodegroup = {
        "desiredSize": EKS_NODESIZE,
        "diskSize": 20,
        "gpu": False,
        "instanceType": "t3.medium",
        "maxSize": EKS_NODESIZE,
        "minSize": EKS_NODESIZE,
        "nodegroupName": random_test_name("test-ng2"),
        "type": "nodeGroup"
    }
    return new_nodegroup


def validate_eks_cluster(cluster_name, eks_config_temp, all_parameters=False):
    eks_cluster = AmazonWebServices().describe_eks_cluster(cluster_name)
    print("\nEKS cluster deployed in EKS Console: {}".
          format(eks_cluster["cluster"]))
    assert eks_cluster["cluster"]["version"] == \
           eks_config["kubernetesVersion"], "K8s version is incorrect"
    assert eks_cluster["cluster"]["status"] == "ACTIVE", \
        "Cluster is NOT in active state"
    nodegroups = eks_config["nodeGroups"]
    for nodegroup in nodegroups:
        print("nodegroup:", nodegroup)
        eks_nodegroup = AmazonWebServices().describe_eks_nodegroup(
            cluster_name, nodegroup["nodegroupName"]
        )
        print("\nNode Group from EKS console: {}".format(eks_nodegroup))
    if all_parameters:
        # check if security groups, subnets are the same
        if EKS_SECURITYGROUP is not None:
            eks_cluster["cluster"]["resourcesVpcConfig"]
            ["securityGroupIds"].sort()
            eks_config_temp["securityGroups"].sort()
            assert eks_cluster["cluster"]["resourcesVpcConfig"]
            ["securityGroupIds"] == \
                   eks_config_temp["securityGroups"] , \
            "Mismatch in Security Groups"
        if EKS_SUBNETS is not None:
            eks_config_temp["subnets"].sort()
            eks_cluster["cluster"]["resourcesVpcConfig"]
            ["subnetIds"].sort()
            assert eks_cluster["cluster"]["resourcesVpcConfig"]
            ["subnetIds"] == \
                   eks_config_temp["subnets"], "Mismatch in Security Groups"
        # verify logging types
        if LOGGING_TYPES is not None:
            assert eks_cluster["cluster"]["logging"]["clusterLogging"]["types"]\
                   == eks_config["cluster"]["loggingTypes"] , \
                "Mismatch in Logging types set"


def edit_eks_cluster(cluster, eks_config_temp):
    # edit eks_config_temp
    # add new cloud cred
    ec2_cloud_credential_new = get_aws_cloud_credential()
    eks_config_temp["amazonCredentialSecret"] = ec2_cloud_credential_new.id
    # add cluster level tags
    eks_config_temp["tags"]["cluster-level-2"] = "tag2"
    # add node group
    new_nodegroup = get_new_node()
    eks_config_temp["nodeGroups"].append(new_nodegroup)
    # remove all logging
    eks_config_temp["loggingTypes"] = get_logging_types()
    client = get_user_client()
    client.update(cluster, eksConfig=eks_config_temp)
    cluster = validate_cluster(client, cluster, intermediate_state="updating",
                               check_intermediate_state=True,
                               skipIngresscheck=True,
                               timeout=DEFAULT_TIMEOUT_EKS)
    return cluster
