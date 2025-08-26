package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Println("Usage: go run main.go <listen_port> <target_port> <cluster_name>")
		fmt.Println("Example: go run main.go 8002 8001 rancher-c-7blgr-8001")
		os.Exit(1)
	}

	listenPort := os.Args[1]
	targetPort := os.Args[2]
	clusterName := os.Args[3]

	listenAddr := "127.0.0.1:" + listenPort
	targetAddr := "127.0.0.1:" + targetPort

	// Start the TLS proxy
	proxy := NewTLSInterceptionProxy(listenAddr, targetAddr, clusterName)
	log.Printf("🔐 TLS INTERCEPTION PROXY: Starting proxy on %s -> %s for cluster %s", listenAddr, targetAddr, clusterName)

	if err := proxy.Start(); err != nil {
		log.Fatalf("🔐 TLS INTERCEPTION PROXY: Failed to start proxy: %v", err)
	}
}

// TLSInterceptionProxy intercepts TLS traffic to capture bearer tokens
type TLSInterceptionProxy struct {
	listenAddr  string
	targetAddr  string
	clusterName string
	certFile    string
	keyFile     string
	caCertFile  string
}

// NewTLSInterceptionProxy creates a new TLS interception proxy
func NewTLSInterceptionProxy(listenAddr, targetAddr, clusterName string) *TLSInterceptionProxy {
	homeDir, _ := os.UserHomeDir()
	certFile := fmt.Sprintf("%s/.kwok/clusters/%s/pki/admin.crt", homeDir, clusterName)
	keyFile := fmt.Sprintf("%s/.kwok/clusters/%s/pki/admin.key", homeDir, clusterName)
	caCertFile := fmt.Sprintf("%s/.kwok/clusters/%s/pki/ca.crt", homeDir, clusterName)

	return &TLSInterceptionProxy{
		listenAddr:  listenAddr,
		targetAddr:  targetAddr,
		clusterName: clusterName,
		certFile:    certFile,
		keyFile:     keyFile,
		caCertFile:  caCertFile,
	}
}

// Start starts the TLS proxy
func (p *TLSInterceptionProxy) Start() error {
	// Load TLS certificate and key
	cert, err := tls.LoadX509KeyPair(p.certFile, p.keyFile)
	if err != nil {
		return fmt.Errorf("failed to load TLS certificate: %v", err)
	}

	// Load CA certificate for client verification
	caCert, err := os.ReadFile(p.caCertFile)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("failed to append CA certificate")
	}

	// Configure TLS - make it more permissive
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.NoClientCert, // Don't require client certs
	}

	// Create listener
	listener, err := tls.Listen("tcp", p.listenAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", p.listenAddr, err)
	}
	defer listener.Close()

	log.Printf("🔐 TLS INTERCEPTION PROXY: Listening on %s with TLS", p.listenAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("🔐 TLS INTERCEPTION PROXY: Failed to accept connection: %v", err)
			continue
		}

		go p.handleTLSConnection(conn)
	}
}

// handleTLSConnection handles a single TLS connection
func (p *TLSInterceptionProxy) handleTLSConnection(clientConn net.Conn) {
	defer clientConn.Close()

	log.Printf("🔐 TLS INTERCEPTION PROXY: New TLS connection from %s", clientConn.RemoteAddr())

	// Connect to target (Kubernetes API server)
	targetConn, err := net.Dial("tcp", p.targetAddr)
	if err != nil {
		log.Printf("🔐 TLS INTERCEPTION PROXY: Failed to connect to target %s: %v", p.targetAddr, err)
		return
	}
	defer targetConn.Close()

	// Create channels for bidirectional communication
	clientToTarget := make(chan bool)
	targetToClient := make(chan bool)

	// Start goroutines for bidirectional forwarding with logging
	go p.forwardWithLogging(clientConn, targetConn, clientToTarget, "CLIENT->TARGET")
	go p.forwardWithLogging(targetConn, clientConn, targetToClient, "TARGET->CLIENT")

	// Wait for either direction to finish
	select {
	case <-clientToTarget:
	case <-targetToClient:
	}
}

