# <p align="center">RANCHER :small_blue_diamond: TERRATEST</p>

##### Automation to allow rancher2 provider terraform resources to be tested with Terratest + Go

---

### Modules + supported tests:

<table align="center">
  <tbody>
    <tr>
      <th>Module</th>
      <th>Provision</th>
      <th>Scale</th>
      <th>K8s Upgrade</th>
    </tr>
    <tr>
      <td>aks</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
    </tr>
    <tr>
      <td>eks</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
    </tr>
    <tr>
      <td>gke</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
    </tr>
    <tr>
      <td>ec2_rke1</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
    </tr>
    <tr>
      <td>ec2_rke2</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
    </tr>
    <tr>
      <td>ec2_k3s</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
    </tr>
    <tr>
      <td>linode_rke1</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
    </tr>
    <tr>
      <td>linode_rke2</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
    </tr>
    <tr>
      <td>linode_k3s</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
    </tr>
  </tbody>
</table>

---

<a name="top"></a>

# <p align="center"> :scroll: Table of contents </p>

-   [Configurations](#configurations)
    -   [Rancher](#configurations-rancher)
    -   [Terraform](#configurations-terraform)
        -   [AKS](#configurations-terraform-aks)
        -   [EKS](#configurations-terraform-eks)
        -   [GKE](#configurations-terraform-gke)
        -   [EC2_RKE1](#configurations-terraform-ec2_rke1)
        -   [LINODE_RKE1](#configurations-terraform-linode_rke1)
        -   [RKE2 + K3S](#configurations-terraform-rke2_k3s)
        -   [EC2_RKE2 + EC2_K3S](#configurations-terraform-rke2_k3s_ec2)
        -   [LINODE_RKE2 + LINODE_K3S](#configurations-terraform-rke2_k3s_linode)
    -   [Terratest](#configurations-terratest)
        -   [Nodepools](#configurations-terratest-nodepools)
            -   [AKS Nodepools](#configurations-terratest-nodepools-aks)
            -   [EKS Nodepools](#configurations-terratest-nodepools-eks)
            -   [GKE Nodepools](#configurations-terratest-nodepools-gke)
            -   [RKE1, RKE2, K3S Nodepools](#configurations-terratest-nodepools-rke1_rke2_k3s)
        -  [Provision](#configurations-terratest-provision)
        -  [Scale](#configurations-terratest-scale)
        -  [Kubernetes Upgrade](#configurations-terratest-kubernetes_upgrade)
        -  [Build Module](#configurations-terratest-build_module)
        -  [Cleanup](#configurations-terratest-cleanup)

<a name="configurations"></a>

### <p align="center"> Configurations </p>

##### These tests require an accurately configured `cattle-config.yaml` to successfully run.

##### Each `cattle-config.yaml` must include the following configurations:

```yaml
rancher:
  # define rancher specific configs here

terraform:
  # define module specific configs here
  
terratest:
  # define test specific configs here
```

---

<a name="configurations-rancher"></a>
#### :small_red_triangle: [Back to top](#top)

The `rancher` configurations in the `cattle-config.yaml` will remain consistent across all modules and tests.  Fields to configure in this section are as follows:

<table>
  <tbody>
    <tr>
      <th>Field</th>
      <th>Description</th>
      <th>Type</th>
      <th>Example</th>
    </tr>
    <tr>
      <td>host</td>
      <td>url to rancher sercer without leading https:// and without trailing /</td>
      <td>string</td>
      <td>url-to-rancher-server.com</td>
    </tr>
    <tr>
      <td>adminToken</td>
      <td>rancher admin bearer token</td>
      <td>string</td>
      <td>token-XXXXX:XXXXXXXXXXXXXXX</td>
    </tr>
    <tr>
      <td>insecure</td>
      <td>must be set to true</td>
      <td>boolean</td>
      <td>true</td>
    </tr>
    <tr>
      <td>cleanup</td>
      <td>If true, resources will be cleaned up upon test completion</td>
      <td>boolean</td>
      <td>true</td>
    </tr>
  </tbody>
</table>

##### Example:

```yaml
rancher:
  host: url-to-rancher-server.com
  adminToken: token-XXXXX:XXXXXXXXXXXXXXX
  insecure: true
  cleanup: true
```

---

<a name="configurations-terraform"></a>
#### :small_red_triangle: [Back to top](#top)

The `terraform` configurations in the `cattle-config.yaml` are module specific.  Fields to configure vary per module.  Module specific fields to configure in this section are as follows:

<a name="configurations-terraform-aks"></a>
#### :small_red_triangle: [Back to top](#top)

##### AKS

<table>
  <tbody>
    <tr>
      <th>Field</th>
      <th>Description</th>
      <th>Type</th>
      <th>Example</th>
    </tr>
    <tr>
      <td>providerVersion</td>
      <td>rancher2 provider version</td>
      <td>string</td>
      <td>'1.25.0'</td>
    </tr>
    <tr>
      <td>module</td>
      <td>specify terraform module here</td>
      <td>string</td>
      <td>aks</td>
    </tr>
    <tr>
      <td>cloudCredentialName</td>
      <td>provide the name of unique cloud credentials to be created during testing</td>
      <td>string</td>
      <td>tf-aks</td>
    </tr>
    <tr>
      <td>azureClientID</td>
      <td>provide azure client id</td>
      <td>string</td>
      <td>XXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX</td>
    </tr>
    <tr>
      <td>azureClientSecret</td>
      <td>provide azure client secret</td>
      <td>string</td>
      <td>XXXXXXXXXXXXXXXXXXXXXXXXXX</td>
    </tr>
    <tr>
      <td>azureSubscriptionID</td>
      <td>provide azure subscription id</td>
      <td>string</td>
      <td>XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX</td>
    </tr>
    <tr>
      <td>clusterName</td>
      <td>provide a unique name for cluster</td>
      <td>string</td>
      <td>jkeslar-cluster</td>
    </tr>
    <tr>
      <td>resourceGroup</td>
      <td>provide an existing resource group from Azure</td>
      <td>string</td>
      <td>my-resource-group</td>
    </tr>
    <tr>
      <td>resourceLocation</td>
      <td>provide location for Azure instances</td>
      <td>string</td>
      <td>east-us</td>
    </tr>
    <tr>
      <td>hostnamePrefix</td>
      <td>provide a unique hostname prefix for resources</td>
      <td>string</td>
      <td>jkeslar</td>
    </tr>
    <tr>
      <td>networkPlugin</td>
      <td>provide network plugin</td>
      <td>string</td>
      <td>kubenet</td>
    </tr>
    <tr>
      <td>availabilityZones</td>
      <td>list of availablilty zones</td>
      <td>[]string</td>
      <td>
      - '1' <br/>
      - '2' <br/>
      - '3'
      </td>
    </tr>
    <tr>
      <td>osDiskSizeGB</td>
      <td>os disk size in gigabytes</td>
      <td>int64</td>
      <td>128</td>
    </tr>
    <tr>
      <td>vmSize</td>
      <td>vm size to be used for instances</td>
      <td>string</td>
      <td>Standard_DS2_v2</td>
    </tr>
  </tbody>
</table>

##### Example:

```yaml
terratest:
  providerVersion: '1.25.0'
  module: aks
  cloudCredentialName: tf-aks
  azureClientID: XXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX
  azureClientSecret: XXXXXXXXXXXXXXXXXXXXXXXXXX
  azureSubscriptionID: XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX
  clusterName: jkeslar-cluster
  resourceGroup: my-resource-group
  resourceLocation: east-us
  hostnamePrefix: jkeslar
  networkPlugin: kubenet
  availabilityZones:
    - '1'
    - '2'
    - '3'
  osDiskSizeGB: 128
  vmSize: Standard_DS2_v2
```

---


<a name="configurations-terraform-eks"></a>
#### :small_red_triangle: [Back to top](#top)

##### EKS

<table>
  <tbody>
    <tr>
      <th>Field</th>
      <th>Description</th>
      <th>Type</th>
      <th>Example</th>
    </tr>
    <tr>
      <td>providerVersion</td>
      <td>rancher2 provider version</td>
      <td>string</td>
      <td>'1.25.0'</td>
    </tr>
    <tr>
      <td>module</td>
      <td>specify terraform module here</td>
      <td>string</td>
      <td>eks</td>
    </tr>
    <tr>
      <td>cloudCredentialName</td>
      <td>provide the name of unique cloud credentials to be created during testing</td>
      <td>string</td>
      <td>tf-eks</td>
    </tr>
    <tr>
      <td>awsAccessKey</td>
      <td>provide aws access key</td>
      <td>string</td>
      <td>XXXXXXXXXXXXXXXXXXXX</td>
    </tr>
    <tr>
      <td>awsSecretKey</td>
      <td>provide aws secret key</td>
      <td>string</td>
      <td>XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX</td>
    </tr>
    <tr>
      <td>awsInstanceType</td>
      <td>provide aws instance type</td>
      <td>string</td>
      <td>t3.medium</td>
    </tr>
    <tr>
      <td>region</td>
      <td>provide a region for resources to be created in</td>
      <td>string</td>
      <td>us-east-2</td>
    </tr>
    <tr>
      <td>awsSubnets</td>
      <td>list of valid subnet IDs</td>
      <td>[]string</td>
      <td>
        - subnet-xxxxxxxx <br/>
        - subnet-yyyyyyyy <br/>
        - subnet-zzzzzzzz
      </td>
    </tr>
    <tr>
      <td>awsSecurityGroups</td>
      <td>list of security group IDs to be applied to AWS instances</td>
      <td>[]string</td>
      <td>- sg-xxxxxxxxxxxxxxxxx</td>
    </tr>
    <tr>
      <td>clusterName</td>
      <td>provide a unique name for your cluster</td>
      <td>string</td>
      <td>jkeslar-cluster</td>
    </tr>
    <tr>
      <td>hostnamePrefix</td>
      <td>provide a unique hostname prefix for resources</td>
      <td>string</td>
      <td>jkeslar</td>
    </tr>
    <tr>
      <td>publicAccess</td>
      <td>If true, public access will be enabled</td>
      <td>boolean</td>
      <td>true</td>
    </tr>
    <tr>
      <td>privateAccess</td>
      <td>If true, private access will be enabled</td>
      <td>boolean</td>
      <td>true</td>
    </tr>
    <tr>
      <td>nodeRole</td>
      <td>Optional with Rancher v2.7+ - if provided, this custom role will be used when creating instances for node groups</td>
      <td>string</td>
      <td>arn:aws:iam::############:role/my-custom-NodeInstanceRole-############</td>
    </tr>
  </tbody>
</table>

##### Example:

```yaml
terratest:
  providerVersion: '1.25.0'
  module: eks
  cloudCredentialName: tf-eks
  awsAccessKey: XXXXXXXXXXXXXXXXXXXX
  awsSecretKey: XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
  awsInstanceType: t3.medium
  region: us-east-2
  awsSubnets:
    - subnet-xxxxxxxx
    - subnet-yyyyyyyy
    - subnet-zzzzzzzz
  awsSecurityGroups:
    - sg-xxxxxxxxxxxxxxxxx
  clusterName: jkeslar-cluster
  hostnamePrefix: jkeslar
  publicAccess: true
  privateAccess: true
  nodeRole: arn:aws:iam::############:role/my-custom-NodeInstanceRole-############
```

---


<a name="configurations-terraform-gke"></a>
#### :small_red_triangle: [Back to top](#top)

##### GKE


<table>
  <tbody>
    <tr>
      <th>Field</th>
      <th>Description</th>
      <th>Type</th>
      <th>Example</th>
    </tr>
    <tr>
      <td>providerVersion</td>
      <td>rancher2 provider version</td>
      <td>string</td>
      <td>'1.25.0'</td>
    </tr>
    <tr>
      <td>module</td>
      <td>specify terraform module here</td>
      <td>string</td>
      <td>eks</td>
    </tr>
    <tr>
      <td>cloudCredentialName</td>
      <td>provide the name of unique cloud credentials to be created during testing</td>
      <td>string</td>
      <td>tf-eks</td>
    </tr>
    <tr>
      <td>clusterName</td>
      <td>provide a unique cluster name</td>
      <td>string</td>
      <td>jkeslar-cluster</td>
    </tr>
    <tr>
      <td>region</td>
      <td>provide region for resources to be created in</td>
      <td>string</td>
      <td>us-central1-a</td>
    </tr>
    <tr>
      <td>gkeProjectID</td>
      <td>provide gke project ID</td>
      <td>string</td>
      <td>my-project-id-here</td>
    </tr>
    <tr>
      <td>gkeNetwork</td>
      <td>specify network here</td>
      <td>string</td>
      <td>default</td>
    </tr>
    <tr>
      <td>gkeSubnetwork</td>
      <td>specify subnetwork here</td>
      <td>string</td>
      <td>default</td>
    </tr>
    <tr>
      <td>hostnamePrefix</td>
      <td>provide a unique hostname prefix for resources</td>
      <td>string</td>
      <td>jkeslar</td>
    </tr>
  </tbody>
</table>

##### Example:

```yaml
terraform:
  providerVersion: '1.25.0'
  module: gke
  cloudCredentialName: tf-creds-gke
  clusterName: jkeslar-cluster
  region: us-central1-a
  gkeProjectID: my-project-id-here
  gkeNetwork: default
  gkeSubnetwork: default
  hostnamePrefix: jkeslar
```

---


<a name="configurations-terraform-ec2_rke1"></a>
#### :small_red_triangle: [Back to top](#top)

##### EC2_RKE1

<table>
  <tbody>
    <tr>
      <th>Field</th>
      <th>Description</th>
      <th>Type</th>
      <th>Example</th>
    </tr>
    <tr>
      <td>providerVersion</td>
      <td>rancher2 provider version</td>
      <td>string</td>
      <td>'1.25.0'</td>
    </tr>
    <tr>
      <td>module</td>
      <td>specify terraform module here</td>
      <td>string</td>
      <td>ec2_rke1</td>
    </tr>
    <tr>
      <td>awsAccessKey</td>
      <td>provide aws access key</td>
      <td>string</td>
      <td>XXXXXXXXXXXXXXXXXXXX</td>
    </tr>
    <tr>
      <td>awsSecretKey</td>
      <td>provide aws secret key</td>
      <td>string</td>
      <td>XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX</td>
    </tr>
    <tr>
      <td>ami</td>
      <td>provide ami; (optional - may be left as empty string '')</td>
      <td>string</td>
      <td>''</td>
    </tr>
    <tr>
      <td>awsInstanceType</td>
      <td>provide aws instance type</td>
      <td>string</td>
      <td>t3.medium</td>
    </tr>
    <tr>
      <td>region</td>
      <td>provide a region for resources to be created in</td>
      <td>string</td>
      <td>us-east-2</td>
    </tr>
    <tr>
      <td>awsSecurityGroupNames</td>
      <td>list of security groups to be applied to AWS instances</td>
      <td>[]string</td>
      <td>- security-group-name</td>
    </tr>
    <tr>
      <td>awsSubnetID</td>
      <td>provide a valid subnet ID</td>
      <td>string</td>
      <td>subnet-xxxxxxxx</td>
    </tr>
    <tr>
      <td>awsVpcID</td>
      <td>provide a valid VPC ID</td>
      <td>string</td>
      <td>vpc-xxxxxxxx</td>
    </tr>
    <tr>
      <td>awsZoneLetter</td>
      <td>provide zone letter to be used</td>
      <td>string</td>
      <td>a</td>
    </tr>
    <tr>
      <td>awsRootSize</td>
      <td>root size in gigabytes</td>
      <td>int64</td>
      <td>80</td>
    </tr>
    <tr>
      <td>clusterName</td>
      <td>provide a unique name for your cluster</td>
      <td>string</td>
      <td>jkeslar-cluster</td>
    </tr>
    <tr>
      <td>networkPlugin</td>
      <td>provide network plugin to be used</td>
      <td>string</td>
      <td>canal</td>
    </tr>
    <tr>
      <td>nodeTemplateName</td>
      <td>provide a unique name for node template</td>
      <td>string</td>
      <td>tf-rke1-template</td>
    </tr>
    <tr>
      <td>hostnamePrefix</td>
      <td>provide a unique hostname prefix for resources</td>
      <td>string</td>
      <td>jkeslar</td>
    </tr>
  </tbody>
</table>

##### Example:

```yaml
terratest:
  providerVersion: '1.25.0'
  module: ec2_rke1
  awsAccessKey: XXXXXXXXXXXXXXXXXXXX
  awsSecretKey: XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
  ami:
  awsInstanceType: t3.medium
  region: us-east-2
  awsSecurityGroupNames:
    - security-group-name
  awsSubnetID: subnet-xxxxxxxx
  awsVpcID: vpc-xxxxxxxx
  awsZoneLetter: a
  awsRootSize: 80
  clusterName: jkeslar-cluster
  networkPlugin: canal
  nodeTemplateName: tf-rke1-template
  hostnamePrefix: jkeslar
```
---


<a name="configurations-terraform-linode_rke1"></a>
#### :small_red_triangle: [Back to top](#top)

##### LINODE_RKE1

<table>
  <tbody>
    <tr>
      <th>Field</th>
      <th>Description</th>
      <th>Type</th>
      <th>Example</th>
    </tr>
    <tr>
      <td>providerVersion</td>
      <td>rancher2 provider version</td>
      <td>string</td>
      <td>'1.25.0'</td>
    </tr>
    <tr>
      <td>module</td>
      <td>specify terraform module here</td>
      <td>string</td>
      <td>linode_rke1</td>
    </tr>
   <tr>
      <td>linodeToken</td>
      <td>provide linode token credential</td>
      <td>string</td>
      <td>XXXXXXXXXXXXXXXXXXXX</td>
    </tr>
    <tr>
      <td>region</td>
      <td>provide a region for resources to be created in</td>
      <td>string</td>
      <td>us-east</td>
    </tr>
    <tr>
      <td>linodeRootPass</td>
      <td>provide a unique root password</td>
      <td>string</td>
      <td>xxxxxxxxxxxxxxxx</td>
    </tr>
    <tr>
      <td>clusterName</td>
      <td>provide a unique name for your cluster</td>
      <td>string</td>
      <td>jkeslar-cluster</td>
    </tr>
    <tr>
      <td>networkPlugin</td>
      <td>provide network plugin to be used</td>
      <td>string</td>
      <td>canal</td>
    </tr>
    <tr>
      <td>nodeTemplateName</td>
      <td>provide a unique name for node template</td>
      <td>string</td>
      <td>tf-rke1-template</td>
    </tr>
    <tr>
      <td>hostnamePrefix</td>
      <td>provide a unique hostname prefix for resources</td>
      <td>string</td>
      <td>jkeslar</td>
    </tr>
  </tbody>
</table>

##### Example:
```yaml
terraform:
  providerVersion: '1.25.0'
  module: linode_rke1
  linodeToken: XXXXXXXXXXXXXXXXXXXX
  region: us-east
  linodeRootPass: xxxxxxxxxxxxxxxx
  clusterName: jkeslar
  networkPlugin: canal
  nodeTemplateName: tf-rke1-template
  hostnamePrefix: jkeslar
```

---

<a name="configurations-terraform-rke2_k3s_ec2"></a>
#### :small_red_triangle: [Back to top](#top)

##### EC2_RKE2 + EC2_K3S

<table>
  <tbody>
    <tr>
      <th>Field</th>
      <th>Description</th>
      <th>Type</th>
      <th>Example</th>
    </tr>
    <tr>
      <td>providerVersion</td>
      <td>rancher2 provider version</td>
      <td>string</td>
      <td>'1.25.0'</td>
    </tr>
    <tr>
      <td>module</td>
      <td>specify terraform module here</td>
      <td>string</td>
      <td>ec2_rke2</td>
    </tr>
    <tr>
      <td>cloudCredentialName</td>
      <td>provide the name of unique cloud credentials to be created during testing</td>
      <td>string</td>
      <td>tf-creds-rke2</td>
    </tr>
    <tr>
      <td>awsAccessKey</td>
      <td>provide aws access key</td>
      <td>string</td>
      <td>XXXXXXXXXXXXXXXXXXXX</td>
    </tr>
    <tr>
      <td>awsSecretKey</td>
      <td>provide aws secret key</td>
      <td>string</td>
      <td>XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX</td>
    </tr>
    <tr>
      <td>ami</td>
      <td>provide ami; (optional - may be left as empty string '')</td>
      <td>string</td>
      <td>''</td>
    </tr>
    <tr>
      <td>region</td>
      <td>provide a region for resources to be created in</td>
      <td>string</td>
      <td>us-east-2</td>
    </tr>
    <tr>
      <td>awsSecurityGroupNames</td>
      <td>list of security groups to be applied to AWS instances</td>
      <td>[]string</td>
      <td>- my-security-group</td>
    </tr>
    <tr>
      <td>awsSubnetID</td>
      <td>provide a valid subnet ID</td>
      <td>string</td>
      <td>subnet-xxxxxxxx</td>
    </tr>
    <tr>
      <td>awsVpcID</td>
      <td>provide a valid VPC ID</td>
      <td>string</td>
      <td>vpc-xxxxxxxx</td>
    </tr>
    <tr>
      <td>awsZoneLetter</td>
      <td>provide zone letter to be used</td>
      <td>string</td>
      <td>a</td>
    </tr>
    <tr>
      <td>machineConfigName</td>
      <td>provide a unique name for machine config</td>
      <td>string</td>
      <td>tf-rke2</td>
    </tr>
    <tr>
      <td>clusterName</td>
      <td>provide a unique name for your cluster</td>
      <td>string</td>
      <td>jkeslar-cluster</td>
    </tr>
    <tr>
      <td>enableNetworkPolicy</td>
      <td>If true, Network Policy will be enabled</td>
      <td>boolean</td>
      <td>false</td>
    </tr>
    <tr>
      <td>defaultClusterRoleForProjectMembers</td>
      <td>select default role to be used for project memebers</td>
      <td>string</td>
      <td>user</td>
    </tr>
  </tbody>
</table>

##### Example:
```yaml
terraform:
  providerVersion: '1.25.0'
  module: ec2_rke2
  cloudCredentialName: tf-creds-rke2
  awsAccessKey: XXXXXXXXXXXXXXXXXXXX
  awsSecretKey: XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
  ami:
  region: us-east-2
  awsSecurityGroupNames:
    - my-security-group
  awsSubnetID: subnet-xxxxxxxx
  awsVpcID: vpc-xxxxxxxx
  awsZoneLetter: a
  machineConfigName: tf-rke2
  clusterName: jkeslar-cluster
  enableNetworkPolicy: false
  defaultClusterRoleForProjectMembers: user
```

---


<a name="configurations-terraform-rke2_k3s_linode"></a>
#### :small_red_triangle: [Back to top](#top)

##### LINODE_RKE2 + LINODE_K3S

<table>
  <tbody>
    <tr>
      <th>Field</th>
      <th>Description</th>
      <th>Type</th>
      <th>Example</th>
    </tr>
    <tr>
      <td>providerVersion</td>
      <td>rancher2 provider version</td>
      <td>string</td>
      <td>'1.25.0'</td>
    </tr>
    <tr>
      <td>module</td>
      <td>specify terraform module here</td>
      <td>string</td>
      <td>linode_k3s</td>
    </tr>
    <tr>
      <td>cloudCredentialName</td>
      <td>provide the name of unique cloud credentials to be created during testing</td>
      <td>string</td>
      <td>tf-linode</td>
    </tr>
    <tr>
      <td>linodeToken</td>
      <td>provide linode token credential</td>
      <td>string</td>
      <td>XXXXXXXXXXXXXXXXXXXX</td>
    </tr>
    <tr>
      <td>linodeImage</td>
      <td>specify image to be used for instances</td>
      <td>string</td>
      <td>linode/ubuntu20.04</td>
    </tr>
    <tr>
      <td>region</td>
      <td>provide a region for resources to be created in</td>
      <td>string</td>
      <td>us-east</td>
    </tr>
    <tr>
      <td>linodeRootPass</td>
      <td>provide a unique root password</td>
      <td>string</td>
      <td>xxxxxxxxxxxxxxxx</td>
    </tr>
    <tr>
      <td>machineConfigName</td>
      <td>provide a unique name for machine config</td>
      <td>string</td>
      <td>tf-k3s</td>
    </tr>
    <tr>
      <td>clusterName</td>
      <td>provide a unique name for your cluster</td>
      <td>string</td>
      <td>jkeslar-cluster</td>
    </tr>
    <tr>
      <td>enableNetworkPolicy</td>
      <td>If true, Network Policy will be enabled</td>
      <td>boolean</td>
      <td>false</td>
    </tr>
    <tr>
      <td>defaultClusterRoleForProjectMembers</td>
      <td>select default role to be used for project memebers</td>
      <td>string</td>
      <td>user</td>
    </tr>
  </tbody>
</table>

##### Example:
```yaml
terraform:
  providerVersion: '1.25.0'
  module: linode_k3s
  cloudCredentialName: tf-linode-creds
  linodeToken: XXXXXXXXXXXXXXXXXXXX
  linodeImage: linode/ubuntu20.04
  region: us-east
  linodeRootPass: xxxxxxxxxxxx
  machineConfigName: tf-k3s
  clusterName: jkeslar-cluster
  enableNetworkPolicy: false
  defaultClusterRoleForProjectMembers: user
```

---


<a name="configurations-terratest"></a>
#### :small_red_triangle: [Back to top](#top)

The `terratest` configurations in the `cattle-config.yaml` are test specific. Fields to configure vary per test. The `nodepools` field in the below configurations will vary depending on the module.  I will outline what each module expects first, then proceed to show the whole test specific configurations. 


<a name="configurations-terratest-nodepools"></a>
#### :small_red_triangle: [Back to top](#top)

###### Nodepools 
type: []Nodepool

<a name="configurations-terratest-nodepools-aks"></a>
#### :small_red_triangle: [Back to top](#top)

###### AKS Nodepool

AKS nodepools only need the `quantity` of nodes per pool to be provided, of type `int64`.  The below example will create a cluster with three node pools, each with a single node.

###### Example:
```yaml
nodepools:
  - quantity: 1
  - quantity: 1
  - quantity: 1
```

<a name="configurations-terratest-nodepools-eks"></a>
#### :small_red_triangle: [Back to top](#top)

###### EKS Nodepool

EKS nodepools require the `instanceType`, as type `string`, the `desiredSize` of the nodepool, as type `int64`, the `maxSize` of the node pool, as type `int64`, and the `minSize` of the node pool, as type `int64`. The minimum requirement for an EKS nodepool's `desiredSize` is `2`.  This must be respected or the cluster will fail to provision.

###### Example:
```yaml
nodepools:
  - instanceType: t3.medium
    desiredSize: 3
    maxSize: 3
    minSize: 0
```

<a name="configurations-terratest-nodepools-gke"></a>
#### :small_red_triangle: [Back to top](#top)

###### GKE Nodepool

GKE nodepools require the `quantity` of the node pool, as type `int64`, and the `maxPodsContraint`, as type `int64`.

###### Example:
```yaml
nodepools:
  - quantity: 2
    maxPodsContraint: 110
```

<a name="configurations-terratest-nodepools-rke1_rke2_k3s"></a>
#### :small_red_triangle: [Back to top](#top)

###### RKE1, RKE2, and K3S - all share the same nodepool configurations

For these modules, the required nodepool fields are the `quantity`, as type `int64`, as well as the roles to be assigned, each to be set or toggled via boolean - [`etcd`, `controlplane`, `worker`]. The following example will create three node pools, each with individual roles, and one node per pool.

###### Example:
```yaml
nodepools:
  - quantity: 1
    etcd: true
    controlplane: false
    worker: false
  - quantity: 1
    etcd: false
    controlplane: true
    worker: false
  - quantity: 1
    etcd: false
    controlplane: false
    worker: true
```

That wraps up the sub-section on nodepools, circling back to the test specific configs now...


Test specific fields to configure in this section are as follows:

<a name="configurations-terratest-provision"></a>
#### :small_red_triangle: [Back to top](#top)

##### Provision

<table>
  <tbody>
    <tr>
      <th>Field</th>
      <th>Description</th>
      <th>Type</th>
      <th>Example</th>
    </tr>
    <tr>
      <td>nodepools</td>
      <td>provide nodepool configs to be initially provisioned</td>
      <td>[]Nodepool</td>
      <td>view section on nodepools above or example yaml below</td>
    </tr>
    <tr>
      <td>kubernetesVersion</td>
      <td>specify the kubernetes version to be used</td>
      <td>string</td>
      <td>view yaml below for all module specific expected k8s version formats</td>
    </tr>
    <tr>
      <td>nodeCount</td>
      <td>provide the expected initial node count</td>
      <td>int64</td>
      <td>3</td>
    </tr>
  </tbody>
</table>

###### Example:
```yaml
# this example is valid for RKE1 provision
terratest:
  nodepools:
    - quantity: 1
      etcd: true
      controlplane: false
      worker: false
    - quantity: 1
      etcd: false
      controlplane: true
      worker: false
    - quantity: 1
      etcd: false
      controlplane: false
      worker: true
  kubernetesVersion: v1.24.9-rancher1-1
  nodeCount: 3


  # Below are the expected formats for all module kubernetes versions
  
  # AKS without leading v
  # e.g. '1.24.6'
  
  # EKS without leading v or any tail ending
  # e.g. '1.23' or '1.24'
  
  # GKE without leading v but with tail ending included
  # e.g. 1.23.12-gke.100
  
  # RKE1 with leading v and -rancher1-1 tail
  # e.g. v1.24.9-rancher1-1

  # RKE2 with leading v and +rke2r# tail
  # e.g. v1.24.9+rke2r1

  # K3S with leading v and +k3s# tail
  # e.g. v1.24.9+k3s1
```

---

<a name="configurations-terratest-scale"></a>
#### :small_red_triangle: [Back to top](#top)

##### Scale

<table>
  <tbody>
    <tr>
      <th>Field</th>
      <th>Description</th>
      <th>Type</th>
      <th>Example</th>
    </tr>
    <tr>
      <td>nodepools</td>
      <td>provide nodepool configs to be initially provisioned</td>
      <td>[]Nodepool</td>
      <td>view section on nodepools above or example yaml below</td>
    </tr>
    <tr>
      <td>scaledUpNodepools</td>
      <td>provide nodepool configs to be scaled up to, after initial provisioning</td>
      <td>[]Nodepool</td>
      <td>view section on nodepools above or example yaml below</td>
    </tr>
    <tr>
      <td>scaledDownNodepools</td>
      <td>provide nodepool configs to be scaled down to, after scaling up cluster</td>
      <td>[]Nodepool</td>
      <td>view section on nodepools above or example yaml below</td>
    </tr>
    <tr>
      <td>kubernetesVersion</td>
      <td>specify the kubernetes version to be used</td>
      <td>string</td>
      <td>view example yaml above for provisioning test for all module specific expected k8s version formats</td>
    </tr>
    <tr>
      <td>nodeCount</td>
      <td>provide the expected initial node count</td>
      <td>int64</td>
      <td>3</td>
    </tr>
    <tr>
      <td>scaledUpNodeCount</td>
      <td>provide the expected node count of scaled up cluster</td>
      <td>int64</td>
      <td>8</td>
    </tr>
    <tr>
      <td>scaledDownNodeCount</td>
      <td>provide the expected node count of scaled down cluster</td>
      <td>int64</td>
      <td>6</td>
    </tr>
  </tbody>
</table>

###### Example:
```yaml
# this example is valid for RKE1 scale
terratest:
  nodepools:
    - quantity: 1
      etcd: true
      controlplane: false
      worker: false
    - quantity: 1
      etcd: false
      controlplane: true
      worker: false
    - quantity: 1
      etcd: false
      controlplane: false
      worker: true
  scaledUpNodepools:
    - quantity: 3
      etcd: true
      controlplane: false
      worker: false
    - quantity: 2
      etcd: false
      controlplane: true
      worker: false
    - quantity: 3
      etcd: false
      controlplane: false
      worker: true
  scaledDownNodepools:
    - quantity: 3
      etcd: true
      controlplane: false
      worker: false
    - quantity: 2
      etcd: false
      controlplane: true
      worker: false
    - quantity: 1
      etcd: false
      controlplane: false
      worker: true
  kubernetesVersion: v1.24.9-rancher1-1
  nodeCount: 3
  scaledUpNodeCount: 8
  scaledDownNodeCount: 6
```

---

<a name="configurations-terratest-kubernetes_upgrade"></a>
#### :small_red_triangle: [Back to top](#top)

##### Kubernetes Upgrade

<table>
  <tbody>
    <tr>
      <th>Field</th>
      <th>Description</th>
      <th>Type</th>
      <th>Example</th>
    </tr>
    <tr>
      <td>nodepools</td>
      <td>provide nodepool configs to be initially provisioned</td>
      <td>[]Nodepool</td>
      <td>view section on nodepools above or example yaml below</td>
    </tr>
    <tr>
      <td>nodeCount</td>
      <td>provide the expected initial node count</td>
      <td>int64</td>
      <td>3</td>
    </tr>
    <tr>
      <td>kubernetesVersion</td>
      <td>specify the kubernetes version to be used</td>
      <td>string</td>
      <td>view example yaml above for provisioning test for all module specific expected k8s version formats</td>
    </tr>
    <tr>
      <td>upgradedKubernetesVersion</td>
      <td>specify the kubernetes version to be upgraded to</td>
      <td>string</td>
      <td>view example yaml above for provisioning test for all module specific expected k8s version formats</td>
    </tr>
  </tbody>
</table>

###### Example:
```yaml
# this example is valid for K3s kubernetes upgrade
terratest:
  nodepools:
    - quantity: 1
      etcd: true
      controlplane: false
      worker: false
    - quantity: 1
      etcd: false
      controlplane: true
      worker: false
    - quantity: 1
      etcd: false
      controlplane: false
      worker: true
  nodeCount: 3
  kubernetesVersion: v1.23.14+k3s1
  upgradedKubernetesVersion: v1.24.8+k3s1
```

---

<a name="configurations-terratest-build_module"></a>
#### :small_red_triangle: [Back to top](#top)

##### Build Module

Build module test may be used and ran to create a main.tf terraform configuration file for the desired module.  This is logged to the output for future reference and use.

Testing configurations for this are the same as outlined in provisioning test above.  Please review provisioning test configurations for more details.

---

<a name="configurations-terratest-cleanup"></a>
#### :small_red_triangle: [Back to top](#top)

##### Cleanup

Cleanup test may be used to clean up resources in situations where rancher config has `cleanup` set to `false`.  This may be helpful in debugging.  This test expects the same configurations used to initially create this environment, to properly clean them up.
