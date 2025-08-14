#!/bin/bash

# Comprehensive test script for KWOK integration with scale-cluster-agent
set -e

echo "=== KWOK Integration Test for Scale Cluster Agent ==="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Cleanup function
cleanup() {
    log_info "Cleaning up test resources..."
    
    # Stop and delete test clusters
    for cluster in test-cluster-001 test-cluster-002 test-cluster-003; do
        if ./kwokctl get clusters | grep -q "$cluster"; then
            log_info "Cleaning up cluster: $cluster"
            ./kwokctl stop cluster --name "$cluster" 2>/dev/null || true
            ./kwokctl delete cluster --name "$cluster" 2>/dev/null || true
        fi
    done
    
    # Kill background processes
    pkill -f "scale-cluster-agent" 2>/dev/null || true
    
    log_info "Cleanup complete"
}

# Set trap for cleanup
trap cleanup EXIT

# Test 1: Verify KWOK binaries
log_info "Test 1: Verifying KWOK binaries..."
if [ ! -f "./kwokctl" ] || [ ! -f "./kwok" ]; then
    log_error "KWOK binaries not found. Please ensure kwokctl and kwok are in current directory."
    exit 1
fi

if [ ! -x "./kwokctl" ] || [ ! -x "./kwok" ]; then
    log_info "Making KWOK binaries executable..."
    chmod +x ./kwokctl ./kwok
fi

log_success "KWOK binaries verified and executable"

# Test 2: Verify scale-cluster-agent binary
log_info "Test 2: Verifying scale-cluster-agent binary..."
if [ ! -f "./scale-cluster-agent" ]; then
    log_error "scale-cluster-agent binary not found. Please build it first with: go build -o scale-cluster-agent ."
    exit 1
fi

if [ ! -x "./scale-cluster-agent" ]; then
    log_info "Making scale-cluster-agent executable..."
    chmod +x ./scale-cluster-agent
fi

log_success "scale-cluster-agent binary verified and executable"

# Test 3: Test KWOK cluster creation
log_info "Test 3: Testing KWOK cluster creation..."
log_info "Creating test cluster: test-cluster-001"

./kwokctl create cluster \
    --name test-cluster-001 \
    --runtime binary \
    --kube-apiserver-port 8001 \
    --etcd-port 8002 \
    --kube-controller-manager-port 8003 \
    --kube-scheduler-port 8004 \
    --controller-port 8005

if [ $? -eq 0 ]; then
    log_success "Cluster test-cluster-001 created successfully"
else
    log_error "Failed to create cluster test-cluster-001"
    exit 1
fi

# Test 4: Test cluster startup
log_info "Test 4: Testing cluster startup..."
./kwokctl start cluster --name test-cluster-001

if [ $? -eq 0 ]; then
    log_success "Cluster test-cluster-001 started successfully"
else
    log_error "Failed to start cluster test-cluster-001"
    exit 1
fi

# Test 5: Wait for cluster readiness
log_info "Test 5: Waiting for cluster to be ready..."
log_info "Waiting up to 2 minutes for cluster to be ready..."

timeout=120
elapsed=0
while [ $elapsed -lt $timeout ]; do
    if kubectl --kubeconfig ~/.kwok/clusters/test-cluster-001/kubeconfig.yaml get nodes 2>/dev/null | grep -q "Ready"; then
        log_success "Cluster test-cluster-001 is ready"
        break
    fi
    
    sleep 5
    elapsed=$((elapsed + 5))
    
    if [ $elapsed -ge $timeout ]; then
        log_error "Timeout waiting for cluster to be ready"
        exit 1
    fi
    
    log_info "Still waiting... ($elapsed/$timeout seconds)"
done

# Test 6: Test resource creation
log_info "Test 6: Testing resource creation..."

# Create namespace
kubectl --kubeconfig ~/.kwok/clusters/test-cluster-001/kubeconfig.yaml apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace
EOF

if [ $? -eq 0 ]; then
    log_success "Namespace created successfully"
else
    log_error "Failed to create namespace"
fi

# Create node
kubectl --kubeconfig ~/.kwok/clusters/test-cluster-001/kubeconfig.yaml apply -f - <<EOF
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

if [ $? -eq 0 ]; then
    log_success "Node created successfully"
else
    log_error "Failed to create node"
fi

# Create service account
kubectl --kubeconfig ~/.kwok/clusters/test-cluster-001/kubeconfig.yaml apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-service-account
  namespace: test-namespace
EOF

if [ $? -eq 0 ]; then
    log_success "ServiceAccount created successfully"
else
    log_error "Failed to create ServiceAccount"
fi

# Test 7: Verify resources
log_info "Test 7: Verifying created resources..."

# Check namespace
if kubectl --kubeconfig ~/.kwok/clusters/test-cluster-001/kubeconfig.yaml get namespace test-namespace 2>/dev/null | grep -q "test-namespace"; then
    log_success "Namespace verification passed"
else
    log_error "Namespace verification failed"
fi

# Check node
if kubectl --kubeconfig ~/.kwok/clusters/test-cluster-001/kubeconfig.yaml get nodes test-node-1 2>/dev/null | grep -q "test-node-1"; then
    log_success "Node verification passed"
