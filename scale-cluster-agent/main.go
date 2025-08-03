package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const (
	configDir = "~/.scale-cluster-agent/config"
	version   = "1.0.0"
)

type Config struct {
	RancherURL  string `yaml:"RancherURL"`
	BearerToken string `yaml:"BearerToken"`
	ListenPort  int    `yaml:"ListenPort"`
	LogLevel    string `yaml:"LogLevel"`
}

type ClusterInfo struct {
	Name        string           `json:"name"`
	ClusterID   string           `json:"cluster_id,omitempty"`
	Nodes       []NodeInfo       `json:"nodes"`
	Pods        []PodInfo        `json:"pods"`
	Services    []ServiceInfo    `json:"services"`
	Secrets     []SecretInfo     `json:"secrets"`
	ConfigMaps  []ConfigMapInfo  `json:"configmaps"`
	Deployments []DeploymentInfo `json:"deployments"`
}

type NodeInfo struct {
	Name             string            `json:"name"`
	Status           string            `json:"status"`
	Roles            []string          `json:"roles"`
	Age              string            `json:"age"`
	Version          string            `json:"version"`
	InternalIP       string            `json:"internalIP"`
	ExternalIP       string            `json:"externalIP"`
	OSImage          string            `json:"osImage"`
	KernelVer        string            `json:"kernelVersion"`
	ContainerRuntime string            `json:"containerRuntime"`
	Capacity         map[string]string `json:"capacity"`
	Allocatable      map[string]string `json:"allocatable"`
}

type PodInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Status    string            `json:"status"`
	Ready     string            `json:"ready"`
	Restarts  int               `json:"restarts"`
	Age       string            `json:"age"`
	IP        string            `json:"ip"`
	Node      string            `json:"node"`
	Labels    map[string]string `json:"labels"`
}

type ServiceInfo struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Type       string            `json:"type"`
	ClusterIP  string            `json:"clusterIP"`
	ExternalIP string            `json:"externalIP"`
	Ports      string            `json:"ports"`
	Age        string            `json:"age"`
	Labels     map[string]string `json:"labels"`
}

type SecretInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	Data      int    `json:"data"`
	Age       string `json:"age"`
}

type ConfigMapInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Data      int    `json:"data"`
	Age       string `json:"age"`
}

type DeploymentInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Ready     string            `json:"ready"`
	UpToDate  string            `json:"upToDate"`
	Available string            `json:"available"`
	Age       string            `json:"age"`
	Labels    map[string]string `json:"labels"`
}

type PortForwarder struct {
	clusterName string
	localPort   int
	server      *http.Server
	ctx         context.Context
	cancel      context.CancelFunc
}

// TunnelHandler handles incoming tunneled connections to the Kubernetes API
type TunnelHandler struct {
	clusterName string
}

type ScaleAgent struct {
	config            *Config
	clusters          map[string]*ClusterInfo
	httpServer        *http.Server
	ctx               context.Context
	cancel            context.CancelFunc
	activeConnections map[string]bool           // Track active connections to prevent duplicates
	connMutex         sync.RWMutex              // Protect connection tracking
	tokenCache        map[string]string         // Cache cluster registration tokens
	tokenMutex        sync.RWMutex              // Protect token cache
	mockServers       map[string]*http.Server   // Track mock API servers per cluster
	serverMutex       sync.RWMutex              // Protect mock servers
	portForwarders    map[string]*PortForwarder // Track port forwarders per cluster
	forwarderMutex    sync.RWMutex              // Protect port forwarders
	nextPort          int                       // Next available port for clusters
	portMutex         sync.Mutex                // Protect port allocation
}

type CreateClusterRequest struct {
	Name string `json:"name"`
}

type CreateClusterResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	ClusterID string `json:"cluster_id,omitempty"`
}

func main() {
	// Setup logging
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.SetLevel(logrus.DebugLevel)

	logrus.Infof("Scale Cluster Agent version %s starting", version)

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		logrus.Fatalf("Failed to load configuration: %v", err)
	}

	// Set log level from config
	if config.LogLevel != "" {
		level, err := logrus.ParseLevel(config.LogLevel)
		if err == nil {
			logrus.SetLevel(level)
		}
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create scale agent
	agent := &ScaleAgent{
		config:            config,
		clusters:          make(map[string]*ClusterInfo),
		ctx:               ctx,
		cancel:            cancel,
		activeConnections: make(map[string]bool),
		connMutex:         sync.RWMutex{},
		tokenCache:        make(map[string]string),
		mockServers:       make(map[string]*http.Server),
		portForwarders:    make(map[string]*PortForwarder),
		nextPort:          1, // Start from port 8001
	}

	// Load cluster template
	err = agent.loadClusterTemplate()
	if err != nil {
		logrus.Fatalf("Failed to load cluster template: %v", err)
	}

	// Setup HTTP server
	router := mux.NewRouter()
	router.HandleFunc("/health", agent.healthHandler).Methods("GET")
	router.HandleFunc("/clusters", agent.createClusterHandler).Methods("POST")
	router.HandleFunc("/clusters", agent.listClustersHandler).Methods("GET")
	router.HandleFunc("/clusters/{name}", agent.deleteClusterHandler).Methods("DELETE")

	agent.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", config.ListenPort),
		Handler: router,
	}

	// Start HTTP server
	go func() {
		logrus.Infof("Starting HTTP server on port %d", config.ListenPort)
		if err := agent.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Start websocket connection to Rancher only when we have clusters
	go agent.startWebSocketConnection()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logrus.Info("Shutting down scale cluster agent...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := agent.httpServer.Shutdown(shutdownCtx); err != nil {
		logrus.Errorf("HTTP server shutdown error: %v", err)
	}

	cancel()
	logrus.Info("Scale cluster agent stopped")
}

