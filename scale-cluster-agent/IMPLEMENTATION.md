# Scale Cluster Agent Implementation

This document provides technical details about the implementation of the Scale Cluster Agent.

## Architecture Overview

The Scale Cluster Agent is a standalone Go application that simulates Kubernetes cluster agents for Rancher server scalability testing. It consists of several key components:

### Core Components

1. **Main Application (`main.go`)**
   - Entry point and application lifecycle management
   - Configuration loading and validation
   - Signal handling and graceful shutdown

2. **WebSocket Client**
   - Maintains persistent connection to Rancher server
   - Uses `github.com/rancher/remotedialer` for WebSocket communication
   - Handles reconnection logic

3. **HTTP Server**
   - Provides REST API for cluster management
   - Uses `github.com/gorilla/mux` for routing
   - Supports cluster CRUD operations

4. **Cluster Manager**
   - Manages virtual cluster instances
   - Template-based cluster generation
   - Cluster data serialization and reporting

5. **Configuration Manager**
   - YAML-based configuration loading
   - Template file management
   - Environment variable support

## Technical Implementation Details

### WebSocket Communication

The agent establishes WebSocket connections to the Rancher server using the same protocol as real cluster agents:

```go
// WebSocket URL format
wsURL := fmt.Sprintf("wss://%s/v3/connect", serverHost)

// Headers for authentication
headers := http.Header{
    "X-API-Tunnel-Token": {bearerToken},
}

// Connection establishment
remotedialer.ClientConnect(ctx, wsURL, headers, nil, allowFunc, onConnect)
```

### Cluster Data Structure

The agent uses structured data types to represent Kubernetes resources:

```go
type ClusterInfo struct {
    Name        string            `json:"name"`
    Nodes       []NodeInfo        `json:"nodes"`
    Pods        []PodInfo         `json:"pods"`
    Services    []ServiceInfo     `json:"services"`
    Secrets     []SecretInfo      `json:"secrets"`
    ConfigMaps  []ConfigMapInfo   `json:"configmaps"`
    Deployments []DeploymentInfo  `json:"deployments"`
}
```

### Template System

The agent uses a template-based approach for generating cluster configurations:

1. **Template Loading**: Loads cluster template from `~/.scale-cluster-agent/config/cluster.yaml`
2. **Placeholder Replacement**: Replaces `{{cluster-name}}` placeholders with actual cluster names
3. **Deep Copying**: Creates independent cluster instances from templates

### Configuration Management

Configuration is loaded from multiple sources with precedence:

1. **File-based**: `~/.scale-cluster-agent/config`
2. **Environment Variables**: `SCALE_AGENT_*` prefixed variables
3. **Defaults**: Built-in default values

### REST API Endpoints

The agent provides the following HTTP endpoints:

- `GET /health` - Health check and status information
- `POST /clusters` - Create a new virtual cluster
- `GET /clusters` - List all managed clusters
- `DELETE /clusters/{name}` - Delete a specific cluster

## Data Flow

### Cluster Creation Flow

1. **HTTP Request**: Client sends POST request to `/clusters`
2. **Validation**: Agent validates cluster name and checks for duplicates
3. **Template Processing**: Creates cluster instance from template
4. **Rancher Registration**: Registers cluster with Rancher server (placeholder)
5. **Storage**: Stores cluster in memory
6. **Response**: Returns success response with cluster ID

### Data Reporting Flow

1. **WebSocket Connection**: Maintains persistent connection to Rancher
2. **Periodic Reporting**: Every 30 seconds, reports cluster information
3. **Data Serialization**: Converts cluster data to Rancher-compatible format
4. **Transmission**: Sends data through WebSocket connection

## Security Considerations

### Authentication

- Uses bearer token authentication for Rancher API access
- Tokens should be stored securely in configuration files
- Supports token rotation and expiration handling

### Network Security

- Uses HTTPS/WSS for all communications
- Supports TLS certificate validation
- Configurable connection timeouts and retry logic

