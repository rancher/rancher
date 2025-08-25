package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

// TCPProxy creates a proxy that logs HTTP requests before forwarding them
type TCPProxy struct {
	listenAddr string
	targetAddr string
}

// NewTCPProxy creates a new TCP proxy
func NewTCPProxy(listenAddr, targetAddr string) *TCPProxy {
	return &TCPProxy{
		listenAddr: listenAddr,
		targetAddr: targetAddr,
	}
}

// Start starts the proxy
func (p *TCPProxy) Start() error {
	listener, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", p.listenAddr, err)
	}
	defer listener.Close()

	log.Printf("🔍 TCP PROXY: Listening on %s, forwarding to %s", p.listenAddr, p.targetAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("🔍 TCP PROXY: Failed to accept connection: %v", err)
			continue
		}

		go p.handleConnection(conn)
	}
}

// handleConnection handles a single connection
func (p *TCPProxy) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	log.Printf("🔍 TCP PROXY: New connection from %s", clientConn.RemoteAddr())

	// Connect to target
	targetConn, err := net.Dial("tcp", p.targetAddr)
	if err != nil {
		log.Printf("🔍 TCP PROXY: Failed to connect to target %s: %v", p.targetAddr, err)
		return
	}
	defer targetConn.Close()

	// Create channels for bidirectional communication
	clientToTarget := make(chan []byte, 100)
	targetToClient := make(chan []byte, 100)

	// Start goroutines for bidirectional forwarding
	go p.forwardWithLogging(clientConn, targetConn, clientToTarget, "CLIENT->TARGET")
	go p.forwardWithLogging(targetConn, clientConn, targetToClient, "TARGET->CLIENT")

	// Wait for either direction to finish
	select {
	case <-time.After(30 * time.Second):
		log.Printf("🔍 TCP PROXY: Connection timeout")
	case <-clientToTarget:
	case <-targetToClient:
	}
}

// forwardWithLogging forwards data with HTTP request logging
func (p *TCPProxy) forwardWithLogging(from, to net.Conn, done chan []byte, direction string) {
	defer close(done)

	// Read the first chunk to check if it's HTTP
	buffer := make([]byte, 4096)
	n, err := from.Read(buffer)
	if err != nil {
		if err != io.EOF {
			log.Printf("🔍 TCP PROXY: Error reading from %s: %v", direction, err)
		}
		return
	}

	data := buffer[:n]

	// Check if this looks like an HTTP request
	if strings.HasPrefix(string(data), "GET ") || strings.HasPrefix(string(data), "POST ") || 
	   strings.HasPrefix(string(data), "PUT ") || strings.HasPrefix(string(data), "DELETE ") {
		
		log.Printf("🔍 TCP PROXY: HTTP request detected in %s", direction)
		p.logHTTPRequest(data)
	}

	// Forward the data
	_, err = to.Write(data)
	if err != nil {
		log.Printf("🔍 TCP PROXY: Error writing to %s: %v", direction, err)
		return
	}

	// Continue forwarding the rest
	io.Copy(to, from)
}

// logHTTPRequest logs HTTP request details including bearer tokens
func (p *TCPProxy) logHTTPRequest(data []byte) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	
	// Read the first line (request line)
	if scanner.Scan() {
		requestLine := scanner.Text()
		log.Printf("🔍 TCP PROXY: HTTP Request: %s", requestLine)
	}

	// Read headers
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break // End of headers
		}

		// Log Authorization header specifically
		if strings.HasPrefix(strings.ToLower(line), "authorization:") {
			log.Printf("🔍 TCP PROXY: AUTHORIZATION HEADER: %s", line)
			
			// Extract bearer token
			if strings.Contains(strings.ToLower(line), "bearer ") {
				parts := strings.SplitN(line, " ", 3)
				if len(parts) >= 3 {
					token := parts[2]
					log.Printf("🔍 TCP PROXY: BEARER TOKEN: %s", token)
					log.Printf("🔍 TCP PROXY: TOKEN LENGTH: %d", len(token))
				}
			}
		}

		// Log other important headers
		if strings.HasPrefix(strings.ToLower(line), "user-agent:") ||
		   strings.HasPrefix(strings.ToLower(line), "content-type:") ||
		   strings.HasPrefix(strings.ToLower(line), "content-length:") {
			log.Printf("🔍 TCP PROXY: Header: %s", line)
		}
	}
}