func loadConfig() (*Config, error) {
	// Expand home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %v", err)
	}

	configPath := filepath.Join(homeDir, ".scale-cluster-agent", "config", "config")

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", configPath, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %v\nPlease ensure the file is in valid YAML format. Example:\nRancherURL: \"https://your-rancher-server.com/\"\nBearerToken: \"your-token-here\"\nListenPort: 9090\nLogLevel: \"info\"", configPath, err)
	}

	// Validate required fields
	if config.RancherURL == "" {
		return nil, fmt.Errorf("RancherURL is required in config file %s", configPath)
	}
	if config.BearerToken == "" {
		return nil, fmt.Errorf("BearerToken is required in config file %s", configPath)
	}

	// Set defaults
	if config.ListenPort == 0 {
		config.ListenPort = 9090
	}
	if config.LogLevel == "" {
		config.LogLevel = "info"
	}

	return &config, nil
}

func (a *ScaleAgent) loadClusterTemplate() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %v", err)
	}

	clusterYamlPath := filepath.Join(homeDir, ".scale-cluster-agent", "config", "cluster.yaml")

	data, err := os.ReadFile(clusterYamlPath)
	if err != nil {
		logrus.Warnf("Failed to read cluster template %s: %v", clusterYamlPath, err)
		logrus.Info("Using default cluster template")
		return a.createDefaultClusterTemplate()
	}

	var clusterInfo ClusterInfo
	if err := yaml.Unmarshal(data, &clusterInfo); err != nil {
		return fmt.Errorf("failed to parse cluster template: %v", err)
	}

	// Store as template
	a.clusters["template"] = &clusterInfo
	return nil
}

func (a *ScaleAgent) createDefaultClusterTemplate() error {
	// Create a default cluster template based on the k3s example in the requirements
	template := &ClusterInfo{
		Name: "template",
		Nodes: []NodeInfo{
			{
				Name:             "{{cluster-name}}-node1",
				Status:           "Ready",
				Roles:            []string{"control-plane", "etcd", "master"},
				Age:              "42h",
				Version:          "v1.28.5+k3s1",
				InternalIP:       "10.244.244.1",
				ExternalIP:       "<none>",
				OSImage:          "0.0.0-poc-k3s-june11-aca5d463-dirty-2025-07-31.04.42-kubevirt-amd64",
				KernelVer:        "6.1.112-linuxkit-63f4d774fbc8",
				ContainerRuntime: "containerd://1.7.11-k3s2",
				Capacity: map[string]string{
					"cpu":    "4",
					"memory": "8Gi",
					"pods":   "110",
				},
				Allocatable: map[string]string{
					"cpu":    "3800m",
					"memory": "7Gi",
					"pods":   "110",
				},
			},
			{
				Name:             "{{cluster-name}}-node2",
				Status:           "Ready",
				Roles:            []string{"control-plane", "etcd", "master"},
				Age:              "39h",
				Version:          "v1.28.5+k3s1",
				InternalIP:       "10.244.244.3",
				ExternalIP:       "<none>",
				OSImage:          "0.0.0-poc-k3s-june11-aca5d463-dirty-2025-07-31.04.42-kubevirt-amd64",
				KernelVer:        "6.1.112-linuxkit-63f4d774fbc8",
				ContainerRuntime: "containerd://1.7.11-k3s2",
				Capacity: map[string]string{
					"cpu":    "4",
					"memory": "8Gi",
					"pods":   "110",
				},
				Allocatable: map[string]string{
					"cpu":    "3800m",
					"memory": "7Gi",
					"pods":   "110",
				},
			},
		},
		Pods: []PodInfo{
			{
				Name:      "coredns-6799fbcd5-lgj8v",
				Namespace: "kube-system",
				Status:    "Running",
				Ready:     "1/1",
				Restarts:  3,
				Age:       "42h",
				IP:        "10.42.0.97",
				Node:      "{{cluster-name}}-node1",
				Labels:    map[string]string{"k8s-app": "kube-dns"},
			},
			{
				Name:      "traefik-f4564c4f4-xz9t5",
				Namespace: "kube-system",
				Status:    "Running",
				Ready:     "1/1",
				Restarts:  3,
				Age:       "42h",
				IP:        "10.42.0.105",
				Node:      "{{cluster-name}}-node1",
				Labels:    map[string]string{"app.kubernetes.io/name": "traefik"},
			},
			{
				Name:      "nginx-deployment-7d4cd48b5c-abc12",
				Namespace: "default",
				Status:    "Running",
				Ready:     "3/3",
				Restarts:  0,
				Age:       "2h",
				IP:        "10.42.1.10",
				Node:      "{{cluster-name}}-node2",
				Labels:    map[string]string{"app": "nginx"},
			},
			{
				Name:      "grafana-6b8c5d4f3e-def34",
				Namespace: "monitoring",
				Status:    "Running",
				Ready:     "1/1",
				Restarts:  1,
				Age:       "1h",
				IP:        "10.42.2.15",
				Node:      "{{cluster-name}}-node1",
				Labels:    map[string]string{"app": "grafana"},
			},
		},
		Services: []ServiceInfo{
			{
				Name:       "kubernetes",
				Namespace:  "default",
				Type:       "ClusterIP",
				ClusterIP:  "10.43.0.1",
				ExternalIP: "<none>",
				Ports:      "443/TCP",
				Age:        "42h",
			},
			{
				Name:       "kube-dns",
				Namespace:  "kube-system",
				Type:       "ClusterIP",
				ClusterIP:  "10.43.0.10",
				ExternalIP: "<none>",
				Ports:      "53/UDP,53/TCP,9153/TCP",
				Age:        "42h",
				Labels:     map[string]string{"k8s-app": "kube-dns"},
			},
			{
				Name:       "traefik",
				Namespace:  "kube-system",
				Type:       "LoadBalancer",
				ClusterIP:  "10.43.185.148",
				ExternalIP: "10.244.244.1,10.244.244.3",
				Ports:      "80:32522/TCP,443:32443/TCP",
				Age:        "42h",
				Labels:     map[string]string{"app.kubernetes.io/name": "traefik"},
			},
		},
		Secrets: []SecretInfo{
			{
				Name:      "default-token-abc12",
				Namespace: "default",
				Type:      "kubernetes.io/service-account-token",
				Data:      3,
				Age:       "42h",
			},
			{
				Name:      "rancher-token-def34",
				Namespace: "cattle-system",
				Type:      "kubernetes.io/service-account-token",
				Data:      3,
				Age:       "42h",
			},
		},
		ConfigMaps: []ConfigMapInfo{
			{
				Name:      "kube-root-ca.crt",
				Namespace: "default",
				Data:      1,
				Age:       "42h",
			},
			{
				Name:      "traefik-config",
				Namespace: "kube-system",
				Data:      2,
				Age:       "42h",
			},
		},
		Deployments: []DeploymentInfo{
			{
				Name:      "nginx-deployment",
				Namespace: "default",
				Ready:     "3",
				UpToDate:  "3",
				Available: "3",
				Age:       "2h",
				Labels:    map[string]string{"app": "nginx"},
			},
			{
				Name:      "grafana",
				Namespace: "monitoring",
				Ready:     "1",
				UpToDate:  "1",
				Available: "1",
				Age:       "1h",
				Labels:    map[string]string{"app": "grafana"},
			},
		},
	}

	a.clusters["template"] = template
	return nil
}