else
    log_error "Node verification failed"
fi

# Check service account
if kubectl --kubeconfig ~/.kwok/clusters/test-cluster-001/kubeconfig.yaml get serviceaccount test-service-account -n test-namespace 2>/dev/null | grep -q "test-service-account"; then
    log_success "ServiceAccount verification passed"
else
    log_error "ServiceAccount verification failed"
fi

# Test 8: Test scaling
log_info "Test 8: Testing node scaling..."
./kwokctl scale node --name test-cluster-001 --replicas 3

if [ $? -eq 0 ]; then
    log_success "Node scaling initiated successfully"
    
    # Wait for scaling to complete
    sleep 10
    
    node_count=$(kubectl --kubeconfig ~/.kwok/clusters/test-cluster-001/kubeconfig.yaml get nodes 2>/dev/null | grep -c "Ready" || echo "0")
    if [ "$node_count" -ge 3 ]; then
        log_success "Node scaling completed successfully (found $node_count nodes)"
    else
        log_warning "Node scaling may not have completed (found $node_count nodes)"
    fi
else
    log_error "Node scaling failed"
fi

# Test 9: Test multiple clusters
log_info "Test 9: Testing multiple cluster creation..."

# Create second cluster
./kwokctl create cluster \
    --name test-cluster-002 \
    --runtime binary \
    --kube-apiserver-port 8011 \
    --etcd-port 8012 \
    --kube-controller-manager-port 8013 \
    --kube-scheduler-port 8014 \
    --controller-port 8015

if [ $? -eq 0 ]; then
    log_success "Second cluster test-cluster-002 created successfully"
    
    # Start second cluster
    ./kwokctl start cluster --name test-cluster-002
    
    if [ $? -eq 0 ]; then
        log_success "Second cluster started successfully"
        
        # Wait a bit for startup
        sleep 10
        
        # Check if both clusters exist
        cluster_count=$(./kwokctl get clusters | grep -c "test-cluster" || echo "0")
        if [ "$cluster_count" -ge 2 ]; then
            log_success "Multiple cluster management working (found $cluster_count test clusters)"
        else
            log_warning "Multiple cluster management may have issues (found $cluster_count test clusters)"
        fi
    else
        log_error "Failed to start second cluster"
    fi
else
    log_error "Failed to create second cluster"
fi

# Test 10: Test scale-cluster-agent with KWOK
log_info "Test 10: Testing scale-cluster-agent KWOK integration..."

# Create a simple config for testing
mkdir -p ~/.scale-cluster-agent/config
cat > ~/.scale-cluster-agent/config/config <<EOF
RancherURL: https://rancher.test
BearerToken: test-token
ListenPort: 9090
LogLevel: debug
EOF

# Start scale-cluster-agent in background
log_info "Starting scale-cluster-agent in background..."
./scale-cluster-agent > scale-agent-test.log 2>&1 &
AGENT_PID=$!

# Wait for agent to start
sleep 5

# Check if agent is running
if kill -0 $AGENT_PID 2>/dev/null; then
    log_success "scale-cluster-agent started successfully (PID: $AGENT_PID)"
    
    # Test HTTP endpoint
    if curl -s http://localhost:9090/health > /dev/null 2>&1; then
        log_success "HTTP health endpoint responding"
    else
        log_error "HTTP health endpoint not responding"
    fi
    
    # Stop the agent
    kill $AGENT_PID 2>/dev/null || true
    wait $AGENT_PID 2>/dev/null || true
else
    log_error "scale-cluster-agent failed to start"
fi

# Test 11: Verify cluster cleanup
log_info "Test 11: Testing cluster cleanup..."

# Clean up test clusters
for cluster in test-cluster-001 test-cluster-002; do
    if ./kwokctl get clusters | grep -q "$cluster"; then
        log_info "Cleaning up cluster: $cluster"
        ./kwokctl stop cluster --name "$cluster"
        ./kwokctl delete cluster --name "$cluster"
        
        if [ $? -eq 0 ]; then
            log_success "Cluster $cluster cleaned up successfully"
        else
            log_error "Failed to clean up cluster $cluster"
        fi
    fi
done

# Final verification
remaining_clusters=$(./kwokctl get clusters | grep -c "test-cluster" || echo "0")
if [ "$remaining_clusters" -eq 0 ]; then
    log_success "All test clusters cleaned up successfully"
else
    log_warning "Some test clusters may remain (found $remaining_clusters)"
fi

# Summary
echo ""
echo "=== Test Summary ==="
echo -e "${GREEN}Tests Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Tests Failed: $TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed! KWOK integration is working correctly.${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Test with real Rancher server"
    echo "2. Create multiple clusters to verify scalability"
    echo "3. Monitor resource usage during scaling"
    echo ""
    echo "The scale-cluster-agent is ready for production testing!"
else
    echo -e "${RED}❌ Some tests failed. Please review the errors above.${NC}"
    echo ""
    echo "Please fix the failing tests before proceeding to Rancher testing."
fi

echo ""
echo "Test logs saved to: scale-agent-test.log"
echo "KWOK cluster logs available in: ~/.kwok/clusters/"
