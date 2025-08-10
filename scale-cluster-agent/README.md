# Scale Cluster Agent

A simulated Kubernetes cluster agent that can register multiple clusters with a Rancher server. This agent creates mock Kubernetes API servers for each cluster and registers them with Rancher, bringing them to 'Active' state.

## Features

- **HTTPS/TLS Support**: All mock Kubernetes API servers run with HTTPS/TLS using self-signed certificates
- **Multiple Cluster Management**: Simulate multiple Kubernetes clusters simultaneously
- **Rancher Integration**: Automatically register clusters with Rancher server
- **Mock Kubernetes API**: Full Kubernetes API simulation with health checks, nodes, pods, services, etc.
- **Configurable**: YAML-based configuration for easy customization
- **REST API**: HTTP API for managing clusters and monitoring status

## Architecture

The Scale Cluster Agent consists of several components:

1. **Main Agent**: Manages the overall lifecycle and provides REST API
2. **Mock Server Manager**: Creates and manages mock Kubernetes API servers
3. **Mock Kubernetes API Servers**: Simulate real Kubernetes clusters with HTTPS/TLS
4. **Rancher Integration**: Registers clusters with Rancher server

## Configuration

Create a `config.yaml` file with the following structure:

```yaml
# Rancher Server Configuration
rancher_server:
  url: "https://your-rancher-server.com"
  token: "your-rancher-token"
  ca_cert: ""  # Optional: CA certificate for Rancher server
  insecure: false  # Set to true to skip TLS verification

# Agent Configuration
agent:
  id: "scale-cluster-agent-001"
  name: "Scale Cluster Agent"
  listen_port: 9090

# Cluster Configuration
clusters:
  - name: "cluster-1"
    display_name: "Production Cluster 1"
    description: "Production cluster for web applications"
    k8s_version: "v1.28.0"
    node_count: 5
    region: "us-west-2"
    labels:
      environment: "production"
      team: "platform"

# Mock Server Configuration
mock_server:
  enabled: true
  port_range_start: 30000
  port_range_end: 31000
  tls_enabled: true

# Connection Settings
connection:
  timeout: "30s"
  keep_alive: true
  max_retries: 3
  retry_delay: "5s"

# Logging Configuration
logging:
  format: "info"  # debug, info, warn, error
  output: "stdout"
```

## Usage

### 1. Build the Agent

```bash
cd scale-cluster-agent
go build -o scale-cluster-agent main.go
```

### 2. Configure the Agent

Copy the example configuration and customize it:

```bash
cp config.example config.yaml
# Edit config.yaml with your Rancher server details
```

### 3. Run the Agent

```bash
nohup ./bin/scale-cluster-agent > scale-agent-debug-$(date +%Y%m%d-%H%M%S).log 2>&1 &
```

The agent will:
- Start the main HTTP server on the configured port
- Create mock Kubernetes API servers for each configured cluster
- Each mock server runs on HTTPS with self-signed certificates
- Register clusters with Rancher server

### 4. Monitor and Manage Clusters

The agent provides a REST API for cluster management:

```bash
# Check agent health
curl http://localhost:9090/health

# List all clusters
curl http://localhost:9090/clusters

# Get cluster status
curl http://localhost:9090/clusters/cluster-1/status

# Register a cluster with Rancher
curl -X POST http://localhost:9090/clusters/cluster-1/register

# Create a new cluster
curl -X POST http://localhost:9090/clusters \
  -H "Content-Type: application/json" \
  -d '{"name": "new-cluster"}'
```

## Mock Kubernetes API Servers

Each cluster gets its own mock Kubernetes API server that:

- Runs on HTTPS with self-signed certificates
- Responds to standard Kubernetes API endpoints
- Simulates cluster resources (nodes, pods, services, etc.)
- Maintains persistent connections like real Kubernetes clusters
- Provides health checks and readiness probes

### Supported Endpoints

- `/healthz` - Health check endpoint
- `/readyz` - Readiness check endpoint
- `/api/v1/nodes` - Node information
- `/api/v1/pods` - Pod information
- `/api/v1/services` - Service information
- `/api/v1/namespaces` - Namespace information
- `/api/v1/secrets` - Secret information
- `/api/v1/configmaps` - ConfigMap information

## Rancher Integration

The agent automatically:

1. Creates clusters in Rancher via the v3 API
2. Configures cluster metadata and labels
3. Sets up proper cluster registration
4. Brings clusters to 'Active' state

### Cluster Registration Process

1. **Create Cluster**: Agent creates cluster in Rancher via API
2. **Mock Server**: Starts HTTPS mock Kubernetes API server
3. **Rancher Connection**: Rancher connects to mock server for cluster management
4. **Active State**: Cluster reaches 'Active' state in Rancher

## Security Features

- **HTTPS/TLS**: All mock servers use HTTPS with TLS 1.2+
- **Self-Signed Certificates**: Automatically generated per cluster
- **Certificate Validation**: Proper certificate chain and validation
- **Secure Cipher Suites**: Modern, secure cipher suite selection

## Development

### Project Structure

```
scale-cluster-agent/
├── main.go              # Main application code
├── config.yaml          # Configuration file
├── config.example       # Example configuration
├── cluster.yaml         # Cluster template
├── README.md            # This file
└── IMPLEMENTATION.md    # Implementation details
```

### Key Components

- **MockServerManager**: Manages mock Kubernetes API servers
- **MockServer**: Individual mock Kubernetes API server
- **ScaleAgent**: Main agent that orchestrates everything
- **Configuration**: YAML-based configuration system

### Building and Testing

```bash
# Build
go build -o scale-cluster-agent main.go

# Run tests (if available)
go test ./...

# Run with debug logging
LOG_LEVEL=debug ./scale-cluster-agent
```

## Troubleshooting

### Common Issues

1. **Port Conflicts**: Ensure port ranges don't conflict with existing services
2. **Certificate Issues**: Mock servers generate self-signed certificates automatically
3. **Rancher Connection**: Verify Rancher server URL and token are correct
4. **HTTPS Issues**: Ensure TLS is properly configured in Rancher

### Logs

The agent provides detailed logging for debugging:

- Mock server creation and management
- Rancher API interactions
- Cluster registration status
- HTTPS/TLS connection details

### Health Checks

Monitor the agent's health:

```bash
curl http://localhost:9090/health
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is part of the Rancher ecosystem and follows the same licensing terms.

## Support

For issues and questions:
- Check the logs for detailed error messages
- Verify configuration settings
- Ensure Rancher server is accessible
- Check network connectivity and firewall rules 