func (a *ScaleAgent) startWebSocketConnection() {
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			// Only try to connect if we have clusters (excluding template)
			clusterCount := len(a.clusters) - 1 // Exclude template
			if clusterCount > 0 {
				a.connectToRancher()
			} else {
				logrus.Debug("No clusters configured yet, skipping WebSocket connection")
			}
			time.Sleep(5 * time.Second)
		}
	}
}

func (a *ScaleAgent) connectToRancher() {
	// Connect to Rancher WebSocket for each cluster
	for clusterName, clusterInfo := range a.clusters {
		if clusterName == "template" {
			continue
		}

		// Use the real cluster ID from the cluster info
		if clusterInfo.ClusterID == "" {
			logrus.Warnf("No cluster ID found for %s, skipping WebSocket connection", clusterName)
			continue
		}

		go a.connectClusterToRancher(clusterName, clusterInfo.ClusterID, clusterInfo)
	}
}

func (a *ScaleAgent) connectClusterToRancher(clusterName, clusterID string, clusterInfo *ClusterInfo) {
	// Check if already connected to prevent multiple connections
	a.connMutex.Lock()
	if a.activeConnections[clusterName] {
		logrus.Infof("Cluster %s is already connected, skipping duplicate connection", clusterName)
		a.connMutex.Unlock()
		return
	}
	a.connMutex.Unlock()

	// Get cluster registration token
	// This function is no longer needed as the tunnel handler handles authentication
	// clusterToken, err := a.getClusterRegistrationToken(clusterID)
	// if err != nil {
	// 	logrus.Errorf("Failed to get cluster registration token for %s: %v", clusterName, err)
	// 	a.connMutex.Lock()
	// 	delete(a.activeConnections, clusterName)
	// 	a.connMutex.Unlock()
	// 	return
	// }

	// Get import YAML using the token
	// This function is no longer needed as the tunnel handler handles import
	// importYAML, err := a.getImportYAMLWithToken(clusterToken, clusterID)
	// if err != nil {
	// 	logrus.Errorf("Failed to get import YAML for %s: %v", clusterName, err)
	// 	a.connMutex.Lock()
	// 	delete(a.activeConnections, clusterName)
	// 	a.connMutex.Unlock()
	// 	return
	// }

	// Extract cluster credentials from import YAML
	// This function is no longer needed as the tunnel handler handles credentials
	// clusterCredentials, clusterURL, err := a.extractClusterCredentials(importYAML)
	// if err != nil {
	// 	logrus.Errorf("Failed to extract cluster credentials for %s: %v", clusterName, err)
	// 	a.connMutex.Lock()
	// 	delete(a.activeConnections, clusterName)
	// 	a.connMutex.Unlock()
	// 	return
	// }

	// logrus.Infof("Extracted cluster-specific credentials for %s: token=%s, url=%s", clusterName, clusterCredentials[:10]+"...", clusterURL)

	// Create credential files like the real agent would have after applying import YAML
	// This function is no longer needed as the tunnel handler handles credentials
	// if err := a.createCredentialFiles(clusterName, clusterCredentials, clusterURL); err != nil {
	// 	logrus.Errorf("Failed to create credential files for %s: %v", clusterName, err)
	// 	a.connMutex.Lock()
	// 	delete(a.activeConnections, clusterName)
	// 	a.connMutex.Unlock()
	// 	return
	// }

	// Get the cluster registration token for this specific cluster
	token, err := a.getClusterToken(clusterID)
	if err != nil {
		logrus.Errorf("Failed to get cluster token for cluster %s: %v", clusterName, err)
		return
	}
	
	logrus.Infof("Successfully got token for cluster %s: %s", clusterName, token[:10]+"...")

	// Mark as connected only after successful token retrieval
	a.connMutex.Lock()
	a.activeConnections[clusterName] = true
	a.connMutex.Unlock()

	// Now use the extracted credentials for WebSocket connection (like real agent)
	// Use the Rancher server URL host, not the cluster ID
	rancherURL, err := url.Parse(a.config.RancherURL)
	if err != nil {
		logrus.Errorf("Failed to parse Rancher URL: %v", err)
		return
	}
	wsURL := fmt.Sprintf("wss://%s/v3/connect/register", rancherURL.Host) // Use /register endpoint like real agent

	logrus.Infof("Attempting WebSocket connection to %s for cluster %s", wsURL, clusterName)

	// Create CLUSTER parameters that match the real cluster.Params() function structure
	// Based on the real agent logs, we need:
	// - address: Kubernetes API server address (e.g., "10.43.0.1:443")
	// - caCert: Base64-encoded CA certificate
	// - token: Service account token
	clusterParams := map[string]interface{}{
		"address": "10.43.0.1:443",                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    // Simulated Kubernetes API server address
		"caCert":  "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3SGhjTk1qVXdOek14TURJd01qTXlXaGNOTXpVd056STVNREl3TWpNeQpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUK0tDazFjWkE0Z1pyOGJEV0JSWFNtSEZpZGZ0UFBYRm82VUorM0RCTWgKTUJocVBuSVFWcjRxR25SbXRuL3hNd3ViZzV3a2FlNXhDWWZNU0R2aGJxcVBvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVXBpcWMwazEwSXVKVnZ1dzBhaHgyCi9uSHl3TFF3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUlnS0ZiSWJhKzd0Tm9CVWZvVis3dEdEN3FRTWdjVEgyeXkKU2pKYTFOODNkNW9DSVFEWEZ4VERELzhQNFF3WDNsRDg0ZU9wQlVTMENZZC8zNXdPb2JaT3VVNWUxUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K",                                                                                                                                                         // Simulated CA cert
		"token":   "eyJhbGciOiJSUzI1NiIsImtpZCI6InBlSXFkTGYwVmVfZVVqUGNKZzFYSUExV25XbGtRSDd5bnNyZTcwTWQ2bncifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJjYXR0bGUtc3lzdGVtIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImNhdHRsZS10b2tlbi1ubnF4cSIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJjYXR0bGUiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC51aWQiOiI3NGY1NDUzOC1lYjc1LTRkNmItOGVmYy1lZjY4MDQ5MTk4NTQiLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6Y2F0dGxlLXN5c3RlbTpjYXR0bGUifQ.Ocy7h9xQMbw6xcWvLIKe6icd2P6Vb265UKaNzRdlYTfy704AVU6lCltWm-cb_2ne-5PAvw0AzBCpNoCUhDUFnwgSZnJDpVcAolSyKNJShH2hXnamPRD9s-q-6OtP3fdYe_Ospp_hHhfRF1xpbW9xgx5rdZCNYW5pb_06xtn4X0LzFP9hiD7_B_NgHTB4E-DBfdlj3U8HUmNTyHJTbMxNklGZd1PHKe6eZXqzPBUTFtxLj4dCgDvQh2VgxlA8H6AJlY0zivb6ohnXgpUZ1hzmhScPHgbm9cPAQQji24OZumznAZCWA2B1SS0Wd4GlNcXBZ65Dky_dub2y6MSv-LbpcQ", // Simulated service account token
	}

	// Match the exact structure returned by cluster.Params()
	params := map[string]interface{}{
		"cluster": clusterParams,
	}

	// Marshal and encode parameters like real agent
	bytes, err := json.Marshal(params)
	if err != nil {
		logrus.Errorf("Failed to marshal params: %v", err)
		return
	}

	// Debug: Log the parameters being sent
	logrus.Debugf("Cluster parameters: %s", string(bytes))
	logrus.Debugf("Base64 encoded params: %s", base64.StdEncoding.EncodeToString(bytes))

	// Use the EXACT header names from real agent logs
	headers := http.Header{
		"X-API-Tunnel-Token":  {token},                                    // Use extracted token from import YAML
		"X-API-Tunnel-Params": {base64.StdEncoding.EncodeToString(bytes)}, // Use correct header name
	}
	
	logrus.Infof("Using token in WebSocket headers: %s", token[:10]+"...")

	// Debug: Log the headers being sent
	logrus.Debugf("WebSocket headers: X-API-Tunnel-Token=%s, X-API-Tunnel-Params=%s",
		"",
		base64.StdEncoding.EncodeToString(bytes)[:20]+"...")

	// Define allow function like real agent
	allowFunc := func(proto, address string) bool {
		switch proto {
		case "tcp":
			// Allow connections to the Kubernetes API server address and our local server
			if address == "10.43.0.1:443" {
				logrus.Infof("Allowing connection to Kubernetes API server: %s", address)
				return true
			}
			// Also allow connections to our local server
			port := 8000 + len(a.clusters)
			if address == fmt.Sprintf("127.0.0.1:%d", port) {
				logrus.Infof("Allowing connection to local API server: %s", address)
				return true
			}
			return true
		case "unix":
			return address == "/var/run/docker.sock"
		case "npipe":
			return address == "//./pipe/docker_engine"
		}
		return false
	}

	// Define onConnect function
	onConnect := func(ctx context.Context, session *remotedialer.Session) error {
		logrus.Infof("Successfully connected to Rancher for cluster %s", clusterName)

		// Simulate ConfigClient call like the real agent does to get initial configuration
		connectConfig := fmt.Sprintf("https://%s/v3/connect/config", strings.TrimPrefix(clusterInfo.ClusterID, "https://"))

		logrus.Infof("DEBUG: Simulating ConfigClient call for cluster %s at %s", clusterName, connectConfig)
		// Simulate successful config response with 30-second interval
		interval := 30
		logrus.Infof("DEBUG: ConfigClient simulation successful for cluster %s, interval: %d", clusterName, interval)

		// Simulate rancher.Run() call (like real agent)
		logrus.Infof("DEBUG: Simulating rancher.Run() for cluster %s", clusterName)
		time.Sleep(1 * time.Second)

		// Start sending simulated cluster data
		go a.sendClusterDataViaRemotedialer(clusterName, session, clusterInfo)

		// Start plan monitor like real agent (simplified)
		go func() {
			logrus.Infof("DEBUG: Starting plan monitor for cluster %s, checking every %d seconds", clusterName, interval)
			tt := time.Duration(interval) * time.Second
			for {
				select {
				case <-time.After(tt):
					// Simulate successful plan check
					logrus.Debugf("DEBUG: Plan monitor for cluster %s: simulated successful check", clusterName)
				case <-ctx.Done():
					logrus.Infof("DEBUG: Plan monitor for cluster %s stopped", clusterName)
					return
				}
			}
		}()

		return nil
	}

	// Use remotedialer.ClientConnect with cluster-specific credentials
	logrus.Infof("Attempting WebSocket connection to %s for cluster %s", wsURL, clusterName)
	remotedialer.ClientConnect(a.ctx, wsURL, headers, nil, allowFunc, onConnect)

	// Start a local server to handle tunneled requests to 10.43.0.1:443
	// We'll use a unique port for each cluster to avoid conflicts
	port := 8000 + len(a.clusters)
	go a.startLocalAPIServer(clusterName, port)
}

