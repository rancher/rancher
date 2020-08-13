import base64
import boto3
import logging
import os
import rsa
import time
from boto3.exceptions import Boto3Error
from botocore.exceptions import ClientError
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
AWS_INSTANCE_TYPE = os.environ.get("AWS_INSTANCE_TYPE", 't3a.medium')

AWS_WINDOWS_VOLUME_SIZE = os.environ.get("AWS_WINDOWS_VOLUME_SIZE", "100")
AWS_WINDOWS_INSTANCE_TYPE = 't3.xlarge'

EKS_VERSION = os.environ.get("RANCHER_EKS_K8S_VERSION")
EKS_ROLE_ARN = os.environ.get("RANCHER_EKS_ROLE_ARN")
EKS_WORKER_ROLE_ARN = os.environ.get("RANCHER_EKS_WORKER_ROLE_ARN")

AWS_SUBNETS = []
if ',' in AWS_SUBNET:
    AWS_SUBNETS = AWS_SUBNET.split(',')
else:
    AWS_SUBNETS = [AWS_SUBNET]


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

        self._db_client = boto3.client(
            'rds',
            aws_access_key_id=AWS_ACCESS_KEY_ID,
            aws_secret_access_key=AWS_SECRET_ACCESS_KEY,
            region_name=AWS_REGION)

        self._eks_client = boto3.client(
            'eks',
            aws_access_key_id=AWS_ACCESS_KEY_ID,
            aws_secret_access_key=AWS_SECRET_ACCESS_KEY,
            region_name=AWS_REGION)

        self.master_ssh_key = None
        self.master_ssh_key_path = None

        if AWS_SSH_KEY_NAME:
            self.master_ssh_key = self.get_ssh_key(AWS_SSH_KEY_NAME)
            self.master_ssh_key_path = self.get_ssh_key_path(AWS_SSH_KEY_NAME)

        # Used for cleanup
        self.created_node = []
        self.created_keys = []

    def create_node(self, node_name, ami=AWS_AMI, ssh_user=AWS_USER,
                    key_name=None, wait_for_ready=True, public_ip=True):
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
                     'AssociatePublicIpAddress': public_ip,
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
            if public_ip:
                node.ready_node()
            else:
                time.sleep(60)
        return node

    def create_multiple_nodes(self, number_of_nodes, node_name_prefix,
                              ami=AWS_AMI, ssh_user=AWS_USER,
                              key_name=None, wait_for_ready=True,
                              public_ip=True):
        nodes = []
        for i in range(number_of_nodes):
            node_name = "{}-{}".format(node_name_prefix, i)
            nodes.append(self.create_node(node_name,
                                          ami=ami, ssh_user=ssh_user,
                                          key_name=key_name,
                                          wait_for_ready=False,
                                          public_ip=public_ip))

        if wait_for_ready:
            nodes = self.wait_for_nodes_state(nodes)
            # wait for window nodes to come up so we can decrypt the password
            if ssh_user == "Administrator":
                time.sleep(60 * 6)
                for node in nodes:
                    node.ssh_password = \
                        self.decrypt_windows_password(node.provider_node_id)

            if public_ip:
                for node in nodes:
                    node.ready_node()
            else:
                time.sleep(60)

        return nodes

    def get_node(self, provider_id, ssh_access=False):
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
            if ssh_access:
                node.ssh_user = AWS_USER
                node.ssh_key_name = AWS_SSH_KEY_NAME.replace('.pem', '')
                node.ssh_key_path = self.master_ssh_key_path
                node.ssh_key = self.master_ssh_key
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

    def delete_eks_cluster(self, cluster_name):
        ng_names = self._eks_client.list_nodegroups(clusterName=cluster_name)
        for node_group in ng_names['nodegroups']:
            print("Deleting node group: " + node_group)
            delete_ng_response = self._eks_client.delete_nodegroup(
                                                    clusterName=cluster_name,
                                                    nodegroupName=node_group)
        waiter_ng = self._eks_client.get_waiter('nodegroup_deleted')
        for node_group in ng_names['nodegroups']:
            print("Waiting for deletion of: " + node_group)
            waiter_ng.wait(clusterName=cluster_name, nodegroupName=node_group)
        print("Deleting cluster: "+ cluster_name)
        delete_response = self._eks_client.delete_cluster(name=cluster_name)
        return delete_response

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

    def create_network_lb(self, name, scheme='internet-facing'):
        return self._elbv2_client.create_load_balancer(
            Name=name, Subnets=[AWS_SUBNET], Scheme=scheme, Type='network'
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

    def upsert_route_53_record_cname(
            self, record_name, record_value, action='UPSERT',
            record_type='CNAME', record_ttl=300):
        return self._route53_client.change_resource_record_sets(
            HostedZoneId=AWS_HOSTED_ZONE_ID,
            ChangeBatch={
                'Comment': 'Record created or updated for automation',
                'Changes': [{
                    'Action': action,
                    'ResourceRecordSet': {
                        'Name': record_name,
                        'Type': record_type,
                        'TTL': record_ttl,
                        'ResourceRecords': [{
                            'Value': record_value
                        }]
                    }
                }]
            }
        )

    def delete_route_53_record(self, record_name):
        record = None
        try:
            res = self._route53_client.list_resource_record_sets(
                HostedZoneId=AWS_HOSTED_ZONE_ID,
                StartRecordName=record_name,
                StartRecordType='CNAME',
                MaxItems='1')
            if len(res["ResourceRecordSets"]) > 0:
                record = res["ResourceRecordSets"][0]
        except ClientError as e:
            print(e.response)

        if record is not None and record["Name"] == record_name:
            self._route53_client.change_resource_record_sets(
                HostedZoneId=AWS_HOSTED_ZONE_ID,
                ChangeBatch={
                    'Comment': 'delete record',
                    'Changes': [{
                        'Action': 'DELETE',
                        'ResourceRecordSet': record}]
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

    def get_target_groups(self, lb_arn):
        tg_list = []
        try:
            res = self._elbv2_client.describe_listeners(
                LoadBalancerArn=lb_arn)
        except ClientError:
            return tg_list
        if res is not None:
            for item in res["Listeners"]:
                tg_arn = item["DefaultActions"][0]["TargetGroupArn"]
                tg_list.append(tg_arn)
        return tg_list

    def get_lb(self, name):
        try:
            res = self._elbv2_client.describe_load_balancers(Names=[name])
            return res['LoadBalancers'][0]['LoadBalancerArn']
        except ClientError:
            return None

    def get_db(self, db_id):
        try:
            res = self._db_client.\
                describe_db_instances(DBInstanceIdentifier=db_id)
            return res['DBInstances'][0]['DBInstanceIdentifier']
        except ClientError:
            return None

    def delete_db(self, db_id):
        try:
            self._db_client.delete_db_instance(DBInstanceIdentifier=db_id,
                                               SkipFinalSnapshot=True,
                                               DeleteAutomatedBackups=True)
        except ClientError:
            return None

    def create_eks_cluster(self, name):
        kubeconfig_path = self.create_eks_controlplane(name)
        self.create_eks_nodegroup(name, '{}-ng'.format(name))
        return kubeconfig_path

    def create_eks_controlplane(self, name):
        vpcConfiguration = {
            "subnetIds": AWS_SUBNETS,
            "securityGroupIds": [AWS_SECURITY_GROUP],
            "endpointPublicAccess": True,
            "endpointPrivateAccess": False
        }

        self._eks_client.\
            create_cluster(name=name,
                           version=EKS_VERSION,
                           roleArn=EKS_ROLE_ARN,
                           resourcesVpcConfig=vpcConfiguration)

        return self.wait_for_eks_cluster_state(name, "ACTIVE")

    def create_eks_nodegroup(self, cluster_name, name):
        scaling_config = {
            "minSize": 3,
            "maxSize": 3,
            "desiredSize": 3
        }

        remote_access = {
            "ec2SshKey": AWS_SSH_KEY_NAME.replace('.pem', '')
        }

        ng = self._eks_client.\
            create_nodegroup(clusterName=cluster_name,
                             nodegroupName=name,
                             scalingConfig=scaling_config,
                             diskSize=20,
                             subnets=AWS_SUBNETS,
                             instanceTypes=[AWS_INSTANCE_TYPE],
                             nodeRole=EKS_WORKER_ROLE_ARN,
                             remoteAccess=remote_access)
        waiter_ng = self._eks_client.get_waiter('nodegroup_active')
        waiter_ng.wait(clusterName=cluster_name, nodegroupName=name)
        return ng

    def describe_eks_cluster(self, name):
        try:
            return self._eks_client.describe_cluster(name=name)
        except ClientError:
            return None

    def describe_eks_nodegroup(self, cluster_name, nodegroup_name):
        try:
            return self._eks_client.describe_nodegroup(
                clusterName=cluster_name,
                nodegroupName=nodegroup_name
            )
        except ClientError:
            return None

    def wait_for_eks_cluster_state(self, name, target_state, timeout=1200):
        start = time.time()
        cluster = self.describe_eks_cluster(name)['cluster']
        status = cluster['status']
        while status != target_state:
            if time.time() - start > timeout:
                raise AssertionError(
                    "Timed out waiting for state to get to " + target_state)

            time.sleep(5)
            cluster = self.describe_eks_cluster(name)['cluster']
            status = cluster['status']
            print(status)
        return cluster

    def wait_for_delete_eks_cluster(self, cluster_name):
        ng_names = self._eks_client.list_nodegroups(clusterName=cluster_name)
        waiter_ng = self._eks_client.get_waiter('nodegroup_deleted')
        for node_group in ng_names['nodegroups']:
            print ("Waiting for deletion of nodegroup: {}".format(node_group))
            waiter_ng.wait(clusterName=cluster_name, nodegroupName=node_group)
        print ("Waiting for deletion of cluster: {}".format(cluster_name))
        waiter = self._eks_client.get_waiter('cluster_deleted')
        waiter.wait(name=cluster_name)
