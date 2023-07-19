# Integration Tests

## Running Tests

To run the integration tests in `tests/v2/integration`, use

```shell
make ci
```

This will run `scripts/test` inside a Dapper container created using `Dockerfile.dapper`. The script will set up and run
Rancher. Upon startup, Rancher will create a local cluster using k3s and deploy CRDs to it. Once the Rancher and the
local cluster are ready, the tests will be run.

This _should_ work on Mac and Linux systems out of the box, at least in theory. The whole integration test process does
consume a fair bit of CPU and memory. If you experience unexpected timeouts, you may not have enough compute power. If
you encounter OOM issues that affect scheduling of containers, you may not have enough memory.

## Test Setup

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
- A Docker container (created by Dapper) running
  - `scripts/test` running
    - Integration tests
    - Rancher
    - Rancher's "local" cluster: a k3s cluster running
      - A number of containers (for stuff like networking, but Rancher-specific things like the rancher-webhook)
      - A `systemd-node` container running
        - The "downstream cluster": a k3s cluster running the cluster agent and some other Rancher downstream components