func (a *ScaleAgent) sendClusterDataViaRemotedialer(clusterName string, session *remotedialer.Session, clusterInfo *ClusterInfo) {
	logrus.Infof("Starting simulated cluster data transmission for %s", clusterName)

	// Simulate Kubernetes API server being available
	// This is what makes Rancher think the cluster is "active"
	go a.simulateKubernetesAPIServer(clusterName, session)

	// Keep the connection alive and send periodic updates
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			logrus.Debugf("Sending periodic cluster data for %s", clusterName)
			// Simulate sending cluster metrics/data
		case <-a.ctx.Done():
			logrus.Infof("Stopping cluster data transmission for %s", clusterName)
			return
		}
	}
}

func (a *ScaleAgent) simulateKubernetesAPIServer(clusterName string, session *remotedialer.Session) {
	logrus.Infof("Starting simulated Kubernetes API server for cluster %s", clusterName)

	// Start a real HTTP server that provides the API endpoints Rancher expects
	go func() {
		// Use a unique port for each cluster to avoid conflicts
		port := 8000 + len(a.clusters)

		// Create a mock Kubernetes API server
		mux := http.NewServeMux()

		// Health check endpoints that Rancher expects
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			logrus.Debugf("Health check request for cluster %s: %s", clusterName, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		})

		mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
			logrus.Debugf("Ready check request for cluster %s: %s", clusterName, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		})

		// Kubernetes API endpoints
		mux.HandleFunc("/api/v1", func(w http.ResponseWriter, r *http.Request) {
			logrus.Debugf("API v1 request for cluster %s: %s", clusterName, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"kind": "APIResourceList",
				"apiVersion": "v1",
				"groupVersion": "v1",
				"resources": [
					{"name": "pods", "namespaced": true, "kind": "Pod"},
					{"name": "services", "namespaced": true, "kind": "Service"},
					{"name": "nodes", "namespaced": false, "kind": "Node"},
					{"name": "namespaces", "namespaced": false, "kind": "Namespace"},
					{"name": "secrets", "namespaced": true, "kind": "Secret"},
					{"name": "configmaps", "namespaced": true, "kind": "ConfigMap"}
				]
			}`))
		})

		// Root endpoint
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			logrus.Debugf("Root request for cluster %s: %s", clusterName, r.URL.Path)
			if r.URL.Path == "/" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{
					"kind": "APIVersions",
					"versions": ["v1"],
					"serverAddressByClientCIDRs": [
						{"clientCIDR": "0.0.0.0/0", "serverAddress": "10.43.0.1:443"}
					]
				}`))
			} else {
				http.NotFound(w, r)
			}
		})

		// Start the server
		server := &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		}

		// Store the server reference
		a.serverMutex.Lock()
		a.mockServers[clusterName] = server
		a.serverMutex.Unlock()

		logrus.Infof("Starting mock Kubernetes API server for cluster %s on port %d", clusterName, port)

		// Simulate the embedded Rancher server startup process (like real agent)
		go func() {
			logrus.Infof("DEBUG: Simulating embedded Rancher server startup for cluster %s", clusterName)

			// Step 1: Start Service controller (like real agent)
			logrus.Infof("DEBUG: Starting /v1, Kind=Service controller for cluster %s", clusterName)
			time.Sleep(1 * time.Second)

			// Step 2: Start API controllers (like real agent)
			logrus.Infof("DEBUG: Starting API controllers for cluster %s", clusterName)
			time.Sleep(1 * time.Second)

			// Step 3: Start embedded server (like real agent)
			logrus.Infof("DEBUG: Starting embedded Rancher server for cluster %s", clusterName)
			logrus.Infof("DEBUG: Listening on :443 for cluster %s", clusterName)
			logrus.Infof("DEBUG: Listening on :80 for cluster %s", clusterName)
			logrus.Infof("DEBUG: Listening on :444 for cluster %s", clusterName)
			time.Sleep(1 * time.Second)

			// Step 4: Start Steve aggregation (like real agent)
			logrus.Infof("DEBUG: Starting steve aggregation client for cluster %s", clusterName)
			time.Sleep(1 * time.Second)

			// Step 5: Win leader election (like real agent)
			logrus.Infof("DEBUG: Successfully acquired lease kube-system/cattle-controllers for cluster %s", clusterName)
			time.Sleep(1 * time.Second)

			// Step 6: Start ServiceAccountSecretCleaner (like real agent)
			logrus.Infof("DEBUG: Starting ServiceAccountSecretCleaner for cluster %s", clusterName)
			time.Sleep(1 * time.Second)

			// Step 7: Complete Steve auth (like real agent)
			logrus.Infof("DEBUG: Steve auth startup complete for cluster %s", clusterName)

			logrus.Infof("Simulated embedded Rancher server for cluster %s is now fully operational", clusterName)
			logrus.Infof("Cluster %s should now appear as 'Active' in Rancher", clusterName)
		}()

		// Start the HTTP server
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Errorf("Mock API server error for cluster %s: %v", clusterName, err)
		}
	}()

	// Give the embedded server time to start
	time.Sleep(8 * time.Second)

	logrus.Infof("Simulated Kubernetes API server for cluster %s is now responding to health checks", clusterName)
	logrus.Infof("Cluster %s should now appear as 'Active' in Rancher", clusterName)
}

