# Integration Test Setup

This program sets up a local environment for running Rancher integration tests. It performs the following actions:

1.  Connects to a local Rancher server instance.
2.  Creates a new k3d cluster.
3.  Imports the k3d cluster into the Rancher server.
4.  Lists the deployments in the imported cluster to verify the connection.

## Prerequisites

*   [Go](https://golang.org/doc/install) (version 1.24 or higher, see `go.mod` in the root directory)
*   Docker
*   k3d (v5.8.3 or compatible)

You can install `k3d` with the following command:
```bash
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=v5.8.3 bash
```

## Running Locally

### 1. Build Rancher Images (Optional)

If you are developing Rancher, you should build your own `rancher/rancher` and `rancher/rancher-agent` images.

To get the correct versions for dependencies, you can source the `export-config` script. You will also need the git commit hash.
```bash
# From the root of the rancher/rancher repository
source ./scripts/export-config
export COMMIT=$(git rev-parse --short HEAD)
export DEV_TAG=dev # or any other tag for your development images
export ARCH=amd64 # or arm64
export RANCHER_REPO=rancher # or your custom docker repo
```

Example build command for the `rancher-agent` image:
```bash
make quick-agent
```

Example build command for the `rancher` server image:
```bash
make quick-server
```

If you are not actively developing Rancher, you can use pre-existing images from Docker Hub, like `rancher/rancher:stable` and `rancher/rancher-agent:stable`.

### 2. Start Rancher Server

Start a Rancher server instance in a Docker container. This program will connect to this server.

First, determine your machine's primary IP address. This will be used for the `CATTLE_SERVER_URL`.
```bash
export RANCHER_IP=$(ip route get 8.8.8.8 | awk '{print $7}')
echo "Your Rancher IP is: $RANCHER_IP"
```

Now, start the Rancher server. Replace `_YOUR_IP_ADDRESS_` with the IP from the previous step.
```bash
# Set the Rancher and Agent image tags you want to test
export RANCHER_IMAGE_TAG=stable # or your custom tag e.g., 'dev'
export RANCHER_AGENT_IMAGE_TAG=stable # or your custom tag e.g., 'dev'

docker run -d --name rancher-server --restart=unless-stopped \
  -p 80:80 -p 443:443 \
  --privileged \
  -e CATTLE_SERVER_URL="https://_YOUR_IP_ADDRESS_" \
  -e CATTLE_BOOTSTRAP_PASSWORD="admin" \
  -e CATTLE_DEV_MODE="yes" \
  -e CATTLE_AGENT_IMAGE="rancher/rancher-agent:${RANCHER_AGENT_IMAGE_TAG}" \
  rancher/rancher:${RANCHER_IMAGE_TAG}
```

Wait a few minutes for Rancher to start up. You can check the logs with `docker logs -f rancher-server`.

### 3. Build the Setup Binary

Navigate to this directory and build the program.

```bash
# From the root of the rancher/rancher repository
cd tests/v2/integration/setup
go build -o setup .
```

### 4. Run the Setup Program

With the Rancher server running, execute the setup binary. Make sure to set the required environment variables.

```bash
# From the tests/v2/integration/setup directory
export CATTLE_BOOTSTRAP_PASSWORD="admin"
export CATTLE_AGENT_IMAGE="rancher/rancher-agent:stable" # or your custom tag

./setup
```

The program will output the progress of creating and importing the k3d cluster, and finally list the deployments found in the new cluster.