### Access Control

- HTTP endpoints should be protected in production
- Consider implementing API key authentication for REST endpoints
- Monitor for unauthorized cluster creation

## Performance Optimizations

### Memory Management

- Template-based generation reduces memory usage
- Efficient data structures for cluster representation
- Garbage collection friendly design

### Network Efficiency

- Persistent WebSocket connections reduce connection overhead
- Configurable reporting intervals
- Connection pooling and reuse

### Scalability

- Supports thousands of virtual clusters
- Efficient cluster lookup and management
- Minimal CPU and memory overhead per cluster

## Error Handling

### Connection Failures

- Automatic reconnection with exponential backoff
- Graceful degradation when Rancher server is unavailable
- Detailed logging for troubleshooting

### Data Validation

- Input validation for all API endpoints
- Template validation and error reporting
- Configuration validation on startup

### Recovery Mechanisms

- Graceful shutdown handling
- State persistence (future enhancement)
- Health check monitoring

## Monitoring and Observability

### Logging

- Structured logging with different levels
- Configurable log output format
- Performance metrics logging

### Health Checks

- Built-in health check endpoint
- Connection status monitoring
- Cluster count and status reporting

### Metrics

- Cluster creation/deletion rates
- WebSocket connection status
- API response times
- Memory and CPU usage

## Future Enhancements

### Planned Features

1. **State Persistence**: Save cluster state to disk for recovery
2. **Advanced Templates**: Support for more complex cluster configurations
3. **Metrics Export**: Prometheus metrics endpoint
4. **Cluster Scaling**: Dynamic cluster scaling based on load
5. **Multi-Rancher Support**: Connect to multiple Rancher instances

### Performance Improvements

1. **Connection Pooling**: Optimize WebSocket connection management
2. **Data Compression**: Compress cluster data for transmission
3. **Caching**: Cache frequently accessed cluster data
4. **Parallel Processing**: Concurrent cluster operations

### Security Enhancements

1. **API Authentication**: Implement API key authentication
2. **Encryption**: Encrypt sensitive configuration data
3. **Audit Logging**: Comprehensive audit trail
4. **RBAC Integration**: Role-based access control

## Testing Strategy

### Unit Testing

- Individual component testing
- Mock WebSocket connections
- Configuration validation testing

### Integration Testing

- End-to-end cluster creation flow
- WebSocket communication testing
- API endpoint testing

### Performance Testing

- Load testing with multiple clusters
- Memory usage profiling
- Network bandwidth testing

### Security Testing

- Authentication validation
- Input validation testing
- Access control verification

## Deployment Considerations

### System Requirements

- Go 1.23 or later
- Minimum 512MB RAM
- Network access to Rancher server
- Disk space for logs and configuration

### Production Deployment

- Use systemd or similar for service management
- Implement log rotation
- Monitor resource usage
- Set up alerting for failures

### Container Deployment

- Docker image available
- Kubernetes deployment manifests
- Helm chart for easy deployment

## Troubleshooting Guide

### Common Issues

1. **Connection Failures**
   - Check Rancher server accessibility
   - Verify bearer token validity
   - Review network configuration

2. **Cluster Creation Failures**
   - Validate cluster names
   - Check template configuration
   - Review Rancher API permissions

3. **Performance Issues**
   - Monitor memory usage
   - Check network bandwidth
   - Review logging levels

### Debug Mode

Enable debug logging for detailed troubleshooting:

```yaml
LogLevel: debug
```

### Health Check

Use the health endpoint to verify agent status:

```bash
curl http://localhost:9090/health
```

## Conclusion

The Scale Cluster Agent provides a robust foundation for Rancher server scalability testing. Its modular design allows for easy extension and customization, while its performance characteristics enable testing with thousands of virtual clusters.

The implementation follows Go best practices and provides comprehensive error handling, monitoring, and security features. Future enhancements will continue to improve performance, security, and usability. 