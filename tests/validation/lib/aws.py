import base64
import boto3
import logging
import os
import rsa
import time

from boto3.exceptions import Boto3Error
from .cloud_provider import CloudProviderBase
from .node import Node

logging.getLogger('boto3').setLevel(logging.CRITICAL)
logging.getLogger('botocore').setLevel(logging.CRITICAL)


AWS_REGION = os.environ.get("AWS_REGION", "us-east-2")
AWS_REGION_AZ = os.environ.get("AWS_REGION_AZ", "us-east-2a")
AWS_SECURITY_GROUP = os.environ.get("AWS_SECURITY_GROUPS",
                                    'sg-0e753fd5550206e55')
AWS_SUBNET = os.environ.get("AWS_SUBNET", "subnet-ee8cac86")
AWS_VPC_ID = os.environ.get("AWS_VPC_ID", "vpc-bfccf4d7")
AWS_SECURITY_GROUPS = [AWS_SECURITY_GROUP]
AWS_ACCESS_KEY_ID = os.environ.get("AWS_ACCESS_KEY_ID")
AWS_SECRET_ACCESS_KEY = os.environ.get("AWS_SECRET_ACCESS_KEY")
AWS_SSH_KEY_NAME = os.environ.get("AWS_SSH_KEY_NAME")
AWS_CICD_INSTANCE_TAG = os.environ.get(
    "AWS_CICD_INSTANCE_TAG", 'rancher-validation')
AWS_INSTANCE_TYPE = os.environ.get("AWS_INSTANCE_TYPE", 't2.medium')
AWS_IAM_PROFILE = os.environ.get("AWS_IAM_PROFILE", "")

AWS_AMI = os.environ.get("AWS_AMI", "")
AWS_USER = os.environ.get("AWS_USER", "ubuntu")
AWS_HOSTED_ZONE_ID = os.environ.get("AWS_HOSTED_ZONE_ID", "")
AWS_VOLUME_SIZE = os.environ.get("AWS_VOLUME_SIZE", "50")
AWS_WINDOWS_VOLUME_SIZE = os.environ.get("AWS_WINDOWS_VOLUME_SIZE", "100")

PRIVATE_IMAGES = {
    "rancheros-v1.5.1-docker-native": {
        'image': 'ami-00769ca587d8e100c', 'ssh_user': 'rancher'},
    "rancheros-v1.5.4-docker-native": {
        'image': 'ami-022fc798437fb846a', 'ssh_user': 'rancher'},
    "rancheros-v1.5.5-docker-native": {
        'image': 'ami-002ab867b8b8591d5', 'ssh_user': 'rancher'},
    "rhel-7.6-docker-native-113": {
        'image': 'ami-04b9aacf7e1512c0b', 'ssh_user': 'ec2-user'},
    "rhel-7.7-docker-native-113-selinux-off": {
        'image': 'ami-03ff3301b6b68eaf8', 'ssh_user': 'ec2-user'},
    "rhel-7.7-docker-native-113-selinux-on": {
        'image': 'ami-0790a63637ac2cc1e', 'ssh_user': 'ec2-user'},
    "rhel-7.7-docker-19.03-selinux-off": {
        'image': 'ami-01ed32daf0a8275e9', 'ssh_user': 'ec2-user'},
    "rhel-7.7-docker-19.03-selinux-on": {
        'image': 'ami-0a3c89a118982f5d8', 'ssh_user': 'ec2-user'},
    "oracleLinux-7.7-kernel-UEKr5-docker-19.03": {
        'image': 'ami-06dd5f94499093e3d', 'ssh_user': 'ec2-user'},
    "suse-sles12-sp2-docker-18061ce": {
        'image': 'ami-0cc154aeb82bd8fa0', 'ssh_user': 'ec2-user'},
    "suse-sles12-sp5-docker-19.03.5": {
        'image': 'ami-0591b00f223575dc5', 'ssh_user': 'ec2-user'},
    "ubuntu-16.04-docker-18.09": {
        'image': 'ami-07e968eb9151b2599', 'ssh_user': 'ubuntu'},
    "ubuntu-18.04-docker-18.09": {
        'image': 'ami-02dcbc347c866fb5d', 'ssh_user': 'ubuntu'},
    "ubuntu-16.04-docker-19.03": {
        'image': 'ami-0965ce682883e7d23', 'ssh_user': 'ubuntu'},
    "ubuntu-18.04-docker-19.03": {
        'image': 'ami-066f14e43aebc7472', 'ssh_user': 'ubuntu'},
    "rhel-7.6-docker-18.09": {
        'image': 'ami-094574ffb6efb3a9b', 'ssh_user': 'ec2-user'},
    "windows-1903-docker-19.03": {
        'image': 'ami-0f5bea682d4bdd318', 'ssh_user': 'Administrator'}}

