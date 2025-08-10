package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
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
	firstClusterConnected bool // track first fake cluster connection
	mockCertPEM           []byte
	nameCounters       map[string]int // track next suffix per base name
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
		nameCounters:      make(map[string]int),
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
	// Start mock HTTPS API server once on localhost:4567
	if len(a.mockCertPEM) == 0 {
		certPEM, _, cert, err := generateSelfSignedCert("localhost")
		if err != nil {
			logrus.Errorf("cert generation failed: %v", err)
			return
		}
		a.mockCertPEM = certPEM
		apiAddr := "127.0.0.1:4567"
		go a.startMockHTTPSAPIServer(apiAddr, cert, clusterName)
	}
	localAPI := "127.0.0.1:4567"
	caB64 := base64.StdEncoding.EncodeToString(a.mockCertPEM)
	clusterParams := map[string]interface{}{
		"address": localAPI,
		"caCert":  caB64,
		"token":   token,
	}
	params := map[string]interface{}{"cluster": clusterParams}
	payload, err := json.Marshal(params)
	if err != nil {
		logrus.Errorf("marshal params: %v", err)
		return
	}
	encodedParams := base64.StdEncoding.EncodeToString(payload)
	logrus.Infof("Prepared tunnel params for %s: %s", clusterName, string(payload))

	headers := http.Header{}
	headers.Set("X-API-Tunnel-Token", token)
	headers.Set("X-API-Tunnel-Params", encodedParams)

	allowFunc := func(proto, address string) bool {
		return proto == "tcp" && address == localAPI
	}

	onConnect := func(ctx context.Context, s *remotedialer.Session) error {
		logrus.Infof("WebSocket connected for %s; Rancher should now dial %s", clusterName, localAPI)
		return nil
	}

	logrus.Infof("Connecting with mock address %s", localAPI)
	go remotedialer.ClientConnect(a.ctx, wsURL, headers, nil, allowFunc, onConnect)
	a.firstClusterConnected = true
}

func (a *ScaleAgent) sendClusterDataViaRemotedialer(clusterName string, session *remotedialer.Session, clusterInfo *ClusterInfo) {
	logrus.Infof("Starting simulated cluster data transmission for %s", clusterName)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			logrus.Debugf("Sending periodic cluster data for %s", clusterName)
		case <-a.ctx.Done():
			logrus.Infof("Stopping cluster data transmission for %s", clusterName)
			return
		}
	}
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

	if a.nameCounters == nil { a.nameCounters = make(map[string]int) }
	baseName := req.Name
	attemptName := baseName
	var clusterID string
	var err error
	var clusterInfo *ClusterInfo

	for attempt := 0; attempt < 15; attempt++ { // more attempts for deterministic suffixes
		// If in-memory duplicate, increment suffix
		if _, exists := a.clusters[attemptName]; exists {
			idx := a.nameCounters[baseName]
			idx++
			a.nameCounters[baseName] = idx
			attemptName = fmt.Sprintf("%s-%d", baseName, idx)
		}

		clusterInfo = a.createClusterFromTemplate(attemptName)
		clusterID, err = a.createClusterInRancher(attemptName)
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "\"code\":\"NotUnique\"") {
			idx := a.nameCounters[baseName]
			idx++
			a.nameCounters[baseName] = idx
			old := attemptName
			attemptName = fmt.Sprintf("%s-%d", baseName, idx)
			logrus.Warnf("Cluster name '%s' not unique, retrying with '%s'", old, attemptName)
			continue
		}
		logrus.Errorf("Failed to create cluster in Rancher: %v", err)
		http.Error(w, "Failed to create cluster", http.StatusInternalServerError)
		return
	}

	if err != nil {
		http.Error(w, "Failed to create cluster after retries", http.StatusInternalServerError)
		return
	}

	clusterInfo.ClusterID = clusterID
	clusterInfo.Name = attemptName
	if attemptName != baseName {
		logrus.Infof("Cluster name adjusted from %s to unique %s", baseName, attemptName)
	}
	if a.clusters == nil { a.clusters = map[string]*ClusterInfo{} }
	if attemptName == "template" { attemptName = attemptName + "-real" }
	if a.clusters[attemptName] != nil { logrus.Warnf("Overwriting existing simulated cluster entry for %s", attemptName) }
	a.clusters[attemptName] = clusterInfo

	if len(a.clusters) == 2 { logrus.Info("First cluster created, WebSocket connection to Rancher will start") }

	response := CreateClusterResponse{ Success: true, Message: fmt.Sprintf("Cluster %s created successfully", attemptName), ClusterID: clusterID }
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)

	if len(a.clusters) == 2 { go func(ci *ClusterInfo) { time.Sleep(2 * time.Second); logrus.Infof("[single-cluster] Initiating first fake cluster websocket flow for %s", ci.Name); a.connectClusterToRancher(ci.Name, ci.ClusterID, ci) }(clusterInfo) }
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

