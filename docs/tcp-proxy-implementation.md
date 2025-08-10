# TCP Proxy Implementation in Scale Cluster Agent

## Overview

The Scale Cluster Agent implements TCP proxying to simulate real Kubernetes cluster agents. This document explains how the TCP proxy works and how it follows the real agent implementation pattern.

## Key Finding: How remotedialer Actually Works

After analyzing the real agent code and the `remotedialer` package source code, we discovered that:

### 1. allowFunc is Just a Permission Check
The `allowFunc` function in the real agent only decides whether to allow connections - it does NOT implement the actual TCP proxying.

```go
// Real agent allowFunc
func(proto, address string) bool {
    switch proto {
    case "tcp":
        return true  // ← Just allows ALL TCP connections
    }
    return false
}
```

### 2. remotedialer Handles TCP Proxying Internally
When `allowFunc` returns `true`, the `remotedialer` package automatically:
- Calls `clientDial()` function
- Dials to the target address (e.g., `10.43.0.1:443`)
- Calls `pipe()` function to set up bidirectional `io.Copy()`

### 3. io.Copy() Implementation in remotedialer
The actual `io.Copy()` implementation is in the `remotedialer` package:

```go
// From remotedialer/client_dialer.go
func pipe(client *connection, server net.Conn) {
    wg := sync.WaitGroup{}
    wg.Add(1)

    go func() {
        defer wg.Done()
        _, err := io.Copy(server, client)  // ← Copy from WebSocket tunnel to TCP
        closePipe(err)
    }()

    _, err := io.Copy(client, server)  // ← Copy from TCP to WebSocket tunnel
    err = closePipe(err)
    wg.Wait()
}
```

## Implementation in Scale Cluster Agent

### 1. allowFunc Implementation
Our agent follows the real agent pattern:

```go
allowFunc := func(proto, address string) bool {
    switch proto {
    case "tcp":
        if strings.HasPrefix(address, "localhost:") {
            // Allow connections to our mock server
            if port == mockServer.Port {
                logrus.Infof("DEBUG: Allowing connection to mock server %s for cluster %s", address, clusterName)
                return true  // ← Just return true, let remotedialer handle the rest
            }
        }
        return false
    }
    return false
}
```

### 2. Mock Server Setup
We start a mock Kubernetes API server that listens on a localhost port:

```go
func (m *MockServerManager) StartMockServer(clusterName string, config *ClusterInfo) (*MockServer, error) {
    // Allocate port and start HTTP server
    port := m.allocatePort()
    listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
    
    // Set up HTTP handlers for Kubernetes API endpoints
    mux := http.NewServeMux()
    mux.HandleFunc("/healthz", ms.handleHealthz)
    mux.HandleFunc("/api/v1/nodes", ms.handleNodes)
    // ... more handlers
    
    server := &http.Server{
        Addr:    fmt.Sprintf("localhost:%d", port),
        Handler: mux,
    }
    
    return &MockServer{
        ClusterName: clusterName,
        Port:        port,
        Server:      server,
        Config:      config,
        Listener:    listener,
    }, nil
}
```

### 3. Data Flow
When Rancher requests a connection to our mock server:

1. **Rancher** sends connection request through WebSocket tunnel to `localhost:4567`
2. **allowFunc** receives the request and validates it's our mock server port
3. **allowFunc returns `true`** - allowing the connection
4. **remotedialer** automatically calls `clientDial()` and dials to `localhost:4567`
5. **remotedialer** calls `pipe()` and sets up bidirectional `io.Copy()`
6. **Our mock server** receives HTTP requests and responds with mock Kubernetes API data
7. **Rancher** receives responses and should mark cluster as "Active"

## Key Differences from Previous Implementation

### Previous (Incorrect) Approach:
- Tried to manually implement `io.Copy()` in `allowFunc`
- Called `a.proxyTCPConnection()` from `allowFunc`
- Fought against `remotedialer` instead of using it

### Current (Correct) Approach:
- `allowFunc` just returns `true` to allow connections
- Let `remotedialer` handle all TCP proxying automatically
- Focus on providing realistic mock API responses

## Benefits of This Approach

1. **Follows Real Agent Pattern**: Matches exactly how the real agent works
2. **Automatic Error Handling**: `remotedialer` handles connection cleanup and errors
3. **Bidirectional Proxying**: Automatic two-way data flow
4. **No Manual Implementation**: No need to implement `io.Copy()` manually
5. **Reliable**: Uses the same proven code path as real agents

## Testing the Implementation

To verify the TCP proxy is working:

1. **Check Logs**: Look for "Allowing connection to mock server" messages
2. **Test Mock Server**: Direct curl requests to `localhost:4567/healthz`
3. **Monitor Connections**: Use `netstat -an | grep 4567` to see active connections
4. **Rancher Status**: Check if cluster transitions to "Active" state

## Conclusion

The TCP proxy implementation now correctly follows the real agent pattern by leveraging `remotedialer`'s built-in TCP proxying capabilities. This approach is more reliable, maintainable, and matches the behavior of real Kubernetes cluster agents. 