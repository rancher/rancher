# Scale Cluster Agent

A standalone program that simulates Kubernetes cluster agents for Rancher server scalability testing. This agent connects to a Rancher server via WebSocket and sends fake cluster information to simulate thousands of managed clusters.

## Overview

The Scale Cluster Agent is designed to help test Rancher server's scalability by creating virtual clusters that appear as real Kubernetes clusters to the Rancher management interface. It maintains WebSocket connections to the Rancher server and periodically reports cluster information including nodes, pods, services, secrets, and other Kubernetes resources.

## Features

- **Standalone Operation**: Runs as a standalone program without requiring a Kubernetes cluster
- **WebSocket Communication**: Connects to Rancher server using the same WebSocket protocol as real cluster agents
- **REST API**: Provides HTTP endpoints for cluster management operations
- **Template-based Clusters**: Uses YAML templates to generate cluster configurations
- **Scalable Testing**: Can simulate thousands of clusters for performance testing
- **Configurable**: Supports configuration files for Rancher server connection and cluster templates

## Architecture

The agent consists of several key components:

1. **WebSocket Client**: Maintains persistent connection to Rancher server
2. **HTTP Server**: Provides REST API for cluster management
3. **Cluster Manager**: Manages virtual cluster instances and their data
4. **Template Engine**: Generates cluster configurations from templates
5. **Configuration Manager**: Handles configuration loading and validation

## Installation

### Prerequisites

- Go 1.23 or later
- Access to a Rancher server instance
- Bearer token for Rancher API access

### Building

```bash
# Clone the repository
git clone <repository-url>
cd rancher/scale-cluster-agent

# Build the binary
go build -o bin/scale-cluster-agent main.go
```

### Configuration

1. Create the configuration directory:
```bash
mkdir -p ~/.scale-cluster-agent/config
```

2. Create the main configuration file `~/.scale-cluster-agent/config`:
```yaml
RancherURL: https://your-rancher-server.com/
BearerToken: token-xxxxx:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
ListenPort: 9090
LogLevel: info
```

3. (Optional) Create a custom cluster template `~/.scale-cluster-agent/config/cluster.yaml`:
```yaml
name: "{{cluster-name}}"
nodes:
  - name: "{{cluster-name}}-node1"
    status: "Ready"
    roles: ["control-plane", "etcd", "master"]
    # ... more configuration
```

## Usage

### Starting the Agent

```bash
# Run the agent
./bin/scale-cluster-agent
```

The agent will:
1. Load configuration from `~/.scale-cluster-agent/config`
2. Establish WebSocket connection to Rancher server
3. Start HTTP server on the configured port
4. Begin reporting cluster information

### REST API Endpoints

#### Health Check
```bash
GET /health
```
Returns agent status and cluster count.

#### Create Cluster
```bash
POST /clusters
Content-Type: application/json

{
  "name": "test-cluster-001"
}
```

#### List Clusters
```bash
GET /clusters
```
Returns list of all managed clusters.

#### Delete Cluster
```bash
DELETE /clusters/{name}
```

### Example Usage

```bash
# Create a new cluster
curl -X POST http://localhost:9090/clusters \
  -H "Content-Type: application/json" \
  -d '{"name": "test-cluster-001"}'

# List all clusters
curl http://localhost:9090/clusters

# Check agent health
curl http://localhost:9090/health

# Delete a cluster
curl -X DELETE http://localhost:9090/clusters/test-cluster-001
```

## Cluster Template

The cluster template defines the structure and default values for simulated clusters. The template uses placeholders that are replaced with actual values when creating clusters:

- `{{cluster-name}}`: Replaced with the actual cluster name

### Template Structure

```yaml
name: "{{cluster-name}}"
nodes:
  - name: "{{cluster-name}}-node1"
    status: "Ready"
    roles: ["control-plane", "etcd", "master"]
    # ... node configuration

pods:
  - name: "coredns-6799fbcd5-lgj8v"
    namespace: "kube-system"
    # ... pod configuration

services:
  - name: "kubernetes"
    namespace: "default"
    # ... service configuration

# ... other resource types
```

## Configuration Options

### Main Configuration (`~/.scale-cluster-agent/config`)

| Option | Description | Default |
|--------|-------------|---------|
| `RancherURL` | Rancher server URL | Required |
| `BearerToken` | Rancher API bearer token | Required |
| `ListenPort` | HTTP server port | 9090 |
| `LogLevel` | Logging level (debug, info, warn, error) | info |

### Environment Variables

- `SCALE_AGENT_CONFIG_DIR`: Override configuration directory path
- `SCALE_AGENT_LOG_LEVEL`: Override log level

## Development

### Project Structure

```
scale-cluster-agent/
├── main.go              # Main application entry point
├── go.mod               # Go module definition
├── go.sum               # Go module checksums
├── cluster.yaml         # Sample cluster template
└── README.md           # This file
```

### Building for Development

```bash
# Install dependencies
go mod tidy

# Run with debug logging
LOG_LEVEL=debug go run main.go

# Build with specific version
go build -ldflags "-X main.version=1.0.0" -o bin/scale-cluster-agent main.go
```

### Testing

```bash
# Run tests
go test ./...

# Run with coverage
go test -cover ./...
```

## Integration with Rancher

The Scale Cluster Agent integrates with Rancher by:

1. **WebSocket Connection**: Establishes persistent WebSocket connection to Rancher server
2. **Authentication**: Uses bearer token for API authentication
3. **Cluster Registration**: Registers virtual clusters with Rancher
4. **Data Reporting**: Periodically sends cluster resource information
5. **Lifecycle Management**: Handles cluster creation and deletion

### Rancher Server Requirements

- Rancher v2.6+ (tested with v2.11.3)
- API access enabled
- Valid bearer token with cluster management permissions

## Troubleshooting

### Common Issues

1. **Connection Failed**
   - Verify Rancher server URL is accessible
   - Check bearer token is valid and not expired
   - Ensure network connectivity

2. **Cluster Creation Fails**
   - Verify cluster name is unique
   - Check Rancher API permissions
   - Review server logs for errors

3. **WebSocket Disconnection**
   - Agent automatically reconnects every 5 seconds
   - Check Rancher server WebSocket endpoint
   - Verify firewall settings

### Logging

The agent uses structured logging with different levels:

- `debug`: Detailed debugging information
- `info`: General operational information
- `warn`: Warning messages
- `error`: Error conditions

### Monitoring

Monitor the agent using:

- Health check endpoint: `GET /health`
- Application logs
- Rancher server cluster list
- System resource usage

## Security Considerations

- Store bearer tokens securely
- Use HTTPS for Rancher server connections
- Restrict access to agent HTTP endpoints
- Regularly rotate API tokens
- Monitor for unauthorized cluster creation

## Performance

The agent is designed for scalability testing:

- Supports thousands of virtual clusters
- Efficient memory usage with template-based generation
- Configurable reporting intervals
- Minimal CPU and network overhead

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the Apache License 2.0.

## Support

For issues and questions:

1. Check the troubleshooting section
2. Review Rancher documentation
3. Open an issue in the repository
4. Contact the development team 