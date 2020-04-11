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
AWS_HOSTED_ZONE_ID = os.environ.get("AWS_HOSTED_ZONE_ID", "")
AWS_VPC_ID = os.environ.get("AWS_VPC_ID", "vpc-bfccf4d7")
AWS_ACCESS_KEY_ID = os.environ.get("AWS_ACCESS_KEY_ID")
AWS_SECRET_ACCESS_KEY = os.environ.get("AWS_SECRET_ACCESS_KEY")
AWS_SSH_KEY_NAME = os.environ.get("AWS_SSH_KEY_NAME")
AWS_CICD_INSTANCE_TAG = os.environ.get("AWS_CICD_INSTANCE_TAG",
                                       'rancher-validation')
AWS_IAM_PROFILE = os.environ.get("AWS_IAM_PROFILE", "")
# by default the public Ubuntu 18.04 AMI is used
AWS_DEFAULT_AMI = "ami-0d5d9d301c853a04a"
AWS_DEFAULT_USER = "ubuntu"
AWS_AMI = os.environ.get("AWS_AMI", AWS_DEFAULT_AMI)
AWS_USER = os.environ.get("AWS_USER", AWS_DEFAULT_USER)
AWS_VOLUME_SIZE = os.environ.get("AWS_VOLUME_SIZE", "50")
AWS_INSTANCE_TYPE = os.environ.get("AWS_INSTANCE_TYPE", 't2.medium')

AWS_WINDOWS_VOLUME_SIZE = os.environ.get("AWS_WINDOWS_VOLUME_SIZE", "100")
AWS_WINDOWS_INSTANCE_TYPE = 't3.xlarge'


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

    def create_node(self, node_name, ami=AWS_AMI, ssh_user=AWS_USER,
                    key_name=None, wait_for_ready=True):
        volume_size = AWS_VOLUME_SIZE
        instance_type = AWS_INSTANCE_TYPE
        if ssh_user == "Administrator":
            volume_size = AWS_WINDOWS_VOLUME_SIZE
            instance_type = AWS_WINDOWS_INSTANCE_TYPE

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

        args = {"ImageId": ami,
                "InstanceType": instance_type,
                "MinCount": 1,
                "MaxCount": 1,
                "TagSpecifications": [
                    {'ResourceType': 'instance',
                     'Tags': [
                         {'Key': 'Name', 'Value': node_name},
                         {'Key': 'CICD', 'Value': AWS_CICD_INSTANCE_TAG}
                     ]}
                ],
                "KeyName": key_name,
                "NetworkInterfaces": [
                    {'DeviceIndex': 0,
                     'AssociatePublicIpAddress': True,
                     'Groups': [AWS_SECURITY_GROUP]
                     }
                ],
                "Placement": {'AvailabilityZone': AWS_REGION_AZ},
                "BlockDeviceMappings":
                    [{"DeviceName": "/dev/sda1",
                      "Ebs": {"VolumeSize": int(volume_size)}
                      }]
                }

        if len(AWS_IAM_PROFILE) > 0:
            args["IamInstanceProfile"] = {'Name': AWS_IAM_PROFILE}

        instance = self._client.run_instances(**args)
        node = Node(
            node_name=node_name,
            provider_node_id=instance['Instances'][0]['InstanceId'],
            state=instance['Instances'][0]['State']['Name'],
            ssh_user=ssh_user,
            ssh_key_name=ssh_private_key_name,
            ssh_key_path=ssh_private_key_path,
            ssh_key=ssh_private_key,
            docker_version=self.DOCKER_VERSION,
            docker_installed=self.DOCKER_INSTALLED)

        # mark for clean up at the end
        self.created_node.append(node.provider_node_id)

        if wait_for_ready:
            node = self.wait_for_node_state(node)
            node.ready_node()
        return node

    def create_multiple_nodes(self, number_of_nodes, node_name_prefix,
                              ami=AWS_AMI, ssh_user=AWS_USER,
                              key_name=None, wait_for_ready=True):
        nodes = []
        for i in range(number_of_nodes):
            node_name = "{}-{}".format(node_name_prefix, i)
            nodes.append(self.create_node(node_name,
                                          ami=ami, ssh_user=ssh_user,
                                          key_name=key_name,
                                          wait_for_ready=False))

        if wait_for_ready:
            nodes = self.wait_for_nodes_state(nodes)
            # hack for instances
            if self.DOCKER_INSTALLED.lower() == 'true':
                time.sleep(5)
                self.reboot_nodes(nodes)
                time.sleep(10)
                nodes = self.wait_for_nodes_state(nodes)

            # wait for window nodes to come up so we can decrypt the password
            if ssh_user == "Administrator":
                time.sleep(60 * 6)
                for node in nodes:
                    node.ssh_password = \
                        self.decrypt_windows_password(node.provider_node_id)

            for node in nodes:
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
        response = client.list_objects(
            Bucket=os.environ.get("AWS_S3_BUCKET_NAME", ""),
            Prefix=os.environ.get("AWS_S3_BUCKET_FOLDER_NAME", ""))

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
        password_data = self._client. \
            get_password_data(InstanceId=instance_id)['PasswordData']
        if password_data:
            password = base64.b64decode(password_data)
            with open(self.get_ssh_key_path(AWS_SSH_KEY_NAME), 'r') \
                    as privkeyfile:
                priv = rsa.PrivateKey.load_pkcs1(privkeyfile.read())
                password = rsa.decrypt(password, priv).decode('utf-8')

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
        sg_filter = [{
            'Name': 'group-id', 'Values': [security_group_id]}]
        try:
            response = self._client.describe_security_groups(Filters=sg_filter)
            security_groups = response.get('SecurityGroups', [])
            if len(security_groups) > 0:
                return security_groups[0]['GroupName']
        except Boto3Error as e:
            msg = "Failed while querying security group name for '{}' " \
                  "in region {}: {}".format(security_group_id,
                                            AWS_REGION, str(e))
            raise RuntimeError(msg)
