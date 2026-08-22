# v2prov Integration Tests

These are a set of provisioning-v2 integration tests.

These tests are designed to be run on a single-node cluster (due to the usage of HostPath volumes), and perform e2e validations of v2prov + CAPR using systemd-node.

## Test Directory Structure

```
tests/v2prov/
├── clients/          # Kubernetes and Rancher client initialization
├── cluster/          # Cluster lifecycle management helpers
├── defaults/         # Default values and constants for tests
├── namespace/        # Namespace management utilities
├── nodeconfig/       # Node configuration helpers
├── objectstore/      # Minio object store for S3 snapshot tests
├── operations/       # Day 2 operation test helpers (snapshots, rotation, scaling)
├── registry/         # Container registry cache configuration
├── systemdnode/      # Systemd-node pod creation for custom clusters
├── tests/            # Test implementations organized by category
│   ├── autoscaler/       # Autoscaling field validation tests
│   ├── custom/           # Custom cluster provisioning and operations
│   ├── fleet/            # Fleet cluster integration tests
│   ├── general/          # General system setting validation tests
│   ├── machineprovisioning/ # Machine-provisioned cluster tests
│   ├── prebootstrap/     # Pre-bootstrap secret synchronization tests
│   └── testhelpers/      # Shared test helper functions
├── utils/            # General utility functions
└── wait/             # Wait and polling utilities
```

## Test Categories

There are several categories of tests contained within this test suite:

### General

General tests validate system-level settings, including:
- System agent version verification
- WINS agent version verification
- CSI proxy agent version verification

### Provisioning_MP

Machine-provisioned cluster tests that use node pools (MachinePools) to manage infrastructure. These tests verify:
- Single and multi-node cluster creation/deletion
- Machine template annotation tracking
- Machine set delete policies
- Node scaling operations
- Drain hooks and strategies

### Provisioning_Custom

Custom cluster tests where systemd-node pods are manually created to simulate node registration. These tests verify:
- One, three, and five node custom clusters
- Role assignment (worker, controlplane, etcd)
- Node labels and taints
- Cluster deletion and cleanup

### Operation_SetA (MP and Custom)

Day 2 operation tests that run in the first operation test set:
- Certificate rotation
- Encryption key rotation (RKE2 only)
- Etcd snapshot creation and in-place restore

### Operation_SetB (MP and Custom)

Day 2 operation tests that run in the second operation test set:
- Etcd snapshot operations with node replacement
- S3-based etcd snapshot storage
- Multi-etcd node snapshot operations

### Fleet

Fleet cluster integration tests:
- Local cluster bootstrap verification
- Downstream cluster Fleet registration
- Agent deployment customization propagation

### PreBootstrap

Pre-bootstrap provisioning flow tests:
- Secret synchronization with cluster variable substitution
- Authorized Cluster Endpoint (ACE) configuration

### Autoscaler

Cluster autoscaler field validation tests:
- RKE machine pool autoscaling min/max size validation
- Create and update validation rules

## Test Naming Format

Within the `tests` folder, tests follow a naming convention to ensure proper organization in CI pipeline stages:

`Test_<Category>_<TestName>`

Examples:
- `Test_General_SystemAgentVersion`
- `Test_Provisioning_MP_SingleNodeAllRolesWithDelete`
- `Test_Provisioning_Custom_ThreeNode`
- `Test_Operation_SetA_MP_CertificateRotation`
- `Test_Operation_SetB_Custom_EtcdSnapshotOperationsOnNewNode`
- `Test_Fleet_ClusterBootstrap`
- `Test_PreBootstrap_Provisioning_Flow`

The `SetA` and `SetB` suffixes for Operation tests help distribute resource-intensive tests across pipeline stages.

## Helper Functions

The `tests/testhelpers` package provides common utilities to reduce code duplication:

- `NewTestClients` - Creates a test client with automatic cleanup
- `CreateCustomClusterNode` - Creates systemd nodes with configurable roles, labels, and taints
- `CreateCustomClusterWithNodes` - Convenience function for simple custom cluster setup
- `WaitForClusterReady` - Waits for cluster readiness with machine count verification
- `EnsureMinimalConflicts` - Validates acceptable conflict message thresholds
- `DeleteClusterAndWait` - Handles cluster deletion with cleanup verification
- `NewTestConfigMap` - Creates uniquely named ConfigMaps for testing
- `GetCustomCommand` - Retrieves and validates custom cluster registration command

## Running Locally

You can run these tests locally on a machine that can support running systemd within containers.

If you invoke `make provisioning-tests`, it will run all of the provisioning/general tests.

You can customize the tests you run through the use of the environment variables: `V2PROV_TEST_RUN_REGEX` and `V2PROV_TEST_DIST`.

- `V2PROV_TEST_DIST` can be either `k3s` (default) or `rke2`
- `V2PROV_TEST_RUN_REGEX` is a regex string indicating the pattern of tests to match

### Examples

Run a specific operation test with RKE2:
```bash
V2PROV_TEST_RUN_REGEX=Test_Operation_SetA_Custom_EncryptionKeyRotation V2PROV_TEST_DIST=rke2 make provisioning-tests
```

Run all custom provisioning tests with K3s:
```bash
V2PROV_TEST_RUN_REGEX=Test_Provisioning_Custom make provisioning-tests
```

Run all general tests:
```bash
V2PROV_TEST_RUN_REGEX=Test_General make provisioning-tests
```

Run all SetA operation tests:
```bash
V2PROV_TEST_RUN_REGEX=Test_Operation_SetA make provisioning-tests
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `V2PROV_TEST_DIST` | Kubernetes distribution (`k3s` or `rke2`) | `k3s` |
| `V2PROV_TEST_RUN_REGEX` | Regex pattern to filter tests | (runs all) |
| `SOME_K8S_VERSION` | Kubernetes version to use for tests | (from defaults) |
| `CATTLE_SYSTEM_AGENT_VERSION` | Expected system agent version | (required for general tests) |
| `CATTLE_WINS_AGENT_VERSION` | Expected WINS agent version | (required for general tests) |
| `CATTLE_CSI_PROXY_AGENT_VERSION` | Expected CSI proxy agent version | (required for general tests) |

## Notes

- Some tests are skipped for RKE2 distributions (marked in test documentation)
- Encryption key rotation tests only run on RKE2
- Tests use the `cluster.SaneConflictMessageThreshold` to validate acceptable levels of conflict errors during provisioning
