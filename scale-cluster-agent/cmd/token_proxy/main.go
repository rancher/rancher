package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run main.go <listen_port> <target_port>")
		fmt.Println("Example: go run main.go 8002 8001")
		os.Exit(1)
	}

	listenPort := os.Args[1]
	targetPort := os.Args[2]

	listenAddr := "127.0.0.1:" + listenPort
	targetAddr := "127.0.0.1:" + targetPort

	// Start the proxy
	proxy := NewTokenCaptureProxy(listenAddr, targetAddr)
	log.Printf("🔍 TOKEN CAPTURE PROXY: Starting proxy on %s -> %s", listenAddr, targetAddr)
	
	if err := proxy.Start(); err != nil {
		log.Fatalf("🔍 TOKEN CAPTURE PROXY: Failed to start proxy: %v", err)
	}
}

// TokenCaptureProxy captures bearer tokens from HTTP requests
type TokenCaptureProxy struct {
	listenAddr string
	targetAddr string
}

// NewTokenCaptureProxy creates a new token capture proxy
func NewTokenCaptureProxy(listenAddr, targetAddr string) *TokenCaptureProxy {
	return &TokenCaptureProxy{
		listenAddr: listenAddr,
		targetAddr: targetAddr,
	}
}

// Start starts the proxy
func (p *TokenCaptureProxy) Start() error {
	listener, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", p.listenAddr, err)
	}
	defer listener.Close()

	log.Printf("🔍 TOKEN CAPTURE PROXY: Listening on %s, forwarding to %s", p.listenAddr, p.targetAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("🔍 TOKEN CAPTURE PROXY: Failed to accept connection: %v", err)
			continue
		}

		go p.handleConnection(conn)
	}
}

// handleConnection handles a single connection
func (p *TokenCaptureProxy) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	log.Printf("🔍 TOKEN CAPTURE PROXY: New connection from %s", clientConn.RemoteAddr())

	// Connect to target
	targetConn, err := net.Dial("tcp", p.targetAddr)
	if err != nil {
		log.Printf("🔍 TOKEN CAPTURE PROXY: Failed to connect to target %s: %v", p.targetAddr, err)
		return
	}
	defer targetConn.Close()

	// Create channels for bidirectional communication
	clientToTarget := make(chan bool)
	targetToClient := make(chan bool)

	// Start goroutines for bidirectional forwarding
	go p.forwardWithTokenCapture(clientConn, targetConn, clientToTarget, "CLIENT->TARGET")
	go p.forwardWithTokenCapture(targetConn, clientConn, targetToClient, "TARGET->CLIENT")

	// Wait for either direction to finish
	select {
	case <-clientToTarget:
	case <-targetToClient:
	}
}

// forwardWithTokenCapture forwards data and captures bearer tokens
func (p *TokenCaptureProxy) forwardWithTokenCapture(from, to net.Conn, done chan bool, direction string) {
	defer func() {
		done <- true
		close(done)
	}()

	// Read the first chunk to check if it's HTTP
	buffer := make([]byte, 8192)
	n, err := from.Read(buffer)
	if err != nil {
		if err != io.EOF {
			log.Printf("🔍 TOKEN CAPTURE PROXY: Error reading from %s: %v", direction, err)
		}
		return
	}

	data := buffer[:n]

	// Log raw data for debugging
	log.Printf("🔍 TOKEN CAPTURE PROXY: Raw data from %s (first 100 bytes): %s", direction, string(data[:min(len(data), 100)]))

	// Check if this looks like an HTTP request
	if strings.HasPrefix(string(data), "GET ") || strings.HasPrefix(string(data), "POST ") || 
	   strings.HasPrefix(string(data), "PUT ") || strings.HasPrefix(string(data), "DELETE ") {
		
		log.Printf("🔍 TOKEN CAPTURE PROXY: HTTP request detected in %s", direction)
		p.captureBearerToken(data)
	} else {
		log.Printf("🔍 TOKEN CAPTURE PROXY: Non-HTTP data detected in %s", direction)
	}

	// Forward the data
	_, err = to.Write(data)
	if err != nil {
		log.Printf("🔍 TOKEN CAPTURE PROXY: Error writing to %s: %v", direction, err)
		return
	}

	// Continue forwarding the rest
	io.Copy(to, from)
}

// captureBearerToken extracts and logs bearer tokens from HTTP requests
func (p *TokenCaptureProxy) captureBearerToken(data []byte) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	
	// Read the first line (request line)
	if scanner.Scan() {
		requestLine := scanner.Text()
		log.Printf("🔍 TOKEN CAPTURE PROXY: HTTP Request: %s", requestLine)
	}

	// Read headers
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break // End of headers
		}

		// Log Authorization header specifically
		if strings.HasPrefix(strings.ToLower(line), "authorization:") {
			log.Printf("🔍 TOKEN CAPTURE PROXY: AUTHORIZATION HEADER: %s", line)
			
			// Extract bearer token
			if strings.Contains(strings.ToLower(line), "bearer ") {
				parts := strings.SplitN(line, " ", 3)
				if len(parts) >= 3 {
					token := parts[2]
					log.Printf("🔍 TOKEN CAPTURE PROXY: BEARER TOKEN: %s", token)
					log.Printf("🔍 TOKEN CAPTURE PROXY: TOKEN LENGTH: %d", len(token))
					log.Printf("🔍 TOKEN CAPTURE PROXY: TOKEN PREFIX: %s...", token[:min(len(token), 20)])
				}
			}
		}

		// Log other important headers
		if strings.HasPrefix(strings.ToLower(line), "user-agent:") ||
		   strings.HasPrefix(strings.ToLower(line), "content-type:") ||
		   strings.HasPrefix(strings.ToLower(line), "content-length:") {
			log.Printf("🔍 TOKEN CAPTURE PROXY: Header: %s", line)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
