# Secret Encryption Configs

## Getting Started
Your GO suite should be set to `-run ^TestK3sSecretEncryption/TestK3sSecretEncryptionEnabled$` or `-run ^TestK3sSecretEncryption/TestK3sSecretEncryptionDisabled$` for K3S clusters and `-run ^TestRKE2SecretEncryption` for RKE2 clusters. You can find any additional suite name(s) by checking the test file you plan to run.

In your config file, set the following:
```json
"rancher": { 
  "host": "rancher_server_address",
  "adminToken": "rancher_admin_token"
  "clusterName": "<cluster-to-run-test>"
},
"sshConfig": {
  "user": "<ssh-user-of-cluster-nodes>"
}
```

Typically, a cluster with the following 3 pools is used for testing:
```yaml
{
  {
    ControlPlane: true,
    Quantity:     1,
  },
  {
    Etcd:     true,
    Quantity: 1,
  },
  {
    Worker:   true,
    Quantity: 1,
  },
}
```

These tests are designed to accept an existing cluster that the user has access to. If you do not have a downstream cluster in rancher, you should create one first before running this test. 