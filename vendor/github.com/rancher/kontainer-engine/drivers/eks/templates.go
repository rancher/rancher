package eks

// These are the CloudFormation templates used for EKS clusters, when making edits here ensure the whitespace is correct.

const (
	vpcTemplate = `---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Sample VPC'

Parameters:

  VpcBlock:
    Type: String
    Default: 192.168.0.0/16
    Description: The CIDR range for the VPC. This should be a valid private (RFC 1918) CIDR range.

  Subnet01Block:
    Type: String
    Default: 192.168.64.0/18
    Description: CidrBlock for subnet 01 within the VPC

  Subnet02Block:
    Type: String
    Default: 192.168.128.0/18
    Description: CidrBlock for subnet 02 within the VPC

  Subnet03Block:
    Type: String
    Default: 192.168.192.0/18
    Description: CidrBlock for subnet 03 within the VPC. This is used only if the region has more than 2 AZs.

Metadata:
  AWS::CloudFormation::Interface:
    ParameterGroups:
      -
        Label:
          default: "Worker Network Configuration"
        Parameters:
          - VpcBlock
          - Subnet01Block
          - Subnet02Block
          - Subnet03Block

Conditions:
  Has2Azs:
    Fn::Or:
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - ap-south-1
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - ap-northeast-2
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - ca-central-1
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - cn-north-1

  HasMoreThan2Azs:
    Fn::Not:
      - Condition: Has2Azs

Resources:
  VPC:
    Type: AWS::EC2::VPC
    Properties:
      CidrBlock:  !Ref VpcBlock
      EnableDnsSupport: true
      EnableDnsHostnames: true
      Tags:
      - Key: Name
        Value: !Sub '${AWS::StackName}-VPC'

  InternetGateway:
    Type: "AWS::EC2::InternetGateway"

  VPCGatewayAttachment:
    Type: "AWS::EC2::VPCGatewayAttachment"
    Properties:
      InternetGatewayId: !Ref InternetGateway
      VpcId: !Ref VPC

  RouteTable:
    Type: AWS::EC2::RouteTable
    Properties:
      VpcId: !Ref VPC
      Tags:
      - Key: Name
        Value: Public Subnets
      - Key: Network
        Value: Public

  Route:
    DependsOn: VPCGatewayAttachment
    Type: AWS::EC2::Route
    Properties:
      RouteTableId: !Ref RouteTable
      DestinationCidrBlock: 0.0.0.0/0
      GatewayId: !Ref InternetGateway

  Subnet01:
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 01
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '0'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: Subnet01Block
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet01"

  Subnet02:
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 02
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '1'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: Subnet02Block
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet02"

  Subnet03:
    Condition: HasMoreThan2Azs
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 03
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '2'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: Subnet03Block
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet03"

  Subnet01RouteTableAssociation:
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet01
      RouteTableId: !Ref RouteTable

  Subnet02RouteTableAssociation:
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet02
      RouteTableId: !Ref RouteTable

  Subnet03RouteTableAssociation:
    Condition: HasMoreThan2Azs
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet03
      RouteTableId: !Ref RouteTable

  ControlPlaneSecurityGroup:
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupDescription: Cluster communication with worker nodes
      VpcId: !Ref VPC

Outputs:

  SubnetIds:
    Description: All subnets in the VPC
    Value:
      Fn::If:
      - HasMoreThan2Azs
      - !Join [ ",", [ !Ref Subnet01, !Ref Subnet02, !Ref Subnet03 ] ]
      - !Join [ ",", [ !Ref Subnet01, !Ref Subnet02 ] ]

  SecurityGroups:
    Description: Security group for the cluster control plane communication with worker nodes
    Value: !Join [ ",", [ !Ref ControlPlaneSecurityGroup ] ]

  VpcId:
    Description: The VPC Id
    Value: !Ref VPC
`
	workerNodesTemplate = `---
AWSTemplateFormatVersion: 2010-09-09
Description: Amazon EKS - Node Group

Parameters:

  KeyName:
    Description: The EC2 Key Pair to allow SSH access to the instances
    Type: AWS::EC2::KeyPair::KeyName

  NodeImageId:
    Description: AMI id for the node instances.
    Type: AWS::EC2::Image::Id

  NodeInstanceType:
    Description: EC2 instance type for the node instances
    Type: String
    Default: t3.medium
    ConstraintDescription: Must be a valid EC2 instance type
    AllowedValues:
      - c1.medium
      - c1.xlarge
      - c3.large
      - c3.xlarge
      - c3.2xlarge
      - c3.4xlarge
      - c3.8xlarge
      - c4.large
      - c4.xlarge
      - c4.2xlarge
      - c4.4xlarge
      - c4.8xlarge
      - c5.large
      - c5.xlarge
      - c5.2xlarge
      - c5.4xlarge
      - c5.9xlarge
      - c5.12xlarge
      - c5.18xlarge
      - c5.24xlarge
      - c5.metal
      - c5d.large
      - c5d.xlarge
      - c5d.2xlarge
      - c5d.4xlarge
      - c5d.9xlarge
      - c5d.18xlarge
      - c5n.large
      - c5n.xlarge
      - c5n.2xlarge
      - c5n.4xlarge
      - c5n.9xlarge
      - c5n.18xlarge
      - cc2.8xlarge
      - cr1.8xlarge
      - d2.xlarge
      - d2.2xlarge
      - d2.4xlarge
      - d2.8xlarge
      - f1.2xlarge
      - f1.4xlarge
      - f1.16xlarge
      - g2.2xlarge
      - g2.8xlarge
      - g3s.xlarge
      - g3.4xlarge
      - g3.8xlarge
      - g3.16xlarge
      - h1.2xlarge
      - h1.4xlarge
      - h1.8xlarge
      - h1.16xlarge
      - hs1.8xlarge
      - i2.xlarge
      - i2.2xlarge
      - i2.4xlarge
      - i2.8xlarge
      - i3.large
      - i3.xlarge
      - i3.2xlarge
      - i3.4xlarge
      - i3.8xlarge
      - i3.16xlarge
      - i3.metal
      - i3en.large
      - i3en.xlarge
      - i3en.2xlarge
      - i3en.3xlarge
      - i3en.6xlarge
      - i3en.12xlarge
      - i3en.24xlarge
      - m1.small
      - m1.medium
      - m1.large
      - m1.xlarge
      - m2.xlarge
      - m2.2xlarge
      - m2.4xlarge
      - m3.medium
      - m3.large
      - m3.xlarge
      - m3.2xlarge
      - m4.large
      - m4.xlarge
      - m4.2xlarge
      - m4.4xlarge
      - m4.10xlarge
      - m4.16xlarge
      - m5.large
      - m5.xlarge
      - m5.2xlarge
      - m5.4xlarge
      - m5.8xlarge
      - m5.12xlarge
      - m5.16xlarge
      - m5.24xlarge
      - m5.metal
      - m5a.large
      - m5a.xlarge
      - m5a.2xlarge
      - m5a.4xlarge
      - m5a.8xlarge
      - m5a.12xlarge
      - m5a.16xlarge
      - m5a.24xlarge
      - m5ad.large
      - m5ad.xlarge
      - m5ad.2xlarge
      - m5ad.4xlarge
      - m5ad.12xlarge
      - m5ad.24xlarge
      - m5d.large
      - m5d.xlarge
      - m5d.2xlarge
      - m5d.4xlarge
      - m5d.8xlarge
      - m5d.12xlarge
      - m5d.16xlarge
      - m5d.24xlarge
      - m5d.metal
      - p2.xlarge
      - p2.8xlarge
      - p2.16xlarge
      - p3.2xlarge
      - p3.8xlarge
      - p3.16xlarge
      - p3dn.24xlarge
      - g4dn.xlarge
      - g4dn.2xlarge
      - g4dn.4xlarge
      - g4dn.8xlarge
      - g4dn.12xlarge
      - g4dn.16xlarge
      - g4dn.metal
      - r3.large
      - r3.xlarge
      - r3.2xlarge
      - r3.4xlarge
      - r3.8xlarge
      - r4.large
      - r4.xlarge
      - r4.2xlarge
      - r4.4xlarge
      - r4.8xlarge
      - r4.16xlarge
      - r5.large
      - r5.xlarge
      - r5.2xlarge
      - r5.4xlarge
      - r5.8xlarge
      - r5.12xlarge
      - r5.16xlarge
      - r5.24xlarge
      - r5.metal
      - r5a.large
      - r5a.xlarge
      - r5a.2xlarge
      - r5a.4xlarge
      - r5a.8xlarge
      - r5a.12xlarge
      - r5a.16xlarge
      - r5a.24xlarge
      - r5ad.large
      - r5ad.xlarge
      - r5ad.2xlarge
      - r5ad.4xlarge
      - r5ad.12xlarge
      - r5ad.24xlarge
      - r5d.large
      - r5d.xlarge
      - r5d.2xlarge
      - r5d.4xlarge
      - r5d.8xlarge
      - r5d.12xlarge
      - r5d.16xlarge
      - r5d.24xlarge
      - r5d.metal
      - t1.micro
      - t2.nano
      - t2.micro
      - t2.small
      - t2.medium
      - t2.large
      - t2.xlarge
      - t2.2xlarge
      - t3.nano
      - t3.micro
      - t3.small
      - t3.medium
      - t3.large
      - t3.xlarge
      - t3.2xlarge
      - t3a.nano
      - t3a.micro
      - t3a.small
      - t3a.medium
      - t3a.large
      - t3a.xlarge
      - t3a.2xlarge
      - u-6tb1.metal
      - u-9tb1.metal
      - u-12tb1.metal
      - x1.16xlarge
      - x1.32xlarge
      - x1e.xlarge
      - x1e.2xlarge
      - x1e.4xlarge
      - x1e.8xlarge
      - x1e.16xlarge
      - x1e.32xlarge
      - z1d.large
      - z1d.xlarge
      - z1d.2xlarge
      - z1d.3xlarge
      - z1d.6xlarge
      - z1d.12xlarge
      - z1d.metal

  NodeAutoScalingGroupMinSize:
    Description: Minimum size of Node Group ASG.
    Type: Number
    Default: 1

  NodeAutoScalingGroupMaxSize:
    Description: Maximum size of Node Group ASG. Set to at least 1 greater than NodeAutoScalingGroupDesiredCapacity.
    Type: Number
    Default: 4

  NodeAutoScalingGroupDesiredCapacity:
    Description: Desired capacity of Node Group ASG.
    Type: Number
    Default: 3

  NodeVolumeSize:
    Description: Node volume size
    Type: Number
    Default: 20

  ClusterName:
    Description: The cluster name provided when the cluster was created. If it is incorrect, nodes will not be able to join the cluster.
    Type: String

  BootstrapArguments:
    Description: Arguments to pass to the bootstrap script. See files/bootstrap.sh in https://github.com/awslabs/amazon-eks-ami
    Type: String
    Default: ""

  NodeGroupName:
    Description: Unique identifier for the Node Group.
    Type: String

  ClusterControlPlaneSecurityGroup:
    Description: The security group of the cluster control plane.
    Type: AWS::EC2::SecurityGroup::Id

  VpcId:
    Description: The VPC of the worker instances
    Type: AWS::EC2::VPC::Id

  Subnets:
    Description: The subnets where workers can be created.
    Type: List<AWS::EC2::Subnet::Id>

  PublicIp:
    Description: Associate the public IP addresses of the worker nodes
    Type: String
    Default: "true"

  EBSEncryption:
    Description: Encrypt EBS volumes of worker nodes
    Type: String
    Default: "false"

Metadata:

  AWS::CloudFormation::Interface:
    ParameterGroups:
      - Label:
          default: EKS Cluster
        Parameters:
          - ClusterName
          - ClusterControlPlaneSecurityGroup
      - Label:
          default: Worker Node Configuration
        Parameters:
          - NodeGroupName
          - NodeAutoScalingGroupMinSize
          - NodeAutoScalingGroupDesiredCapacity
          - NodeAutoScalingGroupMaxSize
          - NodeInstanceType
          - NodeImageId
          - NodeVolumeSize
          - KeyName
          - BootstrapArguments
      - Label:
          default: Worker Network Configuration
        Parameters:
          - VpcId
          - Subnets

Resources:

  NodeInstanceProfile:
    Type: AWS::IAM::InstanceProfile
    Properties:
      Path: "/"
      Roles:
        - !Ref NodeInstanceRole

  NodeInstanceRole:
    Type: AWS::IAM::Role
    Properties:
      AssumeRolePolicyDocument:
        Version: 2012-10-17
        Statement:
          - Effect: Allow
            Principal:
              Service: %s
            Action: sts:AssumeRole
      Path: "/"
      ManagedPolicyArns:
        - arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy
        - arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy
        - arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly

  NodeSecurityGroup:
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupDescription: Security group for all nodes in the cluster
      VpcId: !Ref VpcId
      Tags:
        - Key: !Sub kubernetes.io/cluster/${ClusterName}
          Value: owned

  NodeSecurityGroupIngress:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow node to communicate with each other
      GroupId: !Ref NodeSecurityGroup
      SourceSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: -1
      FromPort: 0
      ToPort: 65535

  NodeSecurityGroupFromControlPlaneIngress:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow worker Kubelets and pods to receive communication from the cluster control plane
      GroupId: !Ref NodeSecurityGroup
      SourceSecurityGroupId: !Ref ClusterControlPlaneSecurityGroup
      IpProtocol: tcp
      FromPort: 1025
      ToPort: 65535

  ControlPlaneEgressToNodeSecurityGroup:
    Type: AWS::EC2::SecurityGroupEgress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow the cluster control plane to communicate with worker Kubelet and pods
      GroupId: !Ref ClusterControlPlaneSecurityGroup
      DestinationSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      FromPort: 1025
      ToPort: 65535

  NodeSecurityGroupFromControlPlaneOn443Ingress:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow pods running extension API servers on port 443 to receive communication from cluster control plane
      GroupId: !Ref NodeSecurityGroup
      SourceSecurityGroupId: !Ref ClusterControlPlaneSecurityGroup
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  ControlPlaneEgressToNodeSecurityGroupOn443:
    Type: AWS::EC2::SecurityGroupEgress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow the cluster control plane to communicate with pods running extension API servers on port 443
      GroupId: !Ref ClusterControlPlaneSecurityGroup
      DestinationSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  ClusterControlPlaneSecurityGroupIngress:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow pods to communicate with the cluster API Server
      GroupId: !Ref ClusterControlPlaneSecurityGroup
      SourceSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      ToPort: 443
      FromPort: 443

  NodeSecurityGroupFromControlPlaneOn80Ingress:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow pods to receive communication from cluster control plane via HTTP service proxy on port 80
      GroupId: !Ref NodeSecurityGroup
      SourceSecurityGroupId: !Ref ClusterControlPlaneSecurityGroup
      IpProtocol: tcp
      FromPort: 80
      ToPort: 80

  ControlPlaneEgressToNodeSecurityGroupOn80:
    Type: AWS::EC2::SecurityGroupEgress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow the cluster control plane to communicate with pods via HTTP service proxy on port 80
      GroupId: !Ref ClusterControlPlaneSecurityGroup
      DestinationSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      FromPort: 80
      ToPort: 80

  NodeGroup:
    Type: AWS::AutoScaling::AutoScalingGroup
    Properties:
      DesiredCapacity: !Ref NodeAutoScalingGroupDesiredCapacity
      LaunchConfigurationName: !Ref NodeLaunchConfig
      MinSize: !Ref NodeAutoScalingGroupMinSize
      MaxSize: !Ref NodeAutoScalingGroupMaxSize
      VPCZoneIdentifier: !Ref Subnets
      Tags:
        - Key: Name
          Value: !Sub ${ClusterName}-${NodeGroupName}-Node
          PropagateAtLaunch: true
        - Key: !Sub kubernetes.io/cluster/${ClusterName}
          Value: owned
          PropagateAtLaunch: true
    UpdatePolicy:
      AutoScalingRollingUpdate:
        MaxBatchSize: 1
        MinInstancesInService: !Ref NodeAutoScalingGroupDesiredCapacity
        PauseTime: PT5M

  NodeLaunchConfig:
    Type: AWS::AutoScaling::LaunchConfiguration
    Properties:
      AssociatePublicIpAddress: !Ref PublicIp
      IamInstanceProfile: !Ref NodeInstanceProfile
      ImageId: !Ref NodeImageId
      InstanceType: !Ref NodeInstanceType
      KeyName: !Ref KeyName
      SecurityGroups:
        - !Ref NodeSecurityGroup
      BlockDeviceMappings:
        - DeviceName: /dev/xvda
          Ebs:
            VolumeSize: !Ref NodeVolumeSize
            VolumeType: gp2
            DeleteOnTermination: true
            Encrypted: !Ref EBSEncryption
      UserData: !Base64
        'Fn::Sub': %q

Outputs:

  NodeInstanceRole:
    Description: The node instance role
    Value: !GetAtt NodeInstanceRole.Arn

  NodeSecurityGroup:
    Description: The security group for the node group
    Value: !Ref NodeSecurityGroup
`
	serviceRoleTemplate = `---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Service Role'


Resources:

  AWSServiceRoleForAmazonEKS:
    Type: AWS::IAM::Role
    Properties:
      AssumeRolePolicyDocument:
        Version: '2012-10-17'
        Statement:
        - Effect: Allow
          Principal:
            Service:
            - eks.amazonaws.com
          Action:
          - sts:AssumeRole
      ManagedPolicyArns:
        - arn:aws:iam::aws:policy/AmazonEKSServicePolicy
        - arn:aws:iam::aws:policy/AmazonEKSClusterPolicy

Outputs:

  RoleArn:
    Description: The role that EKS will use to create AWS resources for Kubernetes clusters
    Value: !GetAtt AWSServiceRoleForAmazonEKS.Arn
    Export:
      Name: !Sub "${AWS::StackName}-RoleArn"

`
)
