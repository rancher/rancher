
# RKE1 Provisioning Configs

For your config, you will need everything in the Prerequisites section on the previous readme, [Define your test](#Provisioning-Input), and at least one [Node Driver Cluster Template](#NodeTemplateConfigs) or [Custom Cluster Template](#Custom-Cluster), which should match what you have specified in `provisioningInput`. 

Your GO test_package should be set to `provisioning/rke1`.
Your GO suite should be set to `-run ^TestRKE1ProvisioningTestSuite$`.
Please see below for more details for your config. 

1. [Prerequisites](../README.md)
2. [Define your test](#Provisioning-Input)
3. [Configure providers to use for Node Driver Clusters](#NodeTemplateConfigs)
4. [Configuring Custom Clusters](#Custom-Cluster)
5. [Back to general provisioning](../README.md)

## Provisioning Input
provisioningInput is needed to the run the RKE1 tests, specifically kubernetesVersion, cni, and providers. nodesAndRoles is only needed for the TestProvisioningDynamicInput test, node pools are divided by "{nodepool},". 

**nodeProviders is only needed for custom cluster tests; the framework only supports custom clusters through aws/ec2 instances.**

```json
"provisioningInput": {
    "nodesAndRoles": [ 
      {
        "etcd": true,
        "controlplane": true,
        "worker": true,
        "quantity": 1,
      },
      {
        "worker": true,
        "quantity": 2,
      }
    ],
    "rke1KubernetesVersion": ["v1.24.2-rancher1-1"],
    "providers": ["linode", "aws", "azure", "harvester"],
    "nodeProviders": ["ec2"]
  }
```

## NodeTemplateConfigs
RKE1 specifically needs a node template config to run properly. These are the inputs needed for the different node providers.

### AWS
```json
  "awsNodeTemplate": {
    "accessKey": "",
    "ami": "",
    "blockDurationMinutes": "0",
    "encryptEbsVolume": false,
    "endpoint": "",
    "httpEndpoint": "enabled",
    "httpTokens": "optional",
    "iamInstanceProfile": "EngineeringUsersUS",
    "insecureTransport": false,
    "instanceType": "t2.2xlarge",
    "keypairName": "your-ssh-key",
    "kmsKey": "",
    "monitoring": false,
    "privateAddressOnly": false,
    "region": "us-east-2",
    "requestSpotInstance": true,
    "retries": "5",
    "rootSize": "16",
    "secretKey": "",
    "securityGroup": ["open-all"],
    "securityGroupReadonly": false,
    "sessionToken": "",
    "spotPrice": "0.50",
    "sshKeyContents": "",
    "sshUser": "ec2-user",
    "subnetId": "subnet-ee8cac86",
    "tags": "",
    "type": "amazonec2Config",
    "useEbsOptimizedInstance": false,
    "usePrivateAddress": false,
    "userdata": "",
    "volumeType": "gp2",
    "vpcId": "vpc-bfccf4d7",
    "zone": "a"
  }
```

### Azure
```json
  "azureNodeTemplate": {
    "availabilitySet": "docker-machine",
    "clientId": "",
    "clientSecret": "",
    "customData": "",
    "diskSize": "30",
    "dns": "",
    "dockerPort": "2376",
    "environment": "AzurePublicCloud",
    "faultDomainCount": "3",
    "image": "canonical:UbuntuServer:18.04-LTS:latest",
    "location": "eastus2",
    "managedDisks": false,
    "noPublicIp": false,
    "openPort": [
      "6443/tcp",
      "2379/tcp",
      "2380/tcp",
      "8472/udp",
      "4789/udp",
      "9796/tcp",
      "10256/tcp",
      "10250/tcp",
      "10251/tcp",
      "10252/tcp"
    ],
    "plan": "",
    "privateIpAddress": "",
    "resourceGroup": "",
    "size": "Standard_D2_v2",
    "sshUser": "azureuser",
    "staticPublicIp": false,
    "storageType": "Standard_LRS",
    "subnet": "docker-machine",
    "subnetPrefix": "192.168.0.0/16",
    "subscriptionId": "",
    "tenantId": "",
    "type": "azureConfig",
    "updateDomainCount": "5",
    "vnet": "docker-machine-vnet"
}
```

### Harvester
```json
"harvesterNodeTemplate": {
    "cloudConfig": "",
    "clusterId": "",
    "clusterType": "",
    "cpuCount": "2",
    "diskBus": "virtio",
    "diskSize": "40",
    "imageName": "default/image-gchq8",
    "keyPairName": "",
    "kubeconfigContent": "",
    "memorySize": "4",
    "networkData": "",
    "networkModel": "virtio",
    "networkName": "",
    "networkType": "dhcp",
    "sshPassword": "",
    "sshPort": "22",
    "sshPrivateKeyPath": "",
    "sshUser": "ubuntu",
    "type": "harvesterConfig",
    "userData": "",
    "vmAffinity": "",
    "vmNamespace": "default"
}
```

### Linode
```json
"linodeNodeTemplate:" { 
    "authorizedUsers": "",
    "createPrivateIp": true,
    "dockerPort": "2376",
    "image": "linode/ubuntu20.04",
    "instanceType": "g6-dedicated-8",
    "label": "",
    "region": "us-east",
    "rootPass": "",
    "sshPort": "22",
    "sshUser": "root",
    "stackscript": "",
    "stackscriptData": "",
    "swapSize": "512",
    "tags": "",
    "token": "",
    "type": "linodeConfig",
    "uaPrefix": "Rancher",
}
```

## Custom Cluster
For custom clusters, the below config is needed, only AWS/EC2. It is important to note that you MUST use an AMI that already has Docker installed and the service is enabled on boot; otherwise, this will not work.
**Ensure you have nodeProviders in provisioningInput**

```json
 "awsEC2Config": {
    "region": "us-east-2",
    "instanceType": "t3a.medium",
    "awsRegionAZ": "",
    "awsAMI": "",
    "awsSecurityGroups": [""],
    "awsAccessKeyID": "",
    "awsSecretAccessKey": "",
    "awsSSHKeyName": "",
    "awsCICDInstanceTag": "",
    "awsIAMProfile": "",
    "awsUser": "ubuntu",
    "volumeSize": 50
  },
```