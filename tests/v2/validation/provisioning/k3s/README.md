
# K3S Provisioning Configs

For your config, you will need everything in the Prerequisites section on the previous readme, [Define your test](#provisioning-input), and at least one [Cloud Credential](#cloud-credentials) and [Node Driver Machine Config](#machine-k3s-config) or [Custom Cluster Template](#custom-cluster), which should match what you have specified in `provisioningInput`. 

Your GO test_package should be set to `provisioning/k3s`.
Your GO suite should be set to `-run ^TestK3SProvisioningTestSuite$`.
Please see below for more details for your config. 

1. [Prerequisites](../README.md)
2. [Define your test](#provisioning-input)
3. [Cloud Credential](#cloud-credentials)
4. [Configure providers to use for Node Driver Clusters](#machine-k3s-config)
5. [Configuring Custom Clusters](#custom-cluster)
6. [Advanced Cluster Settings](#advanced-settings)
7. [Back to general provisioning](../README.md)

## Provisioning Input
provisioningInput is needed to the run the K3S tests, specifically kubernetesVersion and providers. nodesAndRoles is only needed for the TestProvisioningDynamicInput test, node pools are divided by "{nodepool},". psact is optional and takes values `rancher-privileged` and `rancher-restricted` only.

**nodeProviders is only needed for custom cluster tests; the framework only supports custom clusters through aws/ec2 instances.**

```json
"provisioningInput": {
    // for custom clusters, len(nodesAndRoles) should be equal to len(awsEc2Config)
        "machinePools": [
      {
        "nodeRoles": {
          "etcd": true,
          "controlplane": true,
          "worker": false,
          "quantity": 1,
        },
      },
      {
        "nodeRoles": {
          "worker": true,
          "quantity": 2,
        },
        "nodeLabels" {
          "label1": "value1",
          "label2": "value2",
        },
        "nodeTaints" [
          { "key": "TestKey",
            "value": "testValue",
            "effect": "NoSchedule",
          }
        ],
        "specifyPrivateIP": false,
        "specifyPublicIP": true,
        "nodeNamePrefix": "qa",
      },
    ],
    "k3sKubernetesVersion": ["v1.26.8+k3s1"],
    "providers": ["linode", "aws", "azure", "harvester"],
    "nodeProviders": ["ec2"],
    "hardened": true,
    "psact": ""
  }
```

## Cloud Credentials
These are the inputs needed for the different node provider cloud credentials, inlcuding linode, aws, harvester, azure, and google.

### Linode
```json
"linodeCredentials": {
   "token": ""
  },
```
### Azure
```json
"azureCredentials": {
   "clientId": "",
   "clientSecret": "",
     "subscriptionId": "",
     "environment": "AzurePublicCloud"
  },
```
### AWS
```json
"awsCredentials": {
   "secretKey": "",
   "accessKey": "",
   "defaultRegion": ""
  },
```
### Harvester
```json
"harvesterCredentials": {
   "clusterId": "",
   "clusterType": "",
   "kubeconfigContent": ""
},
```
### Google
```json
"googleCredentials": {
    "authEncodedJson": ""
},
```
### VSphere
```json
"vmwarevsphereCredentials": {
  "password": "",
  "username": "",
  "vcenter": "",
  "vcenterPort": "",
}
```

## Machine K3S Config
Machine K3S config is the final piece needed for the config to run K3S provisioning tests.

### AWS K3S Machine Config
```json
"awsMachineConfig": {
    "region": "us-east-2",
    "ami": "",
    "instanceType": "t3a.medium",
    "sshUser": "ubuntu",
    "vpcId": "",
    "volumeType": "gp2",
    "zone": "a",
    "retries": "5",
    "rootSize": "16",
    "securityGroup": ["rancher-nodes"]
},
```
### Linode K3S Machine Config
```json
"linodeMachineConfig": {
  "authorizedUsers": "",
  "createPrivateIp": false,
  "dockerPort": "2376",
  "image": "linode/ubuntu20.04",
  "instanceType": "g6-standard-2",
  "region": "us-west",
  "rootPass": "",
  "sshPort": "22",
  "sshUser": "",
  "stackscript": "",
  "stackscriptData": "",
  "swapSize": "512",
  "tags": "",
  "uaPrefix": ""
},
```
### Azure K3S Machine Config
```json
"azureMachineConfig": {
  "availabilitySet": "docker-machine",
  "diskSize": "30",
  "environment": "AzurePublicCloud",
  "faultDomainCount": "3",
  "image": "canonical:UbuntuServer:18.04-LTS:latest",
  "location": "westus",
  "managedDisks": false,
  "noPublicIp": false,
  "nsg": "",
  "openPort": ["6443/tcp", "2379/tcp", "2380/tcp", "8472/udp", "4789/udp", "9796/tcp", "10256/tcp", "10250/tcp", "10251/tcp", "10252/tcp"],
  "resourceGroup": "docker-machine",
  "size": "Standard_D2_v2",
  "sshUser": "docker-user",
  "staticPublicIp": false,
  "storageType": "Standard_LRS",
  "subnet": "docker-machine",
  "subnetPrefix": "192.168.0.0/16",
  "updateDomainCount": "5",
  "usePrivateIp": false,
  "vnet": "docker-machine-vnet"
},
```
### Harvester K3S Machine Config
```json
"harvesterMachineConfig": {
  "diskSize": "40",
  "cpuCount": "2",
  "memorySize": "8",
  "networkName": "default/ctw-network-1",
  "imageName": "default/image-rpj98",
  "vmNamespace": "default",
  "sshUser": "ubuntu",
  "diskBus": "virtio"
},
```
### Vsphere K3S Machine Config
```json
"vmwarevsphereMachineConfig": {
  "cfgparam": ["disk.enableUUID=TRUE"],
  "cloneFrom": "",
  "cloudinit": "",
  "contentLibrary": "",
  "cpuCount": "4",
  "creationType": "",
  "datacenter": "",
  "datastore": "",
  "datastoreCluster": "",
  "diskSize": "20000",
  "folder": "",
  "hostSystem": "",
  "memorySize": "4096",
  "network": [""],
  "os": "linux",
  "password": "",
  "pool": "",
  "sshPassword": "",
  "sshPort": "22",
  "sshUser": "",
  "sshUserGroup": "",
  "username": "",
  "vcenter": "",
  "vcenterPort": "443"
}
```

## Custom Cluster
For custom clusters, no machineConfig or credentials are needed. Currently only supported for ec2.

Dependencies:
* **Ensure you have machinePools in provisioningInput**
* make sure that all roles are entered at least once in machinePools.nodeRoles
* ensure that nodeProviders is set

```json
{
  "awsEC2Configs": {
    "region": "us-east-2",
    "awsSecretAccessKey": "",
    "awsAccessKeyID": "",
    "awsEC2Config": [
      {
        "instanceType": "t3a.medium",
        "awsRegionAZ": "",
        "awsAMI": "",
        "awsSecurityGroups": [
          ""
        ],
        "awsSSHKeyName": "",
        "awsCICDInstanceTag": "rancher-validation",
        "awsIAMProfile": "",
        "awsUser": "ubuntu",
        "volumeSize": 25,
        "roles": ["worker"]
      },
      {
        "instanceType": "t3a.large",
        "awsRegionAZ": "",
        "awsAMI": "",
        "awsSecurityGroups": [
          ""
        ],
        "awsSSHKeyName": "",
        "awsCICDInstanceTag": "rancher-validation",
        "awsIAMProfile": "",
        "awsUser": "ubuntu",
        "volumeSize": 25,
        "roles": ["etcd", "contolplane"]
      },
    ]
  }
}
```

## Advanced Settings
This encapsulates any other setting that is applied in the cluster.spec. Currently we have support for:
* cluster agent customization 
* fleet agent customization

Please read up on general k8s to get an idea of correct formatting for:
* resource requests
* resource limits
* node affinity
* tolerations

```json
"advancedOptions": {
    "clusterAgentCustomization": { // change this to fleetAgentCustomization for fleet agent
        "appendTolerations": [
            {
                "key": "Testkey",
                "value": "testValue",
                "effect": "NoSchedule"
            }
        ],
        "overrideResourceRequirements": {
            "limits": {
                "cpu": "750m",
                "memory": "500Mi"
            },
            "requests": {
                "cpu": "250m",
                "memory": "250Mi"
            }
        },
        "overrideAffinity": {
            "nodeAffinity": {
                "preferredDuringSchedulingIgnoredDuringExecution": [
                    {
                        "preference": {
                            "matchExpressions": [
                                {
                                    "key": "cattle.io/cluster-agent",
                                    "operator": "In",
                                    "values": [
                                        "true"
                                    ]
                                }
                            ]
                        },
                        "weight": 1
                    }
                ]
            }
        }
    }
}
```