PUBLIC_AMI = {
    'us-east-2': {
        "ubuntu-18.04": {
            'image': 'ami-0d5d9d301c853a04a', 'ssh_user': 'ubuntu'},
        "ubuntu-16.04": {
            'image': 'ami-965e6bf3', 'ssh_user': 'ubuntu'},
        "rhel-7.4": {
            'image': 'ami-0b1e356e', 'ssh_user': 'ec2-user'},
        "ros-1.4.0": {
            'image': 'ami-504b7435', 'ssh_user': 'rancher'}},
    'us-east-1': {
        "ubuntu-16.04": {
            'image': 'ami-cf6c47aa', 'ssh_user': 'ubuntu'},
        "rhel-7.4": {
            'image': 'ami-0b1e356e', 'ssh_user': 'ec2-user'}}
}


class AmazonWebServices(CloudProviderBase):

    def __init__(self):
        self._client = boto3.client(
            'ec2',
            aws_access_key_id=AWS_ACCESS_KEY_ID,
            aws_secret_access_key=AWS_SECRET_ACCESS_KEY,
            region_name=AWS_REGION)

        self._elbv2_client = boto3.client(
            'elbv2',
            aws_access_key_id=AWS_ACCESS_KEY_ID,
            aws_secret_access_key=AWS_SECRET_ACCESS_KEY,
            region_name=AWS_REGION)

        self._route53_client = boto3.client(
            'route53',
            aws_access_key_id=AWS_ACCESS_KEY_ID,
            aws_secret_access_key=AWS_SECRET_ACCESS_KEY)

        self.master_ssh_key = None
        self.master_ssh_key_path = None

        if AWS_SSH_KEY_NAME:
            self.master_ssh_key = self.get_ssh_key(AWS_SSH_KEY_NAME)
            self.master_ssh_key_path = self.get_ssh_key_path(AWS_SSH_KEY_NAME)

        # Used for cleanup
        self.created_node = []
        self.created_keys = []

    # called if docker is to be installed
    def _select_private_ami(self, os_version=None, docker_version=None):
        os_version = os_version or self.OS_VERSION
        docker_version = docker_version or self.DOCKER_VERSION
        image = PRIVATE_IMAGES[
            "{}-docker-{}".format(os_version, docker_version)]
        return image['image'], image['ssh_user']

    def _select_ami(self, os_version=None):
        image = PUBLIC_AMI[AWS_REGION][os_version]
        return image['image'], image['ssh_user']

    def create_node(
        self, node_name, key_name=None, os_version=None, docker_version=None,
            wait_for_ready=True):

        os_version = os_version or self.OS_VERSION
        docker_version = docker_version or self.DOCKER_VERSION
        if AWS_AMI:
            image = AWS_AMI
            ssh_user = AWS_USER
        else:
            if self.DOCKER_INSTALLED.lower() == 'false':
                image, ssh_user = self._select_ami(os_version)
            else:
                image, ssh_user = \
                    self._select_private_ami(os_version, docker_version)

        if key_name:
            # if cert private key
            if key_name.endswith('.pem'):
                ssh_private_key_name = key_name
                ssh_private_key = self.get_ssh_key(key_name)
                ssh_private_key_path = self.get_ssh_key_path(key_name)
            else:
                # get private key
                ssh_private_key_name = key_name.replace('.pub', '')
                ssh_private_key = self.get_ssh_key(ssh_private_key_name)
                ssh_private_key_path = self.get_ssh_key_path(
                    ssh_private_key_name)
        else:
            key_name = AWS_SSH_KEY_NAME.replace('.pem', '')
            ssh_private_key_name = key_name
            ssh_private_key = self.master_ssh_key
            ssh_private_key_path = self.master_ssh_key_path

        if os_version is not None:
            if 'windows' in os_version:
                volume_size = AWS_WINDOWS_VOLUME_SIZE
                instance_type = 't3.xlarge'
            else:
                volume_size = AWS_VOLUME_SIZE
                instance_type = AWS_INSTANCE_TYPE

        args = {"ImageId": image,
                "InstanceType": instance_type,
                "MinCount": 1,
                "MaxCount": 1,
                "TagSpecifications": [{'ResourceType': 'instance', 'Tags': [
                    {'Key': 'Name', 'Value': node_name},
                    {'Key': 'CICD', 'Value': AWS_CICD_INSTANCE_TAG}]}],
                "KeyName": key_name,
                "NetworkInterfaces": [{
                    'DeviceIndex': 0,
                    'AssociatePublicIpAddress': True,
                    'Groups': AWS_SECURITY_GROUPS}],
                "Placement": {'AvailabilityZone': AWS_REGION_AZ},
                "BlockDeviceMappings":
                    [{"DeviceName": "/dev/sda1", "Ebs": {"VolumeSize": int(volume_size)}}]
                }
        if len(AWS_IAM_PROFILE) > 0:
            args["IamInstanceProfile"] = {'Name': AWS_IAM_PROFILE}

        instance = self._client.run_instances(**args)
        node = Node(
            provider_node_id=instance['Instances'][0]['InstanceId'],
            state=instance['Instances'][0]['State']['Name'],
            ssh_user=ssh_user,
            ssh_key_name=ssh_private_key_name,
            ssh_key_path=ssh_private_key_path,
            ssh_key=ssh_private_key,
            os_version=os_version,
            docker_version=docker_version)

        # mark for clean up at the end
        self.created_node.append(node.provider_node_id)

        if wait_for_ready:
            node = self.wait_for_node_state(node)
            node.ready_node()
        return node

    def create_multiple_nodes(
        self, number_of_nodes, node_name_prefix, os_version=None,
            docker_version=None, key_name=None, wait_for_ready=True):

        nodes = []
        for i in range(number_of_nodes):
            node_name = "{}_{}".format(node_name_prefix, i)
            nodes.append(self.create_node(
                node_name, key_name=key_name, os_version=os_version,
                docker_version=docker_version, wait_for_ready=False))

        if wait_for_ready:
            nodes = self.wait_for_nodes_state(nodes)
            # hack for instances
            if self.DOCKER_INSTALLED.lower() == 'true':
                time.sleep(5)
                self.reboot_nodes(nodes)
                time.sleep(10)
                nodes = self.wait_for_nodes_state(nodes)

            # wait for window nodes to come up so we can decrypt the password
            if os_version is not None:
                if 'windows' in os_version:
                    time.sleep(60 * 6)

            for node in nodes:
                if os_version is not None:
                    if 'windows' in os_version:
                        node.ssh_password = self.decrypt_windows_password(node.provider_node_id)
                node.ready_node()
        return nodes

    def get_node(self, provider_id):
        node_filter = [{
            'Name': 'instance-id', 'Values': [provider_id]}]
        try:
            response = self._client.describe_instances(Filters=node_filter)
            nodes = response.get('Reservations', [])
            if len(nodes) == 0:
                return None  # no node found

            aws_node = nodes[0]['Instances'][0]
            node = Node(
                provider_node_id=provider_id,
                # node_name= aws_node tags?,
                host_name=aws_node.get('PublicDnsName'),
                public_ip_address=aws_node.get('PublicIpAddress'),
                private_ip_address=aws_node.get('PrivateIpAddress'),
                state=aws_node['State']['Name'])
            return node
        except Boto3Error as e:
            msg = "Failed while querying instance '{}' state!: {}".format(
                node.node_id, str(e))
            raise RuntimeError(msg)

    def update_node(self, node):
        node_filter = [{
            'Name': 'instance-id', 'Values': [node.provider_node_id]}]
        try:
            response = self._client.describe_instances(Filters=node_filter)
            nodes = response.get('Reservations', [])
            if len(nodes) == 0 or len(nodes[0]['Instances']) == 0:
                return node

            aws_node = nodes[0]['Instances'][0]
            node.state = aws_node['State']['Name']
            node.host_name = aws_node.get('PublicDnsName')
            node.public_ip_address = aws_node.get('PublicIpAddress')
            node.private_ip_address = aws_node.get('PrivateIpAddress')
            return node
        except Boto3Error as e:
            msg = "Failed while querying instance '{}' state!: {}".format(
                node.node_id, str(e))
            raise RuntimeError(msg)

    def start_node(self, node, wait_for_start=True):
        self._client.start_instances(
            InstanceIds=[node.provider_node_id])
        if wait_for_start:
            node = self.wait_for_node_state(node)
        return node

    def reboot_nodes(self, nodes):
        instances = [node.provider_node_id for node in nodes]
        self._client.reboot_instances(
            InstanceIds=instances)
        return

    def stop_node(self, node, wait_for_stopped=False):
        self._client.stop_instances(
            InstanceIds=[node.provider_node_id])
        if wait_for_stopped:
            node = self.wait_for_node_state(node, 'stopped')
        return node

    def delete_node(self, node, wait_for_deleted=False):
        self._client.terminate_instances(
            InstanceIds=[node.provider_node_id])
        if wait_for_deleted:
            node = self.wait_for_node_state(node, 'terminated')
        return node

    def wait_for_node_state(self, node, state='running'):
        # 'running', 'stopped', 'terminated'
        timeout = 300
        start_time = time.time()
        while time.time() - start_time < timeout:
            node = self.update_node(node)
            if node.state == state:
                return node
            time.sleep(5)

    def wait_for_nodes_state(self, nodes, state='running'):
        # 'running', 'stopped', 'terminated'
        timeout = 300
        start_time = time.time()
        completed_nodes = []
        while time.time() - start_time < timeout:
            for node in nodes:
                if len(completed_nodes) == len(nodes):
                    time.sleep(20)  # Give the node some extra time
                    return completed_nodes
                if node in completed_nodes:
                    continue
                node = self.update_node(node)
                if node.state == state:
                    completed_nodes.append(node)
                time.sleep(1)
            time.sleep(4)

    def import_ssh_key(self, ssh_key_name, public_ssh_key):
        self._client.delete_key_pair(KeyName=ssh_key_name)
        self._client.import_key_pair(
            KeyName=ssh_key_name,
            PublicKeyMaterial=public_ssh_key)
        # mark keys for cleanup
        self.created_keys.append(ssh_key_name)

    def delete_ssh_key(self, ssh_key_name):
        self._client.delete_key_pair(KeyName=ssh_key_name)

    def get_nodes(self, filters):
        try:
            response = self._client.describe_instances(Filters=filters)
            nodes = response.get('Reservations', [])
            if len(nodes) == 0:
                return None  # no node found
            ret_nodes = []
            for aws_node_i in nodes:
                aws_node = aws_node_i['Instances'][0]
                node = Node(
                    provider_node_id=aws_node.get('InstanceId'),
                    # node_name= aws_node tags?,
                    host_name=aws_node.get('PublicDnsName'),
                    public_ip_address=aws_node.get('PublicIpAddress'),
                    private_ip_address=aws_node.get('PrivateIpAddress'),
                    state=aws_node['State']['Name'])
                ret_nodes.append(node)
            return ret_nodes
        except Boto3Error as e:
            msg = "Failed while getting instances: {}".format(str(e))
            raise RuntimeError(msg)

    def delete_nodes(self, nodes, wait_for_deleted=False):
        instance_ids = [node.provider_node_id for node in nodes]
        self._client.terminate_instances(InstanceIds=instance_ids)
        if wait_for_deleted:
            for node in nodes:
                node = self.wait_for_node_state(node, 'terminated')

    def delete_keypairs(self, name_prefix):
        if len(name_prefix) > 0:
            key_pairs = self._client.describe_key_pairs()
            print(key_pairs["KeyPairs"])
            key_pair_list = key_pairs["KeyPairs"]
            print(len(key_pair_list))
            for key in key_pair_list:
                keyName = key["KeyName"]
                if keyName.startswith(name_prefix):
                    print(keyName)
                    self._client.delete_key_pair(KeyName=keyName)

    def _s3_list_files(self, client):
        """List files in specific S3 URL"""
        response = client.list_objects(Bucket=os.environ.get(
                                           "AWS_S3_BUCKET_NAME", ""),
                                       Prefix=os.environ.get(
                                           "AWS_S3_BUCKET_FOLDER_NAME", ""))
        for content in response.get('Contents', []):
            yield content.get('Key')

    def s3_backup_check(self, filename=""):
        print(AWS_REGION)
        print(AWS_REGION_AZ)
        client = boto3.client(
            's3',
            aws_access_key_id=AWS_ACCESS_KEY_ID,
            aws_secret_access_key=AWS_SECRET_ACCESS_KEY,
            region_name=AWS_REGION)
        file_list = self._s3_list_files(client)
        found = False
        for file in file_list:
            print(file)
            if filename in file:
                found = True
                break
        return found

    def register_targets(self, targets, target_group_arn):
        self._elbv2_client.register_targets(
            TargetGroupArn=target_group_arn,
            Targets=targets)

    def describe_target_health(self, target_group_arn):
        return self._elbv2_client.describe_target_health(
            TargetGroupArn=target_group_arn)

    def deregister_all_targets(self, target_group_arn):
        target_health_descriptions = \
            self.describe_target_health(target_group_arn)

        if len(target_health_descriptions["TargetHealthDescriptions"]) > 0:
            targets = []

            for target in \
                    target_health_descriptions["TargetHealthDescriptions"]:
                target_obj = target["Target"]
                targets.append(target_obj)

            self._elbv2_client.deregister_targets(
                TargetGroupArn=target_group_arn,
                Targets=targets)

    def create_network_lb(self, name):
        return self._elbv2_client.create_load_balancer(
            Name=name, Subnets=[AWS_SUBNET], Type='network'
        )

    def delete_lb(self, loadBalancerARN):
        self._elbv2_client.delete_load_balancer(
            LoadBalancerArn=loadBalancerARN
        )

    def create_ha_target_group(self, port, name):
        return self._elbv2_client.create_target_group(
            Name=name,
            Protocol='TCP',
            Port=port,
            VpcId=AWS_VPC_ID,
            HealthCheckProtocol='HTTP',
            HealthCheckPort='80',
            HealthCheckEnabled=True,
            HealthCheckPath='/healthz',
            HealthCheckIntervalSeconds=10,
            HealthCheckTimeoutSeconds=6,
            HealthyThresholdCount=3,
            UnhealthyThresholdCount=3,
            Matcher={
                'HttpCode': '200-399'
            },
            TargetType='instance'
        )

    def delete_target_group(self, targetGroupARN):
        self._elbv2_client.delete_target_group(
            TargetGroupArn=targetGroupARN
        )

    def create_ha_nlb_listener(self, loadBalancerARN, port, targetGroupARN):
        return self._elbv2_client.create_listener(
            LoadBalancerArn=loadBalancerARN,
            Protocol='TCP',
            Port=port,
            DefaultActions=[{'Type': 'forward',
                             'TargetGroupArn': targetGroupARN}]
        )

    def upsert_route_53_record_cname(self, recordName, recordValue):
        return self._route53_client.change_resource_record_sets(
            HostedZoneId=AWS_HOSTED_ZONE_ID,
            ChangeBatch={
                'Comment': 'update',
                'Changes': [{
                    'Action': 'UPSERT',
                    'ResourceRecordSet': {
                        'Name': recordName,
                        'Type': 'CNAME',
                        'TTL': 300,
                        'ResourceRecords': [{
                            'Value': recordValue
                        }]
                    }
                }]
            }
        )

    def decrypt_windows_password(self, instance_id):
        password = ""
        password_data = self._client.\
            get_password_data(InstanceId=instance_id)['PasswordData']
        if password_data:
            password = base64.b64decode(password_data)
            with open (self.get_ssh_key_path(AWS_SSH_KEY_NAME),'r') as privkeyfile:
                priv = rsa.PrivateKey.load_pkcs1(privkeyfile.read())
                password = rsa.decrypt(password,priv).decode('utf-8')

        return password

    def get_ebs_volumes(self, provider_node_id):
        node_filter = [{
            'Name': 'attachment.instance-id', 'Values': [provider_node_id]}]
        try:
            response = self._client.describe_volumes(Filters=node_filter)
            volumes = response.get('Volumes', [])
            return volumes
        except (Boto3Error, RuntimeError) as e:
            msg = "Failed while querying instance '{}' volumes!: {}".format(
                provider_node_id, str(e))
            raise RuntimeError(msg)

    def get_security_group_name(self, security_group_id):
        security_group_filter = [{
            'Name': 'group-id', 'Values': [security_group_id]}]
        try:
            response = self._client.describe_security_groups(Filters=security_group_filter)
            security_groups = response.get('SecurityGroups', [])
            if len(security_groups) > 0:
                return security_groups[0]['GroupName']
        except Boto3Error as e:
            msg = "Failed while querying security group name for '{}' in region {}: {}".format(
                security_group_id, AWS_REGION, str(e))
            raise RuntimeError(msg)