// forwardWithLogging forwards data and logs HTTP requests
func (p *TLSInterceptionProxy) forwardWithLogging(from, to net.Conn, done chan bool, direction string) {
	defer func() {
		done <- true
		close(done)
	}()

	// Read the first chunk to check if it's HTTP
	buffer := make([]byte, 8192)
	n, err := from.Read(buffer)
	if err != nil {
		if err != io.EOF {
			log.Printf("🔐 TLS INTERCEPTION PROXY: Error reading from %s: %v", direction, err)
		}
		return
	}

	data := buffer[:n]

	// Check if this looks like an HTTP request
	if strings.HasPrefix(string(data), "GET ") || strings.HasPrefix(string(data), "POST ") ||
		strings.HasPrefix(string(data), "PUT ") || strings.HasPrefix(string(data), "DELETE ") {

		log.Printf("🔐 TLS INTERCEPTION PROXY: HTTP request detected in %s", direction)
		p.logHTTPRequest(data)
	} else {
		log.Printf("🔐 TLS INTERCEPTION PROXY: Non-HTTP data in %s (first 100 bytes): %s", direction, string(data[:min(len(data), 100)]))
	}

	// Forward the data
	_, err = to.Write(data)
	if err != nil {
		log.Printf("🔐 TLS INTERCEPTION PROXY: Error writing to %s: %v", direction, err)
		return
	}

	// Continue forwarding the rest
	io.Copy(to, from)
}

// logHTTPRequest logs HTTP request details including bearer tokens
func (p *TLSInterceptionProxy) logHTTPRequest(data []byte) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	// Read the first line (request line)
	if scanner.Scan() {
		requestLine := scanner.Text()
		log.Printf("🔐 TLS INTERCEPTION PROXY: ===== HTTP REQUEST =====")
		log.Printf("🔐 TLS INTERCEPTION PROXY: Request Line: %s", requestLine)
	}

	// Read headers
	log.Printf("🔐 TLS INTERCEPTION PROXY: ===== REQUEST HEADERS =====")
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break // End of headers
		}

		// Log Authorization header specifically
		if strings.HasPrefix(strings.ToLower(line), "authorization:") {
			log.Printf("🔐 TLS INTERCEPTION PROXY: 🔑 AUTHORIZATION HEADER: %s", line)

			// Extract bearer token
			if strings.Contains(strings.ToLower(line), "bearer ") {
				parts := strings.SplitN(line, " ", 3)
				if len(parts) >= 3 {
					token := parts[2]
					log.Printf("🔐 TLS INTERCEPTION PROXY: 🔑 BEARER TOKEN: %s", token)
					log.Printf("🔐 TLS INTERCEPTION PROXY: 🔑 TOKEN LENGTH: %d", len(token))
					log.Printf("🔐 TLS INTERCEPTION PROXY: 🔑 TOKEN PREFIX: %s...", token[:min(len(token), 20)])
				}
			}
		}

		// Log other important headers
		if strings.HasPrefix(strings.ToLower(line), "user-agent:") ||
			strings.HasPrefix(strings.ToLower(line), "content-type:") ||
			strings.HasPrefix(strings.ToLower(line), "content-length:") ||
			strings.HasPrefix(strings.ToLower(line), "host:") {
			log.Printf("🔐 TLS INTERCEPTION PROXY: Header: %s", line)
		}
	}

	// Log request body if present
	bodyStart := strings.Index(string(data), "\r\n\r\n")
	if bodyStart != -1 && bodyStart+4 < len(data) {
		body := data[bodyStart+4:]
		if len(body) > 0 {
			log.Printf("🔐 TLS INTERCEPTION PROXY: ===== REQUEST BODY =====")
			log.Printf("🔐 TLS INTERCEPTION PROXY: Body Length: %d", len(body))
			log.Printf("🔐 TLS INTERCEPTION PROXY: Body Preview: %s", string(body[:min(len(body), 200)]))
		}
	}

	log.Printf("🔐 TLS INTERCEPTION PROXY: ===== END REQUEST LOGGING =====")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
