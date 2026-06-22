# Integration Tests

## Running Tests

### Method 1: Full CI Run (Recommended for Validation)

To run the integration tests in `tests/v2/integration`, use

```shell
make ci
```

This will run `scripts/test` inside the Rancher runtime container built from `Dockerfile.runtime`. The script
will set up and run Rancher. Upon startup, Rancher will create a local cluster using k3s and deploy CRDs to it. Once
the Rancher and the local cluster are ready, the tests will be run.

This _should_ work on Mac and Linux systems out of the box, at least in theory. The whole integration test process does
consume a fair bit of CPU and memory. If you experience unexpected timeouts, you may not have enough compute power. If
you encounter OOM issues that affect scheduling of containers, you may not have enough memory.

### Method 2: Running Locally Against an External Rancher

This method is useful for iterative development — you point the tests at an already-running Rancher instance instead of
spinning up a new one inside a container.

#### Quick Start (copy-paste)

If you already have a Rancher server running and just want to get tests going:

```bash
# 1. Start Rancher (if not already running)
export RANCHER_IP=$(ifconfig | grep 'inet ' | grep -v 127.0.0.1 | head -1 | awk '{print $2}')
docker run -d --name rancher-server --restart=unless-stopped \
  -p 80:80 -p 443:443 --privileged \
  -e CATTLE_SERVER_URL="https://${RANCHER_IP}" \
  -e CATTLE_BOOTSTRAP_PASSWORD="admin" \
  -e CATTLE_DEV_MODE="yes" \
  -e CATTLE_AGENT_IMAGE="rancher/rancher-agent:v2.14-head" \
  rancher/rancher:v2.14-head

# 2. Create a k3d downstream cluster and generate config.yaml
export CATTLE_BOOTSTRAP_PASSWORD="admin"
export CATTLE_AGENT_IMAGE="rancher/rancher-agent:v2.14-head"
make integration-setup

# 3. Run the tests
make integration-test-local
```

`make integration-setup` builds the setup binary, connects to Rancher, creates a k3d downstream cluster, and writes
`tests/v2/integration/config.yaml`. `make integration-test-local` reads that file and runs the full test suite.

If you already have `config.yaml` from a previous setup run, you can skip straight to step 3.

#### Step 1: Start a Rancher Server

```bash
export RANCHER_IP=$(ifconfig | grep 'inet ' | grep -v 127.0.0.1 | head -1 | awk '{print $2}')

docker run -d --name rancher-server --restart=unless-stopped \
  -p 80:80 -p 443:443 \
  --privileged \
  -e CATTLE_SERVER_URL="https://${RANCHER_IP}" \
  -e CATTLE_BOOTSTRAP_PASSWORD="admin" \
  -e CATTLE_DEV_MODE="yes" \
  -e CATTLE_AGENT_IMAGE="rancher/rancher-agent:v2.14-head" \
  rancher/rancher:v2.14-head
```

Wait for Rancher to be healthy:

```bash
until curl -sk "https://${RANCHER_IP}/ping" | grep -q pong; do echo "waiting for Rancher..."; sleep 5; done
```

#### Step 2: Build and Run the Integration Setup

The setup program connects to Rancher, creates a k3d downstream cluster, imports it, and writes the test config file.

```bash
# Build the setup binary (from repo root)
cd tests/v2/integration
./scripts/build-integration-setup
# Produces: tests/v2/integration/bin/integrationsetup

# Run the setup (from repo root)
cd ../../..
export CATTLE_BOOTSTRAP_PASSWORD="admin"
export CATTLE_AGENT_IMAGE="rancher/rancher-agent:v2.14-head"
export CATTLE_TEST_CONFIG=$(pwd)/tests/v2/integration/config.yaml

./tests/v2/integration/bin/integrationsetup
```

The setup program auto-detects the Rancher host IP, generates an admin token, creates a k3d cluster, imports it into
Rancher, and writes connection details to the path specified by `CATTLE_TEST_CONFIG`.