// Deprecated: legacy startLocalAPIServer replaced by mock HTTPS server.
func (a *ScaleAgent) startLocalAPIServer(clusterName string, port int) { logrus.Debugf("startLocalAPIServer deprecated: cluster=%s port=%d", clusterName, port) }

// Forward declarations (ensure functions exist before use)
// generateSelfSignedCert and startMockHTTPSAPIServer are implemented later in file.
func generateSelfSignedCert(host string) ([]byte, []byte, tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil { return nil, nil, tls.Certificate{}, fmt.Errorf("generate key: %w", err) }
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil { return nil, nil, tls.Certificate{}, fmt.Errorf("serial: %w", err) }
	tmpl := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{CommonName: host, Organization: []string{"scale-cluster-agent"}},
		DNSNames:    []string{"localhost", host},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:        true,
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil { return nil, nil, tls.Certificate{}, fmt.Errorf("create cert: %w", err) }
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	crt, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil { return nil, nil, tls.Certificate{}, fmt.Errorf("x509 key pair: %w", err) }
	return certPEM, keyPEM, crt, nil
}

func (a *ScaleAgent) startMockHTTPSAPIServer(addr string, cert tls.Certificate, clusterName string) {
    namespaces := map[string]map[string]interface{}{}
    serviceAccounts := map[string]map[string]interface{}{}
    secrets := map[string]map[string]interface{}{}
    clusterRoles := map[string]interface{}{}
    clusterRoleBindings := map[string]interface{}{}
    ensureNS := func(ns string) {
        if _, ok := namespaces[ns]; !ok {
            namespaces[ns] = map[string]interface{}{ "apiVersion":"v1","kind":"Namespace","metadata":map[string]interface{}{"name":ns},"status":map[string]interface{}{"phase":"Active"} }
        }
        if _, ok := serviceAccounts[ns]; !ok { serviceAccounts[ns] = map[string]interface{}{} }
        if _, ok := secrets[ns]; !ok { secrets[ns] = map[string]interface{}{} }
    }
    ensureNS("kube-system"); ensureNS("cattle-impersonation-system")
    mux := http.NewServeMux()
    logReq := func(h http.HandlerFunc) http.HandlerFunc { return func(w http.ResponseWriter, r *http.Request){ logrus.Debugf("MOCK API %s %s (cluster=%s)", r.Method, r.URL.Path, clusterName); h(w,r) } }
    writeJSON := func(w http.ResponseWriter, status int, obj interface{}) { w.Header().Set("Content-Type","application/json"); w.WriteHeader(status); _ = json.NewEncoder(w).Encode(obj) }
    genUID := func(name string) string { return fmt.Sprintf("%s-%d", name, time.Now().UnixNano()) }
    // Basic endpoints
    mux.HandleFunc("/healthz", logReq(func(w http.ResponseWriter, r *http.Request){ writeJSON(w,200,map[string]string{"status":"ok"}) }))
    mux.HandleFunc("/readyz", logReq(func(w http.ResponseWriter, r *http.Request){ writeJSON(w,200,map[string]string{"status":"ok"}) }))
    mux.HandleFunc("/livez", logReq(func(w http.ResponseWriter, r *http.Request){ w.WriteHeader(200); _,_ = w.Write([]byte("ok")) }))
    mux.HandleFunc("/version", logReq(func(w http.ResponseWriter, r *http.Request){ writeJSON(w,200,map[string]string{"major":"1","minor":"28","gitVersion":"v1.28.5","platform":"linux/amd64"}) }))
    mux.HandleFunc("/api", logReq(func(w http.ResponseWriter, r *http.Request){ writeJSON(w,200,map[string]interface{}{"kind":"APIVersions","versions":[]string{"v1"}}) }))
    mux.HandleFunc("/api/v1", logReq(func(w http.ResponseWriter, r *http.Request){ writeJSON(w,200,map[string]interface{}{"kind":"APIResourceList","apiVersion":"v1","groupVersion":"v1","resources":[]map[string]interface{}{{"name":"pods","namespaced":true,"kind":"Pod"},{"name":"services","namespaced":true,"kind":"Service"},{"name":"nodes","namespaced":false,"kind":"Node"},{"name":"namespaces","namespaced":false,"kind":"Namespace"},{"name":"serviceaccounts","namespaced":true,"kind":"ServiceAccount"},{"name":"secrets","namespaced":true,"kind":"Secret"}}}) }))
    mux.HandleFunc("/apis", logReq(func(w http.ResponseWriter, r *http.Request){ writeJSON(w,200,map[string]interface{}{"kind":"APIGroupList","apiVersion":"v1","groups":[]map[string]interface{}{{"name":"rbac.authorization.k8s.io","versions":[]map[string]string{{"groupVersion":"rbac.authorization.k8s.io/v1","version":"v1"}},"preferredVersion":map[string]string{"groupVersion":"rbac.authorization.k8s.io/v1","version":"v1"}},{"name":"apps","versions":[]map[string]string{{"groupVersion":"apps/v1","version":"v1"}},"preferredVersion":map[string]string{"groupVersion":"apps/v1","version":"v1"}}}}) }))
    mux.HandleFunc("/apis/apps", logReq(func(w http.ResponseWriter, r *http.Request){ writeJSON(w,200,map[string]interface{}{"kind":"APIGroup","apiVersion":"v1","name":"apps","versions":[]map[string]string{{"groupVersion":"apps/v1","version":"v1"}},"preferredVersion":map[string]string{"groupVersion":"apps/v1","version":"v1"}}) }))
    mux.HandleFunc("/apis/apps/v1", logReq(func(w http.ResponseWriter, r *http.Request){ writeJSON(w,200,map[string]interface{}{"kind":"APIResourceList","apiVersion":"v1","groupVersion":"apps/v1","resources":[]map[string]interface{}{{"name":"deployments","namespaced":true,"kind":"Deployment"}}}) }))
    // RBAC
    mux.HandleFunc("/apis/rbac.authorization.k8s.io/v1/clusterroles", logReq(func(w http.ResponseWriter, r *http.Request){ switch r.Method {case http.MethodGet: items:=[]interface{}{}; for _,cr:= range clusterRoles { items=append(items,cr)}; writeJSON(w,200,map[string]interface{}{"kind":"ClusterRoleList","apiVersion":"rbac.authorization.k8s.io/v1","items":items}); case http.MethodPost: var obj map[string]interface{}; _=json.NewDecoder(r.Body).Decode(&obj); md,_:=obj["metadata"].(map[string]interface{}); if md==nil { md=map[string]interface{}{} }; name,_:=md["name"].(string); if name=="" { name=fmt.Sprintf("cr-%d",time.Now().UnixNano()) }; if _,ex:=clusterRoles[name]; ex { writeJSON(w,409,map[string]interface{}{"kind":"Status","code":409,"reason":"AlreadyExists"}); return }; md["name"]=name; md["uid"]=genUID(name); md["creationTimestamp"]=time.Now().UTC().Format(time.RFC3339); obj["apiVersion"]="rbac.authorization.k8s.io/v1"; obj["kind"]="ClusterRole"; obj["metadata"]=md; clusterRoles[name]=obj; writeJSON(w,201,obj); default: w.WriteHeader(405)} }))
    mux.HandleFunc("/apis/rbac.authorization.k8s.io/v1/clusterrolebindings", logReq(func(w http.ResponseWriter, r *http.Request){ switch r.Method {case http.MethodGet: items:=[]interface{}{}; for _,crb:= range clusterRoleBindings { items=append(items,crb)}; writeJSON(w,200,map[string]interface{}{"kind":"ClusterRoleBindingList","apiVersion":"rbac.authorization.k8s.io/v1","items":items}); case http.MethodPost: var obj map[string]interface{}; _=json.NewDecoder(r.Body).Decode(&obj); md,_:=obj["metadata"].(map[string]interface{}); if md==nil { md=map[string]interface{}{} }; name,_:=md["name"].(string); if name=="" { name=fmt.Sprintf("crb-%d",time.Now().UnixNano()) }; if _,ex:=clusterRoleBindings[name]; ex { writeJSON(w,409,map[string]interface{}{"kind":"Status","code":409,"reason":"AlreadyExists"}); return }; md["name"]=name; md["uid"]=genUID(name); md["creationTimestamp"]=time.Now().UTC().Format(time.RFC3339); obj["apiVersion"]="rbac.authorization.k8s.io/v1"; obj["kind"]="ClusterRoleBinding"; obj["metadata"]=md; clusterRoleBindings[name]=obj; writeJSON(w,201,obj); default: w.WriteHeader(405)} }))
    mux.HandleFunc("/apis/rbac.authorization.k8s.io/v1/roles", logReq(func(w http.ResponseWriter, r *http.Request){ writeJSON(w,200,map[string]interface{}{"kind":"RoleList","apiVersion":"rbac.authorization.k8s.io/v1","items":[]interface{}{}}) }))
    mux.HandleFunc("/apis/rbac.authorization.k8s.io/v1/rolebindings", logReq(func(w http.ResponseWriter, r *http.Request){ writeJSON(w,200,map[string]interface{}{"kind":"RoleBindingList","apiVersion":"rbac.authorization.k8s.io/v1","items":[]interface{}{}}) }))
    // Namespaces list/create
    mux.HandleFunc("/api/v1/namespaces", logReq(func(w http.ResponseWriter, r *http.Request){ switch r.Method {case http.MethodGet: items:=[]interface{}{}; for _,nsObj:= range namespaces { items=append(items,nsObj)}; writeJSON(w,200,map[string]interface{}{"kind":"NamespaceList","apiVersion":"v1","items":items}); case http.MethodPost: var obj map[string]interface{}; _=json.NewDecoder(r.Body).Decode(&obj); md,_:=obj["metadata"].(map[string]interface{}); if md==nil { md=map[string]interface{}{} }; name,_:=md["name"].(string); if name=="" { name=fmt.Sprintf("ns-%d",time.Now().UnixNano()) }; if _,ex:=namespaces[name]; ex { writeJSON(w,409,map[string]interface{}{"kind":"Status","code":409,"reason":"AlreadyExists"}); return }; ensureNS(name); writeJSON(w,201,namespaces[name]); default: w.WriteHeader(405)} }))
    // Namespaced resources & namespace GET by name
    mux.HandleFunc("/api/v1/namespaces/", logReq(func(w http.ResponseWriter, r *http.Request){ parts:=strings.Split(strings.TrimPrefix(r.URL.Path,"/api/v1/namespaces/"),"/")
        if len(parts)==1 || parts[1]=="" { // namespace by name
            if r.Method!=http.MethodGet { http.NotFound(w,r); return }
            name := strings.TrimSuffix(parts[0],"/")
            if ns,ok:=namespaces[name]; ok { writeJSON(w,200,ns); return }
            writeJSON(w,404,map[string]interface{}{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"NotFound","message":fmt.Sprintf("namespaces \"%s\" not found",name),"code":404}); return }
        ns := parts[0]; ensureNS(ns)
        if len(parts) >= 2 {
            res := parts[1]
            switch res {
            case "serviceaccounts":
                saMap := serviceAccounts[ns]
                switch r.Method {
                case http.MethodGet:
                    items := []interface{}{}
                    for _, v := range saMap { items = append(items, v) }
                    writeJSON(w,200,map[string]interface{}{"kind":"ServiceAccountList","apiVersion":"v1","items":items}); return
                case http.MethodPost:
                    var obj map[string]interface{}; _=json.NewDecoder(r.Body).Decode(&obj)
                    md,_:=obj["metadata"].(map[string]interface{}); if md==nil { md=map[string]interface{}{} }
                    name,_:=md["name"].(string); if name=="" { name=fmt.Sprintf("sa-%d",time.Now().UnixNano()) }
                    if _,exists:=saMap[name]; exists { writeJSON(w,409,map[string]interface{}{"kind":"Status","code":409,"reason":"AlreadyExists"}); return }
                    md["name"],md["uid"],md["creationTimestamp"]=name,genUID(name),time.Now().UTC().Format(time.RFC3339)
                    obj["apiVersion"],obj["kind"],obj["metadata"]="v1","ServiceAccount",md
                    saMap[name]=obj; writeJSON(w,201,obj); return
                }
            case "secrets":
                secMap := secrets[ns]
                switch r.Method {
                case http.MethodGet:
                    items := []interface{}{}
                    for _, v := range secMap { items = append(items, v) }
                    writeJSON(w,200,map[string]interface{}{"kind":"SecretList","apiVersion":"v1","items":items}); return
                case http.MethodPost:
                    var obj map[string]interface{}; _=json.NewDecoder(r.Body).Decode(&obj)
                    md,_:=obj["metadata"].(map[string]interface{}); if md==nil { md=map[string]interface{}{} }
                    name,_:=md["name"].(string); if name=="" { name=fmt.Sprintf("secret-%d",time.Now().UnixNano()) }
                    if _,exists:=secMap[name]; exists { writeJSON(w,409,map[string]interface{}{"kind":"Status","code":409,"reason":"AlreadyExists"}); return }
                    md["name"],md["uid"],md["creationTimestamp"]=name,genUID(name),time.Now().UTC().Format(time.RFC3339)
                    obj["apiVersion"],obj["kind"],obj["metadata"]="v1","Secret",md
                    if _,ok:=obj["type"].(string); !ok { obj["type"]="Opaque" }
                    if _,ok:=obj["data"].(map[string]interface{}); !ok { obj["data"]=map[string]interface{}{} }
                    secMap[name]=obj; writeJSON(w,201,obj); return
                }
            }
        }
        http.NotFound(w,r)
    }))
    // Misc lists
    mux.HandleFunc("/api/v1/limitranges", logReq(func(w http.ResponseWriter, r *http.Request){ if r.Method==http.MethodGet { writeJSON(w,200,map[string]interface{}{"kind":"LimitRangeList","apiVersion":"v1","items":[]interface{}{}}); return }; w.WriteHeader(405) }))
    mux.HandleFunc("/api/v1/resourcequotas", logReq(func(w http.ResponseWriter, r *http.Request){ if r.Method==http.MethodGet { writeJSON(w,200,map[string]interface{}{"kind":"ResourceQuotaList","apiVersion":"v1","items":[]interface{}{}}); return }; w.WriteHeader(405) }))
    mux.HandleFunc("/apis/apiregistration.k8s.io/v1/apiservices", logReq(func(w http.ResponseWriter, r *http.Request){ if r.Method==http.MethodGet { items:=[]interface{}{ map[string]interface{}{"apiVersion":"apiregistration.k8s.io/v1","kind":"APIService","metadata":map[string]interface{}{"name":"v1."},"spec":map[string]interface{}{"group":"","version":"v1","insecureSkipTLSVerify":true},"status":map[string]interface{}{"conditions":[]interface{}{map[string]interface{}{"type":"Available","status":"True"}}}}, map[string]interface{}{"apiVersion":"apiregistration.k8s.io/v1","kind":"APIService","metadata":map[string]interface{}{"name":"v1.apps"},"spec":map[string]interface{}{"group":"apps","version":"v1","insecureSkipTLSVerify":true},"status":map[string]interface{}{"conditions":[]interface{}{map[string]interface{}{"type":"Available","status":"True"}}}}, }; writeJSON(w,200,map[string]interface{}{"kind":"APIServiceList","apiVersion":"apiregistration.k8s.io/v1","items":items}); return }; w.WriteHeader(405) }))
    mux.HandleFunc("/api/v1/nodes", logReq(func(w http.ResponseWriter, r *http.Request){ if r.Method==http.MethodGet { nodeName:=fmt.Sprintf("%s-node-1",clusterName); node:=map[string]interface{}{"apiVersion":"v1","kind":"Node","metadata":map[string]interface{}{"name":nodeName,"uid":genUID(nodeName),"creationTimestamp":time.Now().UTC().Format(time.RFC3339)},"status":map[string]interface{}{"conditions":[]interface{}{map[string]interface{}{"type":"Ready","status":"True","lastHeartbeatTime":time.Now().UTC().Format(time.RFC3339),"lastTransitionTime":time.Now().UTC().Format(time.RFC3339)}},"capacity":map[string]interface{}{"cpu":"4","memory":"8192Mi","pods":"110"}}}; writeJSON(w,200,map[string]interface{}{"kind":"NodeList","apiVersion":"v1","items":[]interface{}{node}}); return }; w.WriteHeader(405) }))
    mux.HandleFunc("/", logReq(func(w http.ResponseWriter, r *http.Request){ if r.URL.Path=="/" { writeJSON(w,200,map[string]interface{}{"kind":"APIVersions","versions":[]string{"v1"}}); return }; http.NotFound(w,r) }))
    server := &http.Server{Addr: addr, Handler: mux, TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}}}
    go func(){
        logrus.Infof("Mock HTTPS API server listening at https://%s (cluster=%s)", addr, clusterName)
        ln, err := tls.Listen("tcp", addr, server.TLSConfig)
        if err != nil { logrus.Errorf("mock https api listen error: %v", err); return }
        if err := server.Serve(ln); err != nil && err != http.ErrServerClosed { logrus.Errorf("mock https api server error: %v", err) }
    }()
}