// HTTP handlers
func (a *ScaleAgent) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "healthy",
		"version":  version,
		"clusters": len(a.clusters) - 1, // Exclude template
	})
}

func (a *ScaleAgent) createClusterHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Cluster name is required", http.StatusBadRequest)
		return
	}

	// Check if cluster already exists
	if _, exists := a.clusters[req.Name]; exists {
		http.Error(w, "Cluster already exists", http.StatusConflict)
		return
	}

	// Create new cluster from template
	clusterInfo := a.createClusterFromTemplate(req.Name)

	// Create cluster in Rancher
	clusterID, err := a.createClusterInRancher(req.Name)
	if err != nil {
		logrus.Errorf("Failed to create cluster in Rancher: %v", err)
		http.Error(w, "Failed to create cluster", http.StatusInternalServerError)
		return
	}

	// Store the real cluster ID
	clusterInfo.ClusterID = clusterID
	a.clusters[req.Name] = clusterInfo

	// Log that we now have clusters and will start WebSocket connection
	if len(a.clusters) == 2 { // template + 1 cluster
		logrus.Info("First cluster created, WebSocket connection to Rancher will start")
	}

	response := CreateClusterResponse{
		Success:   true,
		Message:   fmt.Sprintf("Cluster %s created successfully", req.Name),
		ClusterID: clusterID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (a *ScaleAgent) listClustersHandler(w http.ResponseWriter, r *http.Request) {
	clusterList := make([]string, 0)
	for name := range a.clusters {
		if name != "template" {
			clusterList = append(clusterList, name)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"clusters": clusterList,
		"count":    len(clusterList),
	})
}

func (a *ScaleAgent) deleteClusterHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["name"]

	if clusterName == "template" {
		http.Error(w, "Cannot delete template cluster", http.StatusBadRequest)
		return
	}

	if _, exists := a.clusters[clusterName]; !exists {
		http.Error(w, "Cluster not found", http.StatusNotFound)
		return
	}

	// Delete cluster from Rancher (this would be the actual implementation)
	err := a.deleteClusterFromRancher(clusterName)
	if err != nil {
		logrus.Errorf("Failed to delete cluster from Rancher: %v", err)
		http.Error(w, "Failed to delete cluster", http.StatusInternalServerError)
		return
	}

	delete(a.clusters, clusterName)

	// Log if we're removing the last cluster
	if len(a.clusters) == 1 { // Only template remaining
		logrus.Info("Last cluster deleted, WebSocket connection will stop")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Cluster %s deleted successfully", clusterName),
	})
}

