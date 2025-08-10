
# Scale Cluster Agent Project

## Overview
The Scale Cluster Agent is a Go application designed to simulate multiple Rancher cluster agents connecting to a Rancher server for scalability testing. The agent creates simulated clusters and establishes WebSocket connections to Rancher, mimicking the behavior of real cluster agents.

## Key Implementation Approach

### Cluster Simulation Strategy
**Important**: This program simulates clusters - we do NOT have real Kubernetes clusters. We are faking cluster responses to Rancher.

When Rancher sends API requests through the WebSocket tunnel asking for cluster information (nodes, pods, services, etc.), we need to:

1. **Intercept Rancher's API requests** - When Rancher requests connections to `10.43.0.1:443` (the Kubernetes API server), we intercept these requests
2. **Generate fake responses** - Instead of connecting to a real API server, we formulate responses based on our static configuration
3. **Use static cluster data** - We respond with our pre-configured cluster information including:
   - 2 simulated nodes
   - Multiple simulated pods
   - Services, secrets, configmaps, deployments
   - Cluster status information

### Response Simulation
When Rancher asks for:
- `/healthz` → Return healthy status
- `/readyz` → Return ready status  
- `/api/v1/nodes` → Return our 2 configured nodes
- `/api/v1/pods` → Return our configured pods
- `/api/v1/services` → Return our configured services
- `/api/v1/namespaces/kube-system` → Return simulated kube-system namespace
- `/api/v1/secrets` → Return simulated secrets

### WebSocket Tunnel Handling
- Rancher sends messages through WebSocket tunnel requesting connections to `10.43.0.1:443`
- We intercept these requests in the `allowFunc`
- Instead of allowing real connections, we simulate API responses
- This makes Rancher think the cluster is healthy and active

### Goal
Make Rancher believe we have thousands of real, active clusters by responding with realistic but static cluster data to all API requests.

## Architecture
