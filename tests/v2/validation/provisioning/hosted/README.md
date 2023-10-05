## Hosted Provider Provisioning Configs

For your config, you will need everything in the [Prerequisites](../README.md) section on the previous readme along with and at least one [Cloud Credential](#cloud-credentials) and [Hosted Provider Config](#hosted-provider-configs). 

Your GO test_package should be set to `provisioning/hosted`.
Your GO suite should be set to `-run ^TestHostedClusterProvisioningTestSuite$`.
Please see below for more details for your config. 

## Table of Contents
1. [Prerequisites](../README.md)
2. [Cloud Credential](#cloud-credentials)
3. [Hosted Provider Config](#hosted-provider-configs)
4. [Back to general provisioning](../README.md)

Below are example configs needed for the different hosted providers including GKE, AKS, and EKS. In order to run these tests, the [cloud credentials](#cloud-credentials) are also needed. GKE (googleCredentials), AKS(azureCredentials), and EKS(awsCredentials)

## Cloud Credentials

### AWS
```json
"awsCredentials": {
   "secretKey": "",
   "accessKey": "",
   "defaultRegion": ""
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
### Google
```json
"googleCredentials": {
    "authEncodedJson": ""
}
```

## Hosted Provider Configs

### EKS Cluster Config
```json
"eksClusterConfig": {
  "imported": false,
  "kmsKey": "",
  "kubernetesVersion": "1.26",
  "loggingTypes": [],
  "nodeGroups": [
    {
      "desiredSize": 2,
      "diskSize": 20,
      "ec2SshKey": "",
      "gpu": false,
      "imageId": "",
      "instanceType": "t3.medium",
      "labels": {},
      "maxSize": 2,
      "minSize": 2,
      "nodegroupName": "",
      "requestSpotInstances": false,
      "resourceTags": {},
      "subnets": [],
      "tags": {}

    }
  ],
  "privateAccess": true,
  "publicAccess": true,
  "publicAccessSources": [],
  "region": "us-east-2",
  "secretsEncryption": false,
  "securityGroups": [""],
  "serviceRole": "",
  "subnets": ["", ""],
  "tags": {}
},
```

See an example on running this test: `run ^TestHostedEKSClusterProvisioningTestSuite/TestProvisioningHostedEKS$`

### AKS Cluster Config
```json
"aksClusterConfig": {
  "dnsPrefix": "-dns",
  "imported": false,
  "kubernetesVersion": "1.26.6",
  "linuxAdminUsername": "azureuser",
  "loadBalancerSku": "Standard",
  "networkPlugin": "kubenet",
  "nodePools": [
    {
      "availabilityZones": ["1", "2", "3"],
      "nodeCount": 1,
      "enableAutoScaling": false,
      "maxPods": 110,
      "maxCount": 2,
      "minCount": 1,
      "mode": "System",
      "name": "agentpool",
      "osDiskSizeGB": 128,
      "osDiskType": "Managed",
      "osType": "Linux",
      "vmSize": "Standard_DS2_v2"
    }
  ],
  "privateCluster": false,
  "resourceGroup": "",
  "resourceLocation": "eastus",
  "tags": {}
}
```

See an example on running this test: `run ^TestHostedAKSClusterProvisioningTestSuite/TestProvisioningHostedAKS$`

### GKE Cluster Config
Note that the following are required and should be updated:
* kubernetesVersion
* location
* locations
* zone
* labels
* nodePools->name
* nodePools->labels
* nodePools->config->imageType (currently set to COS_CONTAINERD for use with 1.23+)

```json
"gkeClusterConfig" : {
  "clusterAddons": {
    "horizontalPodAutoscaling": true, 
    "httpLoadBalancing": true, 
    "networkPolicyConfig": false
  },
  "horizontalPodAutoscaling": true,
  "httpLoadBalancing": true,
  "networkPolicyConfig": false,
  "clusterIpv4Cidr": "",
  "enableKubernetesAlpha": false,
  "ipAllocationPolicy": {
    "clusterIpv4Cidr": "",
    "clusterIpv4CidrBlock": null,
    "clusterSecondaryRangeName": null,
    "createSubnetwork": false,
    "nodeIpv4CidrBlock": null,
    "servicesIpv4CidrBlock": null,
    "servicesSecondaryRangeName": null,
    "subnetworkName": null,
    "useIpAliases": true
  },
  "kubernetesVersion": "1.26.8-gke.200",
  "labels": {"key": "value"},
  "locations": ["us-central1-c"],
  "location": "us-central1-c"
  "loggingService": "logging.googleapis.com/kubernetes",
  "maintenanceWindow": "",
  "masterAuthorizedNetworks": {
    "enabled": false
  },
  "monitoringService": "monitoring.googleapis.com/kubernetes",
  "network": "default",
  "networkPolicyEnabled": false,
  "nodePools": [
    {
      "autoscaling": {
        "enabled": false,
        "maxNodeCount": null,
        "minNodeCount": null
      },
      "config": {
        "diskSizeGb": 100,
        "diskType": "pd-standard",
        "imageType": "COS_CONTAINERD",
        "labels": {"key": "value"},
        "localSsdCount": 0,
        "machineType": "n1-standard-2",
        "oauthScopes": [
          "https://www.googleapis.com/auth/devstorage.read_only",
          "https://www.googleapis.com/auth/logging.write",
          "https://www.googleapis.com/auth/monitoring",
          "https://www.googleapis.com/auth/servicecontrol",
          "https://www.googleapis.com/auth/service.management.readonly",
          "https://www.googleapis.com/auth/trace.append"
        ],
        "preemptible": false,
        "tags": null,
        "taints": null
      },
      "initialNodeCount": 3,
      "isNew": true,
      "management": {
        "autoRepair": true, 
        "autoUpgrade": true
      },
      "maxPodsConstraint": 110,
      "name": ""
    }
  ],
  "privateClusterConfig": {
    "enablePrivateEndpoint": false, 
    "enablePrivateNodes": false, 
    "masterIpv4CidrBlock": null
  },
  "projectID": "",
  "region": "",
  "subnetwork": "default",
  "zone": "us-central1-c"
}
```

See an example on running this test: `run ^TestHostedGKEClusterProvisioningTestSuite/TestProvisioningHostedGKE$`