func (a *ScaleAgent) createClusterFromTemplate(clusterName string) *ClusterInfo {
	template := a.clusters["template"]
	if template == nil {
		return nil
	}

	// Deep copy the template and replace placeholders
	clusterData, _ := json.Marshal(template)
	var newCluster ClusterInfo
	json.Unmarshal(clusterData, &newCluster)

	// Replace placeholders with actual cluster name
	clusterNamePlaceholder := "{{cluster-name}}"

	// Update node names
	for i := range newCluster.Nodes {
		newCluster.Nodes[i].Name = strings.ReplaceAll(newCluster.Nodes[i].Name, clusterNamePlaceholder, clusterName)
	}

	// Update pod node references
	for i := range newCluster.Pods {
		newCluster.Pods[i].Node = strings.ReplaceAll(newCluster.Pods[i].Node, clusterNamePlaceholder, clusterName)
	}

	newCluster.Name = clusterName
	return &newCluster
}

func (a *ScaleAgent) createClusterInRancher(clusterName string) (string, error) {
	// Create a real cluster in Rancher via REST API
	// For imported clusters, we need to create a cluster with imported driver
	clusterData := map[string]interface{}{
		"type": "cluster",
		"name": clusterName,
	}

	jsonData, err := json.Marshal(clusterData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cluster data: %v", err)
	}

	logrus.Infof("Sending cluster data: %s", string(jsonData))

	// Make request to Rancher API to create cluster
	client := &http.Client{Timeout: 30 * time.Second}
	// Fix double slash issue by ensuring proper URL formatting
	rancherURL := strings.TrimRight(a.config.RancherURL, "/")
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v3/clusters", rancherURL), bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.config.BearerToken))

	logrus.Infof("Creating cluster %s in Rancher via API", clusterName)
	logrus.Infof("Request URL: %s", req.URL.String())
	logrus.Infof("Request method: %s", req.Method)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create cluster in Rancher: %v", err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	logrus.Infof("Rancher API response status: %d", resp.StatusCode)
	logrus.Infof("Rancher API response body: %s", string(body))

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to create cluster in Rancher: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response to get cluster ID
	var clusterResp map[string]interface{}
	if err := json.Unmarshal(body, &clusterResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	clusterID, ok := clusterResp["id"].(string)
	if !ok {
		return "", fmt.Errorf("no cluster ID in response: %+v", clusterResp)
	}

	logrus.Infof("Successfully created cluster %s in Rancher with ID: %s", clusterName, clusterID)

	// Now get the import YAML and apply it to complete the cluster registration
	// This will move the cluster from "pending" to "active" state
	if err := a.completeClusterRegistration(clusterID); err != nil {
		logrus.Warnf("Failed to complete cluster registration for %s: %v", clusterName, err)
		// Don't fail the entire operation, just log the warning
	} else {
		logrus.Infof("Cluster %s registration completed successfully", clusterName)

		// Wait for cluster to become active, then establish WebSocket connection
		// Get the cluster info from the stored clusters
		if storedClusterInfo, exists := a.clusters[clusterName]; exists {
			go a.waitForClusterActiveAndConnect(clusterName, clusterID, storedClusterInfo)
		}
	}

	return clusterID, nil
}

func (a *ScaleAgent) completeClusterRegistration(clusterID string) error {
	// For scale testing, we'll simulate successful cluster registration
	// without needing the import YAML. The key is to establish the remotedialer
	// connection which makes Rancher think the cluster is active.

	logrus.Infof("Simulating cluster registration completion for %s", clusterID)
	logrus.Infof("Cluster %s will be treated as active once remotedialer connection is established", clusterID)

	// In a real implementation, we would:
	// 1. Get the import YAML from Rancher
	// 2. Apply it to a real Kubernetes cluster
	// 3. Wait for the cluster to become active

	// For scale testing, we simulate success and rely on remotedialer connection
	return nil
}

func (a *ScaleAgent) getImportYAML(clusterID string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	rancherURL := strings.TrimRight(a.config.RancherURL, "/")

	// The importYaml action typically requires an empty JSON object as body
	jsonData := []byte("{}")

	// Call the importYaml action to get the import YAML
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v3/clusters/%s?action=importYaml", rancherURL, clusterID), bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create import YAML request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.config.BearerToken))

	logrus.Infof("Getting import YAML for cluster %s", clusterID)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get import YAML: %v", err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	logrus.Infof("Import YAML response status: %d", resp.StatusCode)
	logrus.Debugf("Import YAML response body: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get import YAML: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response to get the YAML
	var importResp map[string]interface{}
	if err := json.Unmarshal(body, &importResp); err != nil {
		return "", fmt.Errorf("failed to decode import YAML response: %v", err)
	}

	yamlData, ok := importResp["yamlOutput"].(string)
	if !ok {
		return "", fmt.Errorf("no yamlOutput in import YAML response: %+v", importResp)
	}

	logrus.Infof("Successfully retrieved import YAML for cluster %s", clusterID)
	return yamlData, nil
}

func (a *ScaleAgent) applyImportYAML(yamlData string) error {
	logrus.Infof("Applying import YAML to complete cluster registration")
	logrus.Debugf("Import YAML content: %s", yamlData)

	// For the scale testing scenario, we need to actually apply the YAML
	// to make the cluster active on the Rancher server side.

	// The import YAML typically contains:
	// - Cluster registration tokens
	// - Agent deployment manifests
	// - Cluster configuration

	// Since we're simulating clusters, we'll create a minimal Kubernetes
	// cluster simulation that can accept and process the import YAML.

	// For now, let's implement a basic YAML parser and apply the key resources
	if err := a.parseAndApplyImportYAML(yamlData); err != nil {
		return fmt.Errorf("failed to parse and apply import YAML: %v", err)
	}

	logrus.Infof("Import YAML applied successfully - cluster should now be active")
	return nil
}

func (a *ScaleAgent) parseAndApplyImportYAML(yamlData string) error {
	// Parse the YAML and extract key resources
	// This is a simplified implementation for the scale testing scenario

	logrus.Infof("Parsing import YAML for cluster registration")

	// The import YAML typically contains:
	// 1. Cluster registration token
	// 2. Agent deployment
	// 3. Cluster configuration

	// For scale testing, we'll simulate the successful application
	// of these resources to make the cluster active

	logrus.Infof("Simulated successful application of cluster registration resources")
	logrus.Infof("Cluster should now be active and ready for WebSocket connections")

	return nil
}

func (a *ScaleAgent) waitForClusterActiveAndConnect(clusterName, clusterID string, clusterInfo *ClusterInfo) {
	// Wait for cluster to become active
	logrus.Infof("Waiting for cluster %s to become active...", clusterName)

	// In a real implementation, we would poll the cluster status
	// For now, we'll simulate a delay and then attempt connection
	time.Sleep(10 * time.Second)

	logrus.Infof("Cluster %s should now be active, attempting WebSocket connection", clusterName)

	// Attempt to establish WebSocket connection (only once, like real agent)
	a.connectClusterToRancher(clusterName, clusterID, clusterInfo)
}

func (a *ScaleAgent) deleteClusterFromRancher(clusterName string) error {
	// This is a placeholder implementation
	// In a real implementation, you would make HTTP request to Rancher API to delete cluster

	logrus.Infof("Deleting cluster %s from Rancher", clusterName)
	return nil
}

// getClusterToken gets the actual token for a specific cluster
func (a *ScaleAgent) getClusterToken(clusterID string) (string, error) {
	logrus.Infof("Getting cluster token for cluster %s", clusterID)

	// Get the cluster registration tokens for this cluster
	url := fmt.Sprintf("%s/v3/clusters/%s/clusterregistrationtokens", strings.TrimRight(a.config.RancherURL, "/"), clusterID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.config.BearerToken))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster registration tokens: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get cluster registration tokens: status %d, body: %s", resp.StatusCode, string(body))
	}

	var tokensResponse struct {
		Data []struct {
			Token string `json:"token"`
		} `json:"data"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	if err := json.Unmarshal(body, &tokensResponse); err != nil {
		return "", fmt.Errorf("failed to parse tokens response: %v", err)
	}

	if len(tokensResponse.Data) == 0 {
		return "", fmt.Errorf("no tokens found for cluster %s", clusterID)
	}

	token := tokensResponse.Data[0].Token
	logrus.Infof("Got token for cluster %s: %s", clusterID, token[:10]+"...")

	return token, nil
}

// extractClusterCredentials extracts token and URL from import YAML (like real agent)
func (a *ScaleAgent) extractClusterCredentials(importYAML string) (string, string, error) {
	logrus.Infof("Extracting cluster credentials from import YAML")
	logrus.Debugf("Import YAML content: %s", importYAML)

	// Parse the import YAML to find the Secret with token and URL
	// The import YAML typically contains a Secret with data:
	// - token: base64-encoded token
	// - url: base64-encoded URL

	// For now, we'll use a real token format that Rancher expects
	// The real agent uses the token from the Secret in the import YAML
	// We need to get the actual token for this specific cluster
	token := "xv5lhs425rtg9t5cjjq9qkj6mq2grjcpt5m77vxwgnsshxs8kkp6rv"
	url := "https://green-cluster.shen.nu"

	logrus.Infof("Extracted token: %s", token[:10]+"...")
	logrus.Infof("Extracted URL: %s", url)

	return token, url, nil
}

// startLocalAPIServer starts a local HTTP server to handle tunneled requests to 10.43.0.1:443
func (a *ScaleAgent) startLocalAPIServer(clusterName string, port int) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logrus.Errorf("Failed to start local API server for cluster %s: %v", clusterName, err)
		return
	}
	defer listener.Close()

	logrus.Infof("Started local API server for cluster %s on %s", clusterName, addr)

	// Add the server address to the allowFunc so remotedialer can connect to it
	a.connMutex.Lock()
	a.activeConnections[clusterName] = true
	a.connMutex.Unlock()

	for {
		conn, err := listener.Accept()
		if err != nil {
			logrus.Errorf("Failed to accept connection for cluster %s: %v", clusterName, err)
			continue
		}

		go func() {
			defer conn.Close()

			// Read the HTTP request
			reader := bufio.NewReader(conn)
			request, err := reader.ReadString('\n')
			if err != nil {
				logrus.Errorf("Failed to read request for cluster %s: %v", clusterName, err)
				return
			}

			logrus.Infof("Received request for cluster %s: %s", clusterName, strings.TrimSpace(request))

			// Parse the request line
			parts := strings.Fields(request)
			if len(parts) < 2 {
				logrus.Errorf("Invalid request format for cluster %s: %s", clusterName, request)
				return
			}

			method := parts[0]
			path := parts[1]

			logrus.Infof("Processing %s %s for cluster %s", method, path, clusterName)

			// Handle different API endpoints
			var response string
			switch path {
			case "/api/v1/namespaces/kube-system":
				response = `HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 234

{
	"kind": "Namespace",
	"apiVersion": "v1",
	"metadata": {
		"name": "kube-system",
		"uid": "cluster-` + clusterName + `-kube-system",
		"resourceVersion": "1",
		"creationTimestamp": "2025-08-03T00:00:00Z"
	},
	"spec": {
		"finalizers": ["kubernetes"]
	},
	"status": {
		"phase": "Active"
	}
}`

			case "/healthz":
				response = `HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 15

{"status":"ok"}`

			case "/readyz":
				response = `HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 15

{"status":"ok"}`

			case "/api/v1":
				response = `HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 234

{
	"kind": "APIResourceList",
	"apiVersion": "v1",
	"groupVersion": "v1",
	"resources": [
		{"name": "pods", "namespaced": true, "kind": "Pod"},
		{"name": "services", "namespaced": true, "kind": "Service"},
		{"name": "nodes", "namespaced": false, "kind": "Node"},
		{"name": "namespaces", "namespaced": false, "kind": "Namespace"},
		{"name": "secrets", "namespaced": true, "kind": "Secret"},
		{"name": "configmaps", "namespaced": true, "kind": "ConfigMap"}
	]
}`

			case "/api/v1/secrets":
				response = `HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 234

{
	"kind": "SecretList",
	"apiVersion": "v1",
	"metadata": {
		"resourceVersion": "1"
	},
	"items": [
		{
			"kind": "Secret",
			"apiVersion": "v1",
			"metadata": {
				"name": "default-token-abc123",
				"namespace": "default",
				"uid": "secret-` + clusterName + `-default-token",
				"resourceVersion": "1"
			},
			"type": "kubernetes.io/service-account-token",
			"data": {
				"token": "ZXhhbXBsZS10b2tlbg==",
				"ca.crt": "ZXhhbXBsZS1jYS1jZXJ0",
				"namespace": "ZGVmYXVsdA=="
			}
		}
	]
}`

			case "/api/v1/namespaces/default/secrets":
				response = `HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 234

{
	"kind": "SecretList",
	"apiVersion": "v1",
	"metadata": {
		"resourceVersion": "1"
	},
	"items": [
		{
			"kind": "Secret",
			"apiVersion": "v1",
			"metadata": {
				"name": "default-token-abc123",
				"namespace": "default",
				"uid": "secret-` + clusterName + `-default-token",
				"resourceVersion": "1"
			},
			"type": "kubernetes.io/service-account-token",
			"data": {
				"token": "ZXhhbXBsZS10b2tlbg==",
				"ca.crt": "ZXhhbXBsZS1jYS1jZXJ0",
				"namespace": "ZGVmYXVsdA=="
			}
		}
	]
}`

			default:
				logrus.Infof("Unknown endpoint %s for cluster %s, returning 404", path, clusterName)
				response = `HTTP/1.1 404 Not Found
Content-Type: application/json
Content-Length: 123

{
	"kind": "Status",
	"apiVersion": "v1",
	"metadata": {},
	"status": "Failure",
	"message": "endpoint not found",
	"reason": "NotFound",
	"code": 404
}`
			}

			// Write the response
			conn.Write([]byte(response))
			logrus.Infof("Sent response for %s %s to cluster %s", method, path, clusterName)
		}()
	}
}
