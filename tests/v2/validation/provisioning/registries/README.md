# Private Registries Automation

- [Private Registries Automation](#private-registries-automation)
  - [Jenkins](#jenkins)
  - [Infrastructure](#infrastructure)
    - [Test Setup - the use  of Corral Packages](#test-setup---the-use-of-corral-packages)
    - [Jenkins job RANCHER\_CORRAL\_CONFIG](#jenkins-job-rancher_corral_config)
    - [Jenkins job REGISTRIES\_CORRAL\_CONFIG](#jenkins-job-registries_corral_config)
    - [Jenkins job \*BRANCH\* and \*REPO\* variables](#jenkins-job-branch-and-repo-variables)
    - [Manual steps that are done automatically by the Pipeline and Test Setup](#manual-steps-that-are-done-automatically-by-the-pipeline-and-test-setup)
    - [Test Setup Result either manual or automated](#test-setup-result-either-manual-or-automated)
    - [The information about the Private Registries and Rancher is ready to be used by Go](#the-information-about-the-private-registries-and-rancher-is-ready-to-be-used-by-go)
  - [Go environment setup and validations](#go-environment-setup-and-validations)
    - [Flags](#flags)
    - [The registries configuration in CONFIG](#the-registries-configuration-in-config)
    - [The Corral Variables configuration in CONFIG](#the-corral-variables-configuration-in-config)
    - [The Go Corral client will use the packages to retrieve the information needed for the Go Validations](#the-go-corral-client-will-use-the-packages-to-retrieve-the-information-needed-for-the-go-validations)
    - [ The provisioningInput used for the downstream clusters in CONFIG](#the-provisioninginput-used-for-the-downstream-clusters-in-config)
  - [Considerations for Testing or Debug ^^^ IMPORTANT ^^^](#considerations-for-testing-or-debug--important-)
  - [USE CASE](#use-case)

## Jenkins

Information about Jenkins is in our Internal Documentation

## Infrastructure

The automation is divided in two Repositories.

- Corral - <a href="https://github.com/rancherlabs/corral-packages"
    rel="nofollow">rancherlabs/corral-packages</a>
- Rancher / Go tests - <a href="https://github.com/rancher/rancher"
    rel="nofollow">rancher/rancher</a>

### Test Setup - the use  of Corral Packages

The Corral takes care of creating the Private Registries and the Rancher
setup.

These configurations and tests are mapped for the <a
href="https://confluence.suse.com/display/RANQA/Test+Case+Mapping+for+Release+Testing+Automation+Tasks"
rel="nofollow">release testing test cases</a>. Scroll down to the
Registries table's rows.

We need two Corral Package Configurations

1. To create the two standalone registries for the Downstream
    Clusters - `Auth`  and  `No Auth`
    1. Or if using the `UseExistingRegistries` flag in the main Go
        framework `CONFIG` file do not use the package and use the
        existing registries configuration
    2. This will create two standalone Private Registries, those will
        be hosted on two standalone nodes, all the registries use Valid
        Certs. - Certificate values are set as base64 strings in
        `CONFIG`
    3. The <a
        href="https://github.com/rancherlabs/corral-packages/blob/main/packages/aws/rancher-registry.yaml"
        rel="nofollow">Registry Standalone</a> package is used for
        this. - The values should match the `rancher-version` under test
2. A second Package to Install Rancher with a standalone Private
    Registry for it. - <a
    href="https://github.com/rancherlabs/corral-packages/blob/main/packages/aws/rancher-registry.yaml"
    rel="nofollow">Rancher with Registry</a> package configuration with
    values updated to required versions  
    1. If the `UseExistingRegistries` flag is set in `CONFIG` use an
        existing Rancher install without using the Corral package
    2. Or use the Corral Package for a <a
        href="https://github.com/rancherlabs/corral-packages/blob/main/packages/aws/rancher.yaml"
        rel="nofollow">Rancher without Registry</a>
    3. Rancher install in both cases use Rancher `self-signed` certs -
        No config needed for certs
3. An existing ECR registry will be configured to use the registry
    images specific to the Rancher version set in the <a
    href="https://github.com/rancherlabs/corral-packages/blob/main/packages/aws/rancher-registry.yaml"
    rel="nofollow">Rancher with Registry</a> package's
    1. `A registry-auth` with a value included as `ecr`  will create
        the config for the ECR creation logic.

The cert manager version for the Rancher `self-signed` certificates
install is also configured in the Rancher Corral packages and the images
pushed to the registry. This is a mandatory dependency for this type of
install.

The default install of the Rancher packages use an RKE2 cluster. This is
the only option so far in Corral Packages.

- It is important that the `kubernetes_version` match the RKE2 cluster
    image included in the Rancher's `rancher-images.txt`
  - `rancher-images.txt` can be found in the Github release page of
        the Rancher version.
  - example:
        <a href="https://github.com/rancher/rancher/releases/tag/v2.7.5"
        rel="nofollow">https://github.com/rancher/rancher/releases/tag/v2.7.5</a>
        Scrolling down to the assets. This file is important for setting
        all the provisioning versions in the `CONFIG`  file too.

### Jenkins job RANCHER_CORRAL_CONFIG

The Corral package yaml values to build Rancher. Example:

Reminder that the `kubernetes_version`  must match with versions
existing in `rancher-images.txt` of the Rancher version under test.

``` yml
manifest:
  name: rancher-registry
  description: rancher-registry
  variables:
    registry_auth:
      default: enabled
templates:
  - aws/cluster_nodes
  - aws/registry_nodes
  - registry-standalone
  - rke2
  - rancher
variables:
  cni:
    - calico
  kubernetes_version:
    - v1.23.10+rke2r1
    - v1.24.4+rke2r1
  registry_auth:
    - global
  docker_compose_version:
    - 2.15.1
  rancher_version:
    - 2.7.0
  cert_manager_version:
    - 1.11.0
```

### Jenkins job REGISTRIES_CORRAL_CONFIG

``` yml
manifest:
  name: registry
  description: registry
  variables:
    registry_auth:
      default: enabled
templates:
  - aws/registry_nodes
  - registry-standalone
variables:
  registry_auth:
    - global
    - enabled
    - disabled
    - ecr
  docker_compose_version:
    - 2.15.1
  rancher_version:
    - 2.7.0
  cert_manager_version:
    - 1.11.0
```

``` java
awsCredentials:
  accessKey: <REDACTED>
  defaultRegion: us-east-2
  secretKey: <REDACTED>
awsEC2Config:
  awsAMI: <REDACTED>
  awsAccessKeyID: <REDACTED>
  awsCICDInstanceTag: rancher-validation
  awsIAMProfile: ""
  awsRegionAZ: ""
  awsSSHKeyName: <REDACTED>
  awsSecretAccessKey: <REDACTED>
  awsSecurityGroups:
  - <REDACTED>
  awsUser: ubuntu
  instanceType: t3a.xlarge
  region: us-east-2
  volumeSize: 50
awsMachineConfig:
  instanceType: t3a.xlarge
  region: us-east-2
  retries: "5"
  rootSize: "16"
  securityGroup:
  - open-all
  sshUser: ubuntu
  volumeType: gp3
  vpcId: <REDACTED>
  zone: a
awsNodeTemplate:
  accessKey: <REDACTED>
  ami: <REDACTED>
  blockDurationMinutes: 0
  encryptEbsVolume: false
  endpoint: ""
  httpEndpoint: enabled
  httpTokens: optional
  iamInstanceProfile: <REDACTED>
  insecureTransport: false
  instanceType: t3a.xlarge
  keypairName: <REDACTED>
  kmsKey: ""
  monitoring: false
  privateAddressOnly: false
  region: us-west-1
  requestSpotInstance: true
  retries: 5
  rootSize: 50
  secretKey: <REDACTED>
  securityGroup:
  - allopen-dualstack
  securityGroupReadonly: false
  sessionToken: ""
  spotPrice: 0.5
  sshKeyContents: ""
  sshUser: ubuntu
  subnetId: <REDACTED>
  tags: ""
  type: amazonec2Config
  useEbsOptimizedInstance: false
  usePrivateAddress: false
  userdata: ""
  volumeType: gp3
  vpcId: <REDACTED>
  zone: b
corralConfigs:
  corralConfigUser: izaac
  corralConfigVars:
    agent_count: 1
    aws_access_key: <REDACTED>
    aws_ami: <REDACTED>
    aws_domain: <REDACTED>
    aws_hostname_prefix: example1
    aws_ipv6_80_target_group_arn: <REDACTED>
    aws_ipv6_443_target_group_arn: <REDACTED>
    aws_region: us-west-1
    aws_route53_zone: <REDACTED>
    aws_secret_key: <REDACTED>
    aws_security_group: <REDACTED>
    aws_ssh_user: ubuntu
    aws_subnet: <REDACTED>
    aws_vpc: <REDACTED>
    aws_zone_id: <REDACTED>
    instance_type: t3a.xlarge
    registry_cert: < BASE64 of certificate from the QA certs confluence doc >
    registry_ecr_fqdn: <ID>.dkr.ecr.us-west-1.amazonaws.com
    registry_key: < BASE64 of certificate key from the QA certs confluence doc >
    server_count: 3
  corralSSHPath: /Users/izaac/.ssh/id_rsa.pub
corralPackages:
  corralPackageImages:
    # THESE paths will depend on the versions configured in the corral RANCHER and REGISTRY packages yaml config variables.
    # The /root/go/src/github.com/rancherlabs/corral-packages comes from the Dockerfile image
    corralecr: /root/go/src/github.com/rancherlabs/corral-packages/dist/aws-registry-standalone-ecr-2.15.1-2.7.7-rc4-1.11.0
    rancherha: /root/go/src/github.com/rancherlabs/corral-packages/dist/aws-aws-registry-standalone-rke2-rancher-calico-v1.24.16-rke2r1-global-2.15.1-2.7.7-rc4-1.11.0
    registryauthdisabled: /root/go/src/github.com/rancherlabs/corral-packages/dist/aws-registry-standalone-disabled-2.15.1-2.7.7-rc4-1.11.0
    registryauthenabled: /root/go/src/github.com/rancherlabs/corral-packages/dist/aws-registry-standalone-enabled-2.15.1-2.7.7-rc4-1.11.0
  hasCleanup: false
  hasDebug: true
corralRancherHA:
  name: rancherha
flags:
  # when there is expected for the automation to create the registries
  # desiredflags should be desiredflags: InstallRancher without UseExistingRegistries
  desiredflags: UseExistingRegistries
provisioningInput:
  cni:
  - calico
  k3sKubernetesVersion:
  - v1.25.12-k3s1
  nodeProviders:
  - ec2
  nodesAndRoles:
  - controlplane: true
    etcd: true
    quantity: 1
    worker: true
  nodesAndRolesRKE1:
  - controlplane: true
    etcd: true
    quantity: 3
    worker: true
  providers:
  - aws
  rke1KubernetesVersion:
  - v1.25.12-rancher1
  rke2KubernetesVersion:
  - v1.24.16-rke2r1
rancher:
  host: izrancher7.qa.rancher.space
  adminToken: < Automatically generated if the UseExistingRegistries flag is not set >
  adminPassword: 
  insecure: true
  cleanup: true
  caFile: ""
  caCerts: ""
  clusterName: auto-noauthregcluster-ezgrq
  shellImage: ""
registries:
  existingAuthRegistry: # Configuration for existing auth registry (optional)
    password: <REDACTED>
    url: <REDACTED>
    username: corral
  ecrRegistryConfig: # Configuration for existing ecr registry (optional)
    url: <ID>.dkr.ecr.us-west-1.amazonaws.com
    password: <REDACTED>
    awsAccessKeyId: <REDACTED>
    awsSecretAccessKey: <REDACTED>
  existingNoAuthRegistry: <REDACTED>
  registryConfigNames:
  - registryauthdisabled
  - registryauthenabled
  - corralecr
sshPath:
  sshPath: /go/src/github.com/rancher/rancher/tests/v2/validation/.ssh
```

### Jenkins job \*BRANCH\* and \*REPO\* variables

These variables can be modified if there's the need to use forked
repositories used for testing

### Manual steps that are done automatically by the Pipeline and Test Setup

The manual steps done by the GoLang framework section does in the test
suite is:

- clone the `corral-packages` repo
- Grab the yaml configurations retrieved from the Pipeline
    configuration parameters
- In the `corral-packages` repository directory do the following
  - `rm -rf dist`  this is not done as this directory doesn't exist.
        But this is for manual steps on a local machine.
  - `make build` this will generate the corral package images with
        the values set in the corral packages
- Start executing `corral create --debug <corral-configuration>` on
    each package required by the Golang registries test.
- Get the corral vars values and set these into the *CONFIG* yaml if
    applicable
- Execute the Golang tests

The setup at this point will look like the following:

### Test Setup Result either manual or automated

**The generated Rancher package and the Registry Standalone packages
`rancher_version` must match**

- Registry with Auth with images set in `rancher_version` variable of
    the `registry-standalone`  corral package
- Registry without Auth with images set in `rancher_version` variable
    of the `registry-standalone`  corral package
- ECR Registry configured for the `rancher_version` too
- A global registry without Auth for the Rancher install. - This
    depends on if the RANCHER corral package has a registry configured.

All of the above should be pre-existing if running the tests with the
`UseExistingRegistries` flag. The configuration information should also
exist in `CONFIG` if this is the case. Below the *registries* key.

``` java
registries:
  existingAuthRegistry: # Configuration for existing auth registry (optional)
    password: <<REDACTED>
    url: <REDACTED>
    username: corral # corral is the default user in the registry-standalone corral package
  ecrRegistryConfig: # Configuration for existing ecr registry (optional)
    url: <ID>.dkr.ecr.us-west-1.amazonaws.com
    password: <REDACTED>
    awsAccessKeyId: <REDACTED>
    awsSecretAccessKey: <REDACTED>
  existingNoAuthRegistry: <REDACTED>
```

### The information about the Private Registries and Rancher is ready to be used by Go

The variables information is stored on each Corral Package and ready to
be used using the Go Corral client configure the needed information for
the tests to run and validate.

The information includes and is automatically taken from the corral vars
by the corral client

Registry Standalone Package

- Private Registry fqdn, user and password if is a registry with auth
  - Or just the registry fqdn user if the private registry is No
        Auth
- Private Registry hosts/fqdn
- ECR - The user is always `AWS`, the CER fqdn and the ECR password

Ranche Registry package values configured in `CONFIG` and used for the
client login and the information set for the global registry if Rancher
use a Registry

- Rancher global registry fqdn
- The Rancher global registry is an empty string based in the Global
    Registry configuration value get by the test setup.
  - This is when using a Rancher Corral Package without a private
        registry

**These configuration variables keys in `CONFIG` is described in a
section below.**

## Go environment setup and validations

### Flags

These flags will determine some of the main logic depending on the setup
infrastructure used for the tests

``` yml
flags:
  desiredflags: UseExistingRegistries, InstallRancher
```

In the example above all the registries infrastructure needed is
existing and Rancher is installed by automation.

``` yml
flags:
  desiredFlags: InstallRancher
```

All registries as built by Automation and also Rancher.

``` yml
flags:
  desiredFlags: UseExistingRegistries
```

In the example above all the registries infrastructure needed is
existing and Rancher is also pre-existing

In this case the `rancher` section in `CONFIG` must be manually set. As
well as the `registries` section.

``` yml
rancher:
  host: 
  adminToken: 
  adminPassword: 
  insecure: true
  cleanup: true
```

### The registries configuration in CONFIG

Most of these values are automatically retrieved by the Corral client
when not using the `UseRegistries`  flag.

If using the `UseRegistries` flag all the information below is needed.

``` yml
registries:
  existingAuthRegistry:
    password: 
    url: <fqdn>
    username: corral
  ecrRegistryConfig:
    url: <ID>.dkr.ecr.us-west-1.amazonaws.com
    password: 
    awsAccessKeyId: 
    awsSecretAccessKey: 
  existingNoAuthRegistry: <fqdn>
  registryConfigNames:
  - registryauthdisabled
  - registryauthenabled
  - registryecr
```

### The Corral Variables configuration in CONFIG

These values are needed to generate the corral images configurations to
setup the RKE2 nodes, target groups, load balancer, route53 domain
registry and the registry nodes.

As well as the ECR fqdn.

``` yml
corralConfigs:
  corralConfigUser: izaac
  corralConfigVars:
    agent_count: 1
    aws_access_key: 
    aws_ami: 
    aws_domain: <REDACTED>
    aws_hostname_prefix: example1
    aws_ipv6_80_target_group_arn: 
    aws_ipv6_443_target_group_arn: 
    aws_region: us-west-1
    aws_route53_zone: <REDACTED>
    aws_secret_key: 
    aws_security_group: <REDACTED>
    aws_ssh_user: ec2-user
    aws_subnet: <REDACTED>
    aws_vpc: <REDACTED>
    aws_zone_id: 
    instance_type: t3a.xlarge
    registry_cert: <base64 string>
    registry_key: <base64 string>
    registry_ecr_fqdn: <ID>.dkr.ecr.us-west-1.amazonaws.com
    server_count: 3
```

### The Go Corral client will use the packages to retrieve the information needed for the Go Validations

`CONFIG`  section example of the Corral Package Images.

This should match the directory names generated by the `make build`
command in the `corral-packages`  repo.

- `rancherha`  is the Rancher package
- registryauthdisabled is the Registry without Auth
- registryauthenabled is the Registry with Auth
- corralecr is the ECR registry

The Global Registry is not needed as this is set in the `rancherha`
package configuration.

The versions configured on all packages must match.

- `rancher_version`
- cert_manager_version
- the `kubernetes_version`  must match with an RKE2 kubernetes version
    included in `rancher-images.txt`

``` yml
corralPackages:
  cleanup: false
  corralPackageImages:
    rancherha: /root/src/github.com/rancherlabs/corral-packages/dist/aws-aws-registry-standalone-rke2-rancher-calico-v1.23.10-rke2r1-global-2.15.1-2.7.0-1.11.0
    registryauthdisabled: /root/src/github.com/rancherlabs/corral-packages/dist/aws-registry-standalone-disabled-2.15.1-2.7.0-1.11.0
    registryauthenabled: /root/src/github.com/rancherlabs/corral-packages/dist/aws-registry-standalone-enabled-2.15.1-2.7.0-1.11.0
    corralecr: /root/src/github.com/rancherlabs/corral-packages/dist/aws-registry-standalone-ecr-2.15.1-2.7.0-1.11.0
```

The above paths should exist in the test container.

###  The provisioningInput used for the downstream clusters in CONFIG

All the kubernetes version should match with the included in
`rancher-images.txt`

``` yml
provisioningInput:
  cni:
  - calico
  k3sKubernetesVersion:
  - v1.24.7+k3s1
  nodeProviders:
  - ec2
  nodesAndRoles:
  - controlplane: true
    etcd: true
    quantity: 1
    worker: true
  nodesAndRolesRKE1:
  - controlplane: true
    etcd: true
    quantity: 3
    worker: true
  providers:
  - aws
  rke1KubernetesVersion:
  - v1.23.12-rancher1-1
  rke2KubernetesVersion:
  - v1.24.7+rke2r1
```

## Considerations for Testing or Debug ^^^ IMPORTANT ^^^

- Check All the versions for the different components used for the
    `rancher_version`  under test must have images in the
    `rancher-images.txt` file in the Rancher Release page for the
    version under test.
- If there was a KDM update after the `rancher_version` under test was
    released and a different or newer `kubernetes_version` is used.
  - We must add the `rke-tools` images that match the
        `kubernetes_version` manually in the registries.  
    - This information can be <a
            href="https://github.com/rancher/kontainer-driver-metadata/blob/dev-v2.7/pkg/rke/k8s_rke_system_images.go"
            rel="nofollow">found in the KDM code</a>
  - It is recommended to do this using the `UseExistingRegistries`
        flag in CONFIG  
    - Create registries with the standalone-registry corral
            package and use the existing registries flag in CONFIG
- To debug if the `corralPackageImages` filesystem path values in
    CONFIG are correct this can be debugged by editing
    `setup_environment.sh` in `tests/v2/validation/pipeline/scripts`
  - This is in the `rancher/rancher` repo
  - Adding a directory list ( ls ) command after the line:
        `sh tests/v2/validation/pipeline/scripts/build_corral_packages.sh`
    - `For example: ls /root/src/github.com/rancherlabs/corral-packages/dist`
- If the `Jenkinsfile.e2e`  needs some update as part of a code
    change. In order to test the changes the repo url and branch should
    be changes to the fork's url and branch. In the Job setting not the
    job variables.

## USE CASE

Configuration for testing Rancher
<a href="https://github.com/rancher/rancher/releases/tag/v2.7.1"
rel="nofollow">v2.7.1</a>

- Collect the `rancher-images.txt`  from the Rancher release page for
    that version.
- Analyze the RKE2, K3s, RKE1 kubernetes versions to configure
    RANCHER_CORRAL_CONFIG and CONFIG versions
  - Example RKE1 - hyperkube:v1.24.9-rancher1
    - Search the <a
            href="https://github.com/rancher/kontainer-driver-metadata/blob/dev-v2.7/pkg/rke/k8s_rke_system_images.go"
            rel="nofollow">KDM</a> to obtain the exact version string
            for that image.
    - This result in `v1.24.9-rancher1-1`
  - Example RKE2 - rke2-runtime:v1.23.13-rke2r1
    - Results in: `v1.23.13-rke2r1`
  - Example K3s - system-agent-installer-k3s:v1.23.15-k3s1
    - Results in: `v1.23.15-k3s1`

With the k8s versions under tests are identified it's time to configure
the YAML configurations.

RANCHER_CORRAL_CONFIG

The Rancher deployment only works with RKE2 as per the current corral
package support. The kubernetes_version should be an RKE2 and make sure
it is added with a + not a - before the suffix.

- `v1.23.13-rke2r1` NO
- `v1.23.13+rke2r1` YES

*This configuration can be used to create registries before automation
is executed. This can save time for multiple runs by using the
UseExistingRegistries flag. In that case this YAML with be ignored by
the Jenkins run.  
*

``` java
manifest:
  name: rancher-registry
  description: rancher-registry
  variables:
    registry_auth:
      default: enabled
templates:
  - aws/cluster_nodes
  - aws/registry_nodes
  - registry-standalone
  - rke2
  - rancher
variables:
  cni:
    - calico
  kubernetes_version:
    - v1.23.13+rke2r1
  registry_auth:
    - global
  docker_compose_version:
    - 2.15.1
  rancher_version:
    - 2.7.1
  cert_manager_version:
    - 1.11.0
```

That will create a corral image folder:
aws-aws-registry-standalone-rke2-rancher-calico-v1.23.13-rke2r1-global-2.15.1-2.7.1-1.11.0

REGISTRIES_CORRAL_CONFIG

We only need the Rancher version here and update docker compose and
cert-manager as needed.

*This configuration can be used to create registries before automation
is executed. This can save time for multiple runs by using the
UseExistingRegistries flag. In that case this YAML with be ignored by
the Jenkins run.*

``` java
manifest:
  name: registry
  description: registry
  variables:
    registry_auth:
      default: enabled
templates:
  - aws/registry_nodes
  - registry-standalone
variables:
  registry_auth:
    - global
    - enabled
    - disabled
    - ecr
  docker_compose_version:
    - 2.15.1
  rancher_version:
    - 2.7.1
  cert_manager_version:
    - 1.11.0
```

CONFIG

This is an excerpt of the configuration options that need an update to
match the above configuration values.

The `corralPackageImages` are basically folders that reflect the corral
package variables values in their name. This must change to match the
expected folder name for the configuration of RANCHER_CORRAL_CONFIG and
REGISTRIES_CORRAL_CONFIG

The below code give an example based on the previous configurations.

The `provisioningInput`  configuration block is also very important for
the downstream clusters provisioning for testing. The Kubernetes
versions must exist in the rancher-images.txt like the RKE2 one used for
the Rancher package and checked in KDM.

For this example on `v2.7.1` the kubernetes versions in the
`provisioningInput` were confirmed they exist in the rancher-images.txt

The values in the `registries`  block are optional but needed if the
`UseExistingRegistries`  flag is used in CONFIG

- existingAuthRegistry
- existingNoAuthRegistry

ecrRegistryConfig is needed for RKE1 ecr tests.

``` java
 corralPackages:
  corralPackageImages:
    # THESE paths will depend on the versions configured in the corral RANCHER and REGISTRY packages yaml config variables.
    # The /root/go/src/github.com/rancherlabs/corral-packages comes from the Dockerfile image
    corralecr: /root/go/src/github.com/rancherlabs/corral-packages/dist/aws-registry-standalone-ecr-2.15.1-2.7.1-1.11.0
    rancherha: /root/go/src/github.com/rancherlabs/corral-packages/dist/aws-aws-registry-standalone-rke2-rancher-calico-v1.23.13-rke2r1-global-2.15.1-2.7.1-1.11.0
    registryauthdisabled: /root/go/src/github.com/rancherlabs/corral-packages/dist/aws-registry-standalone-disabled-2.15.1-2.7.1-1.11.0
    registryauthenabled: /root/go/src/github.com/rancherlabs/corral-packages/dist/aws-registry-standalone-enabled-2.15.1-2.7.1-1.11.0
  hasCleanup: false
  hasDebug: true
corralRancherHA:
  name: rancherha
flags:
  # when there is expected for the automation to create the registries
  # desiredflags should be desiredflags: InstallRancher without UseExistingRegistries
  desiredflags: UseExistingRegistries
provisioningInput:
  cni:
  - calico
  k3sKubernetesVersion:
  - v1.23.15-k3s1 
  nodeProviders:
  - ec2
  nodesAndRoles:
  - controlplane: true
    etcd: true
    quantity: 1
    worker: true
  nodesAndRolesRKE1:
  - controlplane: true
    etcd: true
    quantity: 3
    worker: true
  providers:
  - aws
  rke1KubernetesVersion:
  - v1.24.9-rancher1-1 
  rke2KubernetesVersion:
  - v1.23.13-rke2r1 
rancher:
  host: izrancher7.qa.rancher.space
  adminToken: < Automatically generated if the UseExistingRegistries flag is not set >
  adminPassword: 
  insecure: true
  cleanup: true
  caFile: ""
  caCerts: ""
  clusterName: auto-noauthregcluster-ezgrq
  shellImage: ""
registries:
  existingAuthRegistry: # Configuration for existing auth registry (optional)
    password: e04090bb395d
    url: <REDACTED>
    username: corral
  ecrRegistryConfig: # Configuration for existing ecr registry (optional)
    url: <ID>.dkr.ecr.us-west-1.amazonaws.com
    password: <REDACTED>
    awsAccessKeyId: <REDACTED>
    awsSecretAccessKey: <REDACTED>
  existingNoAuthRegistry: <REDACTED>
  registryConfigNames:
  - registryauthdisabled
  - registryauthenabled
  - corralecr
```

GOTEST_TESTCASE

This Jenkins job variable allows to run an specific test in the test
suite. By default it runs everything

- -run ^TestRegistryTestSuite$

But that can be narrowed to an specific, for example K3S only

- -run ^TestRegistryTestSuite/TestRegistryK3S$
