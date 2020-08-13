import os
from .common import  get_user_client
from .common import random_test_name
from .common import  validate_cluster
from .common import  wait_for_cluster_delete
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
AWS_SSH_KEY_NAME = os.environ.get("AWS_SSH_KEY_NAME")
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
def test_eks_v2_hosted_cluster_create_basic():
    cluster_name = random_test_name("test-auto-eks")
    eks_config_temp = get_eks_config_basic(cluster_name)
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

    # validate cluster created
    validate_eks_cluster(cluster_name, eks_config_temp)

    # validate nodegroups created
    validate_nodegroup(eks_config_temp["nodeGroups"], cluster_name)


@ekscredential
def test_eks_v2_hosted_cluster_create_all():
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

    # validate cluster created
    validate_eks_cluster(cluster_name, eks_config_temp)

    # validate nodegroups created
    validate_nodegroup(eks_config_temp["nodeGroups"], cluster_name)


@ekscredential
def test_eks_v2_hosted_cluster_edit():
    cluster_name = random_test_name("test-auto-eks")
    eks_config_temp = get_eks_config_basic(cluster_name)
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

    # validate cluster created
    validate_eks_cluster(cluster_name, eks_config_temp)

    # validate nodegroups created
    validate_nodegroup(eks_config_temp["nodeGroups"], cluster_name)


@ekscredential
def test_eks_v2_hosted_cluster_delete():
    cluster_name = random_test_name("test-auto-eks")
    eks_config_temp = get_eks_config_basic(cluster_name)
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
    AmazonWebServices().wait_for_delete_eks_cluster(cluster_name)


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
            if len(client.list_cluster(name=name).data) > 0:
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


def get_eks_config_basic(cluster_name):
    ec2_cloud_credential = get_aws_cloud_credential()
    global eks_config
    eks_config_temp = eks_config.copy()
    eks_config_temp["displayName"] = cluster_name
    eks_config_temp["amazonCredentialSecret"] = ec2_cloud_credential.id
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
        "nodegroupName": random_test_name("test-ng"),
        "ec2SshKey": AWS_SSH_KEY_NAME.split(".pem")[0],
        "type": "nodeGroup"
    }
    return new_nodegroup


def validate_eks_cluster(cluster_name, eks_config_temp):
    eks_cluster = AmazonWebServices().describe_eks_cluster(cluster_name)
    print("\nEKS cluster deployed in EKS Console: {}".
          format(eks_cluster["cluster"]))

    # check k8s version
    assert eks_cluster["cluster"]["version"] == \
           eks_config_temp["kubernetesVersion"], "K8s version is incorrect"

    # check cluster status
    assert eks_cluster["cluster"]["status"] == "ACTIVE", \
        "Cluster is NOT in active state"

    # verify security groups
    if "securityGroups" in eks_config_temp.keys():
        assert \
    eks_cluster["cluster"]["resourcesVpcConfig"]["securityGroupIds"].sort() \
    == eks_config_temp["securityGroups"].sort()\
        , "Mismatch in Security Groups"

    # verify subnets
    if "subnets" in eks_config_temp.keys():
        assert \
            eks_cluster["cluster"]["resourcesVpcConfig"]["subnetIds"].sort() \
            == eks_config_temp["subnets"].sort(), "Mismatch in Security Groups"

    # verify logging types
    if "loggingTypes" in eks_config_temp.keys():
        for logging in eks_cluster["cluster"]["logging"]["clusterLogging"]:
            if logging["enabled"]:
                assert logging["types"].sort() \
                       == eks_config_temp["loggingTypes"].sort() , \
                    "Mismatch in Logging types set"

    # verify serviceRole
    if "serviceRole" in eks_config_temp.keys():
        assert eks_config_temp["serviceRole"] in \
               eks_cluster["cluster"]["roleArn"]

    # verify publicAccessSources
    if "publicAccessSources" in eks_config_temp.keys():
        assert eks_config_temp["publicAccessSources"].sort() == \
    eks_cluster["cluster"]["resourcesVpcConfig"]["publicAccessCidrs"].sort()


def edit_eks_cluster(cluster, eks_config_temp):
    # edit eks_config_temp
    # add new cloud cred
    ec2_cloud_credential_new = get_aws_cloud_credential()
    eks_config_temp["amazonCredentialSecret"] = ec2_cloud_credential_new.id
    # add cluster level tags
    eks_config_temp["tags"] = {"cluster-level-2": "tag2"}
    # add node group
    new_nodegroup = get_new_node()
    eks_config_temp["nodeGroups"].append(new_nodegroup)
    # remove all logging
    eks_config_temp["loggingTypes"] = ["audit","api","authenticator"]
    client = get_user_client()
    client.update(cluster, name=cluster.name, eksConfig=eks_config_temp)
    cluster = validate_cluster(client, cluster, intermediate_state="updating",
                               check_intermediate_state=True,
                               skipIngresscheck=True,
                               timeout=DEFAULT_TIMEOUT_EKS)
    return cluster


def validate_nodegroup(nodegroup_list, cluster_name):
    for nodegroup in nodegroup_list:
        print("nodegroup:", nodegroup)
        eks_nodegroup = AmazonWebServices().describe_eks_nodegroup(
            cluster_name, nodegroup["nodegroupName"]
        )
        print("\nNode Group from EKS console: {}".format(eks_nodegroup))

        # k8s version check
        eks_cluster = AmazonWebServices().describe_eks_cluster(cluster_name)
        assert eks_cluster["cluster"]["version"] == \
               eks_nodegroup["nodegroup"]["version"], \
            "Mismatch between K8s version of cluster and nodegroup"

        # status of nodegroup
        assert eks_nodegroup["nodegroup"]["status"] == "ACTIVE", \
            "Nodegroups are not in active status"

        # check scalingConfig
        assert nodegroup["maxSize"] \
               == eks_nodegroup["nodegroup"]["scalingConfig"]["maxSize"], \
            "maxSize is incorrect on the nodes"
        assert nodegroup["minSize"] \
               == eks_nodegroup["nodegroup"]["scalingConfig"]["minSize"], \
            "minSize is incorrect on the nodes"
        assert nodegroup["minSize"] \
               == eks_nodegroup["nodegroup"]["scalingConfig"]["minSize"], \
            "minSize is incorrect on the nodes"

        # check instance type
        assert nodegroup["instanceType"] \
               == eks_nodegroup["nodegroup"]["instanceTypes"][0], \
            "instanceType is incorrect on the nodes"

        # check disk size
        assert nodegroup["diskSize"] \
               == eks_nodegroup["nodegroup"]["diskSize"], \
            "diskSize is incorrect on the nodes"
        # check ec2SshKey
        if "ec2SshKey" in nodegroup.keys():
            assert nodegroup["ec2SshKey"] \
                   == eks_nodegroup["nodegroup"]["remoteAccess"]["ec2SshKey"], \
                "Ssh key is incorrect on the nodes"
