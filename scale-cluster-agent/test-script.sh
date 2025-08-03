#!/bin/bash

# Scale Cluster Agent Test Script
# This script demonstrates the basic usage of the scale cluster agent API

set -e

# Configuration
AGENT_URL="http://localhost:9090"
CLUSTER_NAME="test-cluster-$(date +%s)"

echo "=== Scale Cluster Agent Test Script ==="
echo "Agent URL: $AGENT_URL"
echo "Test cluster name: $CLUSTER_NAME"
echo

# Function to check if agent is running
check_agent() {
    echo "Checking if agent is running..."
    if curl -s "$AGENT_URL/health" > /dev/null; then
        echo "✓ Agent is running"
        return 0
    else
        echo "✗ Agent is not running or not accessible"
        return 1
    fi
}

# Function to get agent health
get_health() {
    echo "Getting agent health..."
    curl -s "$AGENT_URL/health" | jq .
    echo
}

# Function to list clusters
list_clusters() {
    echo "Listing clusters..."
    curl -s "$AGENT_URL/clusters" | jq .
    echo
}

# Function to create a cluster
create_cluster() {
    echo "Creating cluster: $CLUSTER_NAME"
    curl -s -X POST "$AGENT_URL/clusters" \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"$CLUSTER_NAME\"}" | jq .
    echo
}

# Function to delete a cluster
delete_cluster() {
    echo "Deleting cluster: $CLUSTER_NAME"
    curl -s -X DELETE "$AGENT_URL/clusters/$CLUSTER_NAME" | jq .
    echo
}

# Main test sequence
main() {
    echo "Starting test sequence..."
    echo

    # Check if agent is running
    if ! check_agent; then
        echo "Please start the scale cluster agent first:"
        echo "  ./bin/scale-cluster-agent"
        exit 1
    fi

    # Get initial health
    get_health

    # List initial clusters
    list_clusters

    # Create a test cluster
    create_cluster

    # Wait a moment for processing
    sleep 2

    # List clusters again
    list_clusters

    # Get health again
    get_health

    # Wait a moment
    sleep 2

    # Delete the test cluster
    delete_cluster

    # List clusters one more time
    list_clusters

    echo "=== Test completed successfully ==="
}

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo "Warning: jq is not installed. JSON output will not be formatted."
    echo "Install jq for better output formatting."
    echo
fi

# Run the test
main 