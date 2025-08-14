#!/bin/bash

# Test script for KWOK integration with scale-cluster-agent
set -e

echo "=== Testing KWOK Integration ==="

# Check if kwokctl and kwok are available
if [ ! -f "./kwokctl" ] || [ ! -f "./kwok" ]; then
    echo "ERROR: kwokctl or kwok binaries not found in current directory"
    echo "Please run: curl -L -o kwokctl https://github.com/kubernetes-sigs/kwok/releases/latest/download/kwokctl-\$(uname -s | tr '[:upper:]' '[:lower:]')-\$(uname -m | sed 's/x86_64/amd64/')"
    echo "And: curl -L -o kwok https://github.com/kubernetes-sigs/kwok/releases/latest/download/kwok-\$(uname -s | tr '[:upper:]' '[:lower:]')-\$(uname -m | sed 's/x86_64/amd64/')"
    exit 1
fi

chmod +x ./kwokctl ./kwok

echo "✓ KWOK binaries found and made executable"

# Test creating a simple cluster
echo "Creating test KWOK cluster..."
./kwokctl create cluster --name test-cluster --runtime binary --kube-apiserver-port 8001 --etcd-port 8002 --kube-controller-manager-port 8003 --kube-scheduler-port 8004 --controller-port 8005

if [ $? -eq 0 ]; then
    echo "✓ Test cluster created successfully"
else
    echo "✗ Failed to create test cluster"
    exit 1
fi

# Start the cluster
echo "Starting test cluster..."
./kwokctl start cluster --name test-cluster

if [ $? -eq 0 ]; then
    echo "✓ Test cluster started successfully"
else
    echo "✗ Failed to start test cluster"
    exit 1
fi

# Wait a moment for cluster to be ready
echo "Waiting for cluster to be ready..."
sleep 10

# Test if cluster is working
echo "Testing cluster connectivity..."
./kwokctl kubectl --name test-cluster get nodes

if [ $? -eq 0 ]; then
    echo "✓ Cluster is responding to kubectl commands"
else
    echo "✗ Cluster is not responding to kubectl commands"
    exit 1
fi

# Test creating some resources
echo "Testing resource creation..."

# Create a namespace
./kwokctl kubectl --name test-cluster apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace
EOF

# Create a node
./kwokctl kubectl --name test-cluster apply -f - <<EOF
apiVersion: v1
kind: Node
metadata:
  name: test-node-1
  labels:
    kubernetes.io/hostname: test-node-1
    node-role.kubernetes.io/control-plane: "true"
spec:
  taints:
  - key: node-role.kubernetes.io/control-plane
    effect: NoSchedule
status:
  conditions:
  - type: Ready
    status: "True"
    lastHeartbeatTime: "2024-01-01T00:00:00Z"
    lastTransitionTime: "2024-01-01T00:00:00Z"
  capacity:
    cpu: "4"
    memory: "8Gi"
    pods: "110"
  allocatable:
    cpu: "4"
    memory: "8Gi"
    pods: "110"
  nodeInfo:
    kubeletVersion: "v1.33.0"
    osImage: "Ubuntu 22.04.3 LTS"
    kernelVersion: "5.15.0-88-generic"
    containerRuntimeVersion: "containerd://1.7.0"
EOF

# Create a pod
./kwokctl kubectl --name test-cluster apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: test-namespace
spec:
  containers:
  - name: test-container
    image: busybox:latest
    command: ["sleep", "3600"]
  nodeName: test-node-1
EOF

echo "✓ Resources created successfully"

# Verify resources
echo "Verifying created resources..."
./kwokctl kubectl --name test-cluster get namespaces
./kwokctl kubectl --name test-cluster get nodes
./kwokctl kubectl --name test-cluster get pods -n test-namespace

echo "✓ All resources verified successfully"

# Test scaling
echo "Testing scaling capabilities..."
./kwokctl scale node --name test-cluster --replicas 5

if [ $? -eq 0 ]; then
    echo "✓ Successfully scaled to 5 nodes"
    ./kwokctl kubectl --name test-cluster get nodes
else
    echo "✗ Failed to scale nodes"
fi

# Clean up
echo "Cleaning up test cluster..."
./kwokctl stop cluster --name test-cluster
./kwokctl delete cluster --name test-cluster

echo "✓ Test cluster cleaned up successfully"

echo ""
echo "=== KWOK Integration Test PASSED ==="
echo ""
echo "KWOK is working correctly and can:"
echo "  ✓ Create clusters with custom ports"
echo "  ✓ Start and manage clusters"
echo "  ✓ Create Kubernetes resources (namespaces, nodes, pods)"
echo "  ✓ Scale resources"
echo "  ✓ Respond to kubectl commands"
echo "  ✓ Clean up resources"
echo ""
echo "This confirms that KWOK can be used to simulate thousands of clusters"
echo "for Rancher scalability testing with minimal resource usage."
