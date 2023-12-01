# Rbac

## Getting Started
Your GO suite should be set to `-run ^Test<>TestSuite$`. For example to run the cluster_role_test.go, set the GO suite to `-run ^TestClusterRoleTestSuite$` You can find specific tests by checking the test file you plan to run.
Config needed for each of the suites cluster_role_test.go, project_role_test.go and restrictedadmin_role_test.go require the following config:

```json
"rancher": { 
  "host": "rancher_server_address",
  "adminToken": "rancher_admin_token",
  "clusterName": "cluster_to_run_tests_on",
  "insecure": true/optional,
  "cleanup": false/optional,
}
```

For the restrictedadmin_role_test.go run, we need the following additional paramters to be passed in the config file as we create a downstream cluster. We require rke1 custom cluster config as the test creates an RKE1 cluster: 
provisioningInput:
  nodePools:
    - nodeRoles:
        controlplane: true
        etcd: true
        quantity: 1
        worker: true
  cni:
    - calico
  providers:
    - aws
  nodeProviders:
    - ec2
  hardened: false
  psact: ""
awsEC2Configs:
  region: us-east-2
  awsSecretAccessKey: <Your Secret Key>
  awsAccessKeyID: <Your Access Key>
  awsEC2Config:
    - instanceType: t3a.xlarge
      awsRegionAZ: ""
      awsAMI: <Your AMI>
      awsSecurityGroups:
        - <Your SG>
      awsSSHKeyName: <Your Key>
      awsIAMProfile: 
      awsUser: ubuntu
      volumeSize: 100
      roles:
        - etcd
        - controlplane
        - worker
sshPath:
  sshPath: <Your ssh path>

For more info, please use the following links to continue adding to your config for provisioning tests:
 [Define your test](../provisioning/rke1/README.md#provisioning-input)
(#Provisioning-Input)