#### Step 3: (Optional) Create `config.yaml` Manually

If you already have a Rancher instance with an imported cluster, you can skip the setup binary and create the config
file directly. See [Configuration Reference](#configuration-reference) below for all fields.

#### Step 4: Run the Tests

```bash
export CATTLE_TEST_CONFIG=$(pwd)/tests/v2/integration/config.yaml

# Run all integration tests
go test -v -timeout 30m -failfast -p 1 ./tests/v2/integration/...

# Run a specific test suite
go test -v -count=1 -timeout 30m -run TestChartsTestSuite ./tests/v2/integration/catalogv2/

# Run a specific test within a suite
go test -v -count=1 -run TestRTBTestSuite/TestUserVsUserBaseGlobalRoleVisibility ./tests/v2/integration/rbac/

# Run Steve API tests (local cluster only — no downstream cluster needed)
go test -v -count=1 -run TestSteveLocal ./tests/v2/integration/steveapi/
```

### Common `go test` Flags

| Flag | Example | Description |
|---|---|---|
| `-timeout` | `-timeout 30m` | Hard deadline for the entire test binary. The default is **10 minutes**, which is too short for catalog tests that pull external repositories. Use `30m` for full suite runs. |
| `-run` | `-run TestChartsTestSuite` | Run only tests/suites matching the regex. Supports `/` to select a sub-test: `-run Suite/TestName`. |
| `-count` | `-count=1` | Disable test result caching. Always use `-count=1` when running integration tests to ensure a fresh run. |
| `-v` | `-v` | Verbose output — prints each test name and PASS/FAIL as it runs. Useful for spotting which test hangs. |
| `-failfast` | `-failfast` | Stop the run after the first test failure. Used in CI to avoid wasting time once something breaks. |
| `-p` | `-p 1` | Number of test packages to build and run in parallel. Must be `1` for integration tests to avoid resource conflicts. |

---

## Configuration Reference

### `config.yaml`

The test config file is read from the path in `CATTLE_TEST_CONFIG`. A minimal example:

```yaml
rancher:
  adminToken: "token-xxxxx:yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy"
  host: "192.168.1.100"        # Rancher host (no https://, no trailing slash)
  clusterName: "my-k3d-cluster" # name of the imported downstream cluster in Rancher
  insecure: true
  cleanup: true
```

All supported fields:

| Field | Type | Description | Required |
|---|---|---|---|
| `rancher.adminToken` | string | Bearer token for admin API access. Obtain from Rancher UI: User Icon → Account & API Keys → Create API Key (No Scope). | Yes |
| `rancher.host` | string | Rancher server hostname or IP (no scheme, no trailing slash). | Yes |
| `rancher.clusterName` | string | Name of the downstream cluster in Rancher. Required for downstream tests; use `local` to target the local cluster only. | Yes |
| `rancher.insecure` | bool | Skip TLS verification. Useful for self-signed certs. Default: `false`. | No |
| `rancher.cleanup` | bool | Whether tests should delete the resources they create. Default: `true`. | No |
| `rancher.adminPassword` | string | Admin password (alternative to `adminToken`). | No |
| `rancher.caFile` | string | Path to a CA certificate file for TLS verification. | No |
| `rancher.caCerts` | string | Inline PEM-encoded CA certificate(s). | No |

### Environment Variables

| Variable | Used By | Description |
|---|---|---|
| `CATTLE_TEST_CONFIG` | Tests + setup | **Required.** Absolute path to `config.yaml`. Must be exported before running tests or the setup binary. |
| `CATTLE_BOOTSTRAP_PASSWORD` | Setup only | Bootstrap password for Rancher first-login. Default: `admin`. |
| `CATTLE_AGENT_IMAGE` | Setup only | Full image reference for the Rancher agent (e.g. `rancher/rancher-agent:v2.14-head`). Used when importing the k3d cluster. |
| `CATTLE_RANCHER_HOST` | Setup only | Override the auto-detected Rancher host (e.g. `192.168.1.100:443` or `localhost:8443`). If unset, the setup binary detects the host by inspecting the machine's outbound IP. |

---

## Test Suites

Tests that only use the `local` cluster do **not** require a downstream cluster and can be run with just a basic
`config.yaml`. Tests marked "downstream required" need an imported cluster referenced by `rancher.clusterName`.

| Directory | Test Function | What It Tests | Downstream Required? |
|---|---|---|---|
| `catalogv2/` | `TestChartsTestSuite` | Chart installation, tolerations, pull-through | Yes |
| `catalogv2/` | `TestClusterRepoTestSuite` | ClusterRepo CRUD, OCI repos | No |
| `catalogv2/` | `TestSystemChartsVersionSuite` | System chart version constraints | No |
| `catalogv2/` | `TestUIPluginSuite` | UI plugin extensions | No |
| `catalogv2/` | `TestRancherManagedChartsSuite` | Rancher-managed Helm charts | No |
| `clusters/` | `TestK8sProxy` | K8s API proxy through Rancher | Yes |
| `projects/` | `TestResourceQuotaTestSuite` | Namespace resource quotas | No |
| `projects/` | `TestProjectUserTestSuite` | Project-level user access | No |
| `rbac/` | `TestRTBTestSuite` | Role/ClusterRole template bindings, features, impersonation, projects | No (uses `local`) |
| `steveapi/` | `TestSteveLocal` | Steve resource listing API (local cluster) | No |
| `steveapi/` | `TestSteveDownstream` | Steve API on downstream cluster | Yes (currently skipped) |
| `users/` | `TestUserTestSuite` | User CRUD operations | No |
| `authconfigs/` | `TestAuthConfig` | Auth configuration management | No |
| `serviceaccount/` | `TestSATestSuite` | Service account token handling | No |

---

## Test Setup Details

Setup for the integration tests can be found in `scripts/test` and `tests/v2/integration/setup/main.go`. The latter is
responsible primarily for
1. Generating and saving a test config file that will be used by the integration tests.
2. Creating a user and corresponding token with which to access Rancher from tests.
3. Creating a new test namespace in the local cluster to which credentials for Docker container registries will be 
deployed in the form of secrets.
4. Deploying two registries to the `default` namespace. 

### Registry Setup 
The first of the two registries deployed during the setup process is a configured normally, so images can be pushed to 
and pulled from it. The second registry is configured as a pull-through cache, and its sole purpose is to cache 
images pulled by downstream clusters when they create containers in order to speed up the test process. The process of 
creating these registries will also result in secrets being deployed to the aforementioned test namespace. The cattle
cluster agent image built locally (by `scripts/ci`) is then pushed to the first registry, so it can be pulled by 
downstream clusters. Configuration for these two registries is merged together and used to create a test cluster that
is used in integration tests. The merged registry config is what allows the downstream cluster to access the registries
inside the local cluster.

### Downstream Cluster Provisioning

The downstream cluster is created the same way in the integration test setup as it would be in v2 provisioning tests.
We create the downstream cluster using the provided `cluster.New()` function in 
`github.com/rancher/rancher/tests/v2prov/cluster`. This, in turn, uses Rancher's v2 provisioning functionality to create
a container in the test namespace that runs a machine provisioner. This machine provisioner will create a
[`systemd-node`](https://github.com/rancher/systemd-node/tree/master) container that will create its own contained
Kubernetes cluster. In other words, the end result is
- A Docker container running the Rancher runtime environment
  - `scripts/test` running
    - Integration tests
    - Rancher
    - Rancher's "local" cluster: a k3s cluster running
      - A number of containers (for stuff like networking, but Rancher-specific things like the rancher-webhook)
      - A `systemd-node` container running
        - The "downstream cluster": a k3s cluster running the cluster agent and some other Rancher downstream components
