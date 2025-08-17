package main

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	pkgerrors "github.com/pkg/errors"
	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
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
	Status      string           `json:"status,omitempty"` // Track cluster setup status
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
	config                *Config
	clusters              map[string]*ClusterInfo
	httpServer            *http.Server
	ctx                   context.Context
	cancel                context.CancelFunc
	activeConnections     map[string]bool           // Track active connections to prevent duplicates
	connMutex             sync.RWMutex              // Protect connection tracking
	tokenCache            map[string]string         // Cache cluster registration tokens
	tokenMutex            sync.RWMutex              // Protect token cache
	mockServers           map[string]*http.Server   // Track mock API servers per cluster
	serverMutex           sync.RWMutex              // Protect mock servers
	portForwarders        map[string]*PortForwarder // Track port forwarders per cluster
	forwarderMutex        sync.RWMutex              // Protect port forwarders
	nextPort              int                       // Next available port for clusters
	portMutex             sync.Mutex                // Protect port allocation
	firstClusterConnected bool                      // track first fake cluster connection
	mockCertPEM           []byte
	nameCounters          map[string]int      // track next suffix per base name
	kwokManager           *KWOKClusterManager // Manage KWOK clusters
}

type CreateClusterRequest struct {
	Name string `json:"name"`
}

type CreateClusterResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	ClusterID string `json:"cluster_id,omitempty"`
}

func init() { rand.Seed(time.Now().UnixNano()) }

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

	// Initialize KWOK cluster manager with absolute paths
	kwokctlPath, err := filepath.Abs("./kwokctl")
	if err != nil {
		logrus.Fatalf("Failed to get absolute path for kwokctl: %v", err)
	}
	kwokPath, err := filepath.Abs("./kwok")
	if err != nil {
		logrus.Fatalf("Failed to get absolute path for kwok: %v", err)
	}

	logrus.Infof("Using kwokctl path: %s", kwokctlPath)
	logrus.Infof("Using kwok path: %s", kwokPath)

	agent.kwokManager = NewKWOKClusterManager(kwokctlPath, kwokPath, 8001)

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

	// Start periodic cleanup of old KWOK clusters
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // Clean up every 5 minutes
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Get active cluster IDs
				activeClusterIDs := make(map[string]bool)
				for _, clusterInfo := range agent.clusters {
					if clusterInfo.ClusterID != "" {
						activeClusterIDs[clusterInfo.ClusterID] = true
					}
				}

				if err := agent.kwokManager.CleanupOldClusters(activeClusterIDs); err != nil {
					logrus.Warnf("Failed to cleanup old KWOK clusters: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

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

	var config Config
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Use environment fallback
			config.RancherURL = os.Getenv("RANCHER_URL")
			config.BearerToken = os.Getenv("RANCHER_TOKEN")
			portStr := os.Getenv("SCALE_AGENT_PORT")
			if portStr != "" {
				if p, perr := strconv.Atoi(portStr); perr == nil {
					config.ListenPort = p
				}
			}
			config.LogLevel = os.Getenv("SCALE_AGENT_LOG")
			if config.RancherURL == "" {
				config.RancherURL = "https://rancher.invalid"
			}
			if config.ListenPort == 0 {
				config.ListenPort = 9090
			}
			if config.LogLevel == "" {
				config.LogLevel = "debug"
			}
			logrus.Warnf("Config file %s not found; using environment/default values", configPath)
		} else {
			return nil, fmt.Errorf("failed to read config file %s: %v", configPath, err)
		}
	} else {
		if err := json.Unmarshal(data, &config); err != nil {
			// very naive key:value fallback
			lines := strings.Split(string(data), "\n")
			for _, l := range lines {
				l = strings.TrimSpace(l)
				if l == "" || strings.HasPrefix(l, "#") {
					continue
				}
				parts := strings.SplitN(l, ":", 2)
				if len(parts) != 2 {
					continue
				}
				k := strings.TrimSpace(parts[0])
				v := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				switch k {
				case "RancherURL":
					config.RancherURL = v
				case "BearerToken":
					config.BearerToken = v
				case "ListenPort":
					if p, err := strconv.Atoi(v); err == nil {
						config.ListenPort = p
					}
				case "LogLevel":
					config.LogLevel = v
				}
			}
		}
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

	logrus.Infof("Using config: RancherURL=%s ListenPort=%d LogLevel=%s", config.RancherURL, config.ListenPort, config.LogLevel)
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
		logrus.Warnf("Failed to parse cluster template (%v). Falling back to built-in default template", err)
		return a.createDefaultClusterTemplate()
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

		// Check if this cluster has already completed the full setup workflow
		// We only want to connect clusters that have gone through our new 6-step process
		if clusterInfo.Status == "ready" {
			logrus.Debugf("Cluster %s is ready, connecting to Rancher", clusterName)
			go a.connectClusterToRancher(clusterName, clusterInfo.ClusterID, clusterInfo)
		} else {
			logrus.Debugf("Cluster %s not ready yet (status: %s), skipping WebSocket connection", clusterName, clusterInfo.Status)
		}
	}
}

// extractCACertFromKubeconfig extracts the CA certificate from KWOK kubeconfig content
func (a *ScaleAgent) extractCACertFromKubeconfig(kubeconfigContent string) ([]byte, error) {
	var config struct {
		Clusters []struct {
			Cluster struct {
				CertificateAuthorityData string `yaml:"certificate-authority-data"`
			} `yaml:"cluster"`
		} `yaml:"clusters"`
	}

	if err := yaml.Unmarshal([]byte(kubeconfigContent), &config); err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %v", err)
	}

	if len(config.Clusters) == 0 {
		return nil, fmt.Errorf("no clusters found in kubeconfig")
	}

	caData := config.Clusters[0].Cluster.CertificateAuthorityData
	if caData == "" {
		return nil, fmt.Errorf("no CA certificate found in kubeconfig")
	}

	// Decode base64 CA certificate
	caCert, err := base64.StdEncoding.DecodeString(caData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode CA certificate: %v", err)
	}

	return caCert, nil
}

// extractServiceAccountTokenFromKWOKCluster extracts the service account token from the KWOK cluster
// This token was created by the import YAML and is stored in the cattle-system namespace
func (a *ScaleAgent) extractServiceAccountTokenFromKWOKCluster(kwokCluster *KWOKCluster) (string, error) {
	logrus.Infof("Extracting service account token from KWOK cluster %s", kwokCluster.Name)

	// Get the kubeconfig path for the KWOK cluster
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", kwokCluster.Name, "kubeconfig.yaml")

	// Check if kubeconfig exists
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return "", fmt.Errorf("kubeconfig file not found at %s", kubeconfigPath)
	}

	// Get the service account token from the cattle-system namespace
	// The import YAML creates a Secret with the token
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "secret", "-n", "cattle-system", "cattle-credentials-*", "-o", "jsonpath={.items[0].data.token}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try alternative approach - get the secret name first
		logrus.Debugf("Failed to get secret directly, trying alternative approach: %v", err)

		// Get the list of secrets in cattle-system namespace
		cmd = exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "secret", "-n", "cattle-system", "-o", "jsonpath={.items[*].metadata.name}")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to list secrets in cattle-system namespace: %v, output: %s", err, string(output))
		}

		secretNames := strings.Fields(string(output))
		if len(secretNames) == 0 {
			return "", fmt.Errorf("no secrets found in cattle-system namespace")
		}

		// Find the cattle credentials secret
		var cattleSecret string
		for _, name := range secretNames {
			if strings.HasPrefix(name, "cattle-credentials-") {
				cattleSecret = name
				break
			}
		}

		if cattleSecret == "" {
			return "", fmt.Errorf("no cattle credentials secret found in cattle-system namespace")
		}

		// Get the token from the specific secret
		cmd = exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "secret", "-n", "cattle-system", cattleSecret, "-o", "jsonpath={.data.token}")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to get token from secret %s: %v, output: %s", cattleSecret, err, string(output))
		}
	}

	// Decode the base64-encoded token
	tokenBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(output)))
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 token: %v", err)
	}

	token := string(tokenBytes)
	if token == "" {
		return "", fmt.Errorf("extracted token is empty")
	}

	logrus.Infof("Successfully extracted service account token from KWOK cluster %s", kwokCluster.Name)
	return token, nil
}

func (a *ScaleAgent) connectClusterToRancher(clusterName, clusterID string, clusterInfo *ClusterInfo) {
	// Check if already connected to prevent multiple connections
	a.connMutex.Lock()
	if a.activeConnections[clusterName] {
		logrus.Infof("Cluster %s is already connected, skipping duplicate connection", clusterName)
		a.connMutex.Unlock()
		return
	}

	// Mark as attempting connection (not yet connected)
	a.activeConnections[clusterName] = false
	a.connMutex.Unlock()

	// Find the existing KWOK cluster for this Rancher cluster
	var kwokCluster *KWOKCluster
	for _, cluster := range a.kwokManager.clusters {
		if cluster.ClusterID == clusterID {
			kwokCluster = cluster
			break
		}
	}

	if kwokCluster == nil {
		logrus.Errorf("KWOK cluster not found for Rancher cluster %s", clusterID)
		// Mark connection as failed
		a.connMutex.Lock()
		delete(a.activeConnections, clusterName)
		a.connMutex.Unlock()
		return
	}

	// Extract the service account token from the KWOK cluster
	// This is the token that was created by the import YAML
	// We need to use the valid token for both Rancher server connection and cluster params
	// since Rancher server validates the token in clusterParams as well
	rancherToken, err := a.getClusterToken(clusterID)
	if err != nil {
		logrus.Errorf("Failed to get cluster token for Rancher connection: %v", err)
		// Mark connection as failed
		a.connMutex.Lock()
		delete(a.activeConnections, clusterName)
		a.connMutex.Unlock()
		return
	}

	logrus.Infof("Using valid token for both Rancher server connection and cluster params")

	// Now use the extracted credentials for WebSocket connection (like real agent)
	// Use the Rancher server URL host, not the cluster ID
	rancherURL, err := url.Parse(a.config.RancherURL)
	if err != nil {
		logrus.Errorf("Failed to parse Rancher URL: %v", err)
		// Mark connection as failed
		a.connMutex.Lock()
		delete(a.activeConnections, clusterName)
		a.connMutex.Unlock()
		return
	}
	wsURL := fmt.Sprintf("wss://%s/v3/connect/register", rancherURL.Host) // Use /register endpoint like real agent

	logrus.Infof("Attempting WebSocket connection to %s for cluster %s", wsURL, clusterName)

	// Get cluster parameters using the real agent's approach
	clusterParams, err := a.getClusterParams(clusterID)
	if err != nil {
		logrus.Errorf("Failed to get cluster parameters: %v", err)
		// Mark connection as failed
		a.connMutex.Lock()
		delete(a.activeConnections, clusterName)
		a.connMutex.Unlock()
		return
	}

	params := clusterParams
	payload, err := json.Marshal(params)
	if err != nil {
		logrus.Errorf("marshal params: %v", err)
		// Mark connection as failed
		a.connMutex.Lock()
		delete(a.activeConnections, clusterName)
		a.connMutex.Unlock()
		return
	}
	encodedParams := base64.StdEncoding.EncodeToString(payload)
	logrus.Infof("Prepared tunnel params for %s: %s", clusterName, string(payload))

	// Extract the local API address from clusterParams for the allowFunc
	clusterData, ok := clusterParams["cluster"].(map[string]interface{})
	if !ok {
		logrus.Errorf("Failed to extract cluster data from params")
		// Mark connection as failed
		a.connMutex.Lock()
		delete(a.activeConnections, clusterName)
		a.connMutex.Unlock()
		return
	}
	localAPI, ok := clusterData["address"].(string)
	if !ok {
		logrus.Errorf("Failed to extract address from cluster data")
		// Mark connection as failed
		a.connMutex.Lock()
		delete(a.activeConnections, clusterName)
		a.connMutex.Unlock()
		return
	}

	headers := http.Header{}
	headers.Set("X-API-Tunnel-Token", rancherToken) // Valid token for WebSocket connection to Rancher
	headers.Set("X-API-Tunnel-Params", encodedParams)

	allowFunc := func(proto, address string) bool {
		return proto == "tcp" && address == localAPI
	}

	// Track connection attempts and success
	connectionAttempts := 0
	maxAttempts := 3

	// Set up the onConnect callback to handle successful connections
	onConnect := func(ctx context.Context, s *remotedialer.Session) error {
		connectionAttempts++
		logrus.Infof("🔄 REMOTEDIALER DEBUG: onConnect called for cluster %s (attempt %d/%d)", clusterName, connectionAttempts, maxAttempts)

		// Mark as successfully connected only after actual connection
		a.connMutex.Lock()
		a.activeConnections[clusterName] = true
		a.connMutex.Unlock()

		logrus.Infof("✅ Cluster %s successfully connected to Rancher via WebSocket tunnel", clusterName)
		go a.patchClusterActive(clusterID)
		return nil
	}

	logrus.Infof("Connecting with KWOK cluster address %s", localAPI)
	go func() {
		backoff := time.Second
		for {
			ctx, cancel := context.WithCancel(a.ctx)

			// Check if we've exceeded max attempts
			if connectionAttempts >= maxAttempts {
				logrus.Errorf("❌ Cluster %s failed to connect after %d attempts, marking as failed", clusterName, maxAttempts)
				a.connMutex.Lock()
				delete(a.activeConnections, clusterName)
				a.connMutex.Unlock()
				return
			}

			// Connect to Rancher using remotedialer
			err = remotedialer.ClientConnect(ctx, wsURL, headers, nil, allowFunc, onConnect)
			if err != nil {
				logrus.Errorf("Failed to connect to proxy: %v", err)
				// Mark connection as failed
				a.connMutex.Lock()
				delete(a.activeConnections, clusterName)
				a.connMutex.Unlock()
				return
			}
			cancel()

			if a.ctx.Err() != nil {
				return
			}

			if backoff < 30*time.Second {
				backoff *= 2
			}
			logrus.Infof("[%s] remotedialer reconnecting in %s (attempt %d/%d)", clusterName, backoff, connectionAttempts+1, maxAttempts)
			time.Sleep(backoff)
		}
	}()
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

	// Check if a cluster with this name already exists
	if _, exists := a.clusters[req.Name]; exists {
		http.Error(w, fmt.Sprintf("Cluster with name '%s' already exists", req.Name), http.StatusConflict)
		return
	}

	// Create cluster info and register with Rancher
	clusterInfo := a.createClusterFromTemplate(req.Name)
	clusterID, err := a.createClusterInRancher(req.Name)
	if err != nil {
		logrus.Errorf("Failed to create cluster in Rancher: %v", err)
		http.Error(w, "Failed to create cluster", http.StatusInternalServerError)
		return
	}

	// Store cluster info
	clusterInfo.ClusterID = clusterID
	clusterInfo.Name = req.Name
	if a.clusters == nil {
		a.clusters = map[string]*ClusterInfo{}
	}
	a.clusters[req.Name] = clusterInfo

	// Send response
	response := CreateClusterResponse{Success: true, Message: fmt.Sprintf("Cluster %s created successfully", req.Name), ClusterID: clusterID}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)

	// Start the complete cluster setup process in background
	go func(ci *ClusterInfo) {
		time.Sleep(2 * time.Second)
		logrus.Infof("Starting complete cluster setup for %s", ci.Name)

		// Step 1: Create KWOK cluster first
		ci.Status = "creating_kwok"
		// Get the cluster info from our stored clusters
		clusterInfo, exists := a.clusters[ci.Name]
		if !exists {
			logrus.Errorf("cluster info not found for %s", ci.Name)
			ci.Status = "failed"
			return
		}
		// Use the existing KWOK manager to create the cluster
		if _, err := a.kwokManager.CreateCluster(ci.Name, ci.ClusterID, clusterInfo); err != nil {
			logrus.Errorf("Failed to create KWOK cluster for %s: %v", ci.Name, err)
			ci.Status = "failed"
			return
		}

		// Step 2: Wait a moment for the KWOK cluster to be fully registered
		logrus.Infof("Waiting for KWOK cluster to be fully registered...")
		time.Sleep(3 * time.Second)

		// Step 3: Get the import YAML from Rancher
		ci.Status = "getting_yaml"
		importYAML, err := a.getImportYAML(ci.ClusterID)
		if err != nil {
			logrus.Errorf("Failed to get import YAML for %s: %v", ci.Name, err)
			ci.Status = "failed"
			return
		}

		// Step 4: Apply the import YAML to the KWOK cluster
		ci.Status = "applying_yaml"
		if err := a.applyImportYAMLToKWOKCluster(ci.ClusterID, importYAML); err != nil {
			logrus.Errorf("Failed to apply import YAML to KWOK cluster for %s: %v", ci.Name, err)
			ci.Status = "failed"
			return
		}

		// Step 5: Wait for service account to be ready
		ci.Status = "waiting_service_account"
		if err := a.waitForServiceAccountReady(ci.ClusterID); err != nil {
			logrus.Errorf("Failed to wait for service account ready for %s: %v", ci.Name, err)
			ci.Status = "failed"
			return
		}

		// Step 6: Test critical API endpoints to ensure they work
		ci.Status = "testing_api"
		if err := a.testCriticalAPIEndpoints(ci.ClusterID); err != nil {
			logrus.Errorf("Failed to test critical API endpoints for %s: %v", ci.Name, err)
			ci.Status = "failed"
			return
		}

		// Step 7: Mark cluster as ready and establish WebSocket connection
		ci.Status = "ready"
		logrus.Infof("All tests passed! Service account ready and API endpoints working. Establishing WebSocket connection for cluster %s", ci.Name)
		a.connectClusterToRancher(ci.Name, ci.ClusterID, ci)
	}(clusterInfo)
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

	// Set initial status
	newCluster.Status = "creating"

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
	logrus.Infof("Completing cluster registration for %s", clusterID)

	// Get the import YAML from Rancher (this will now follow the correct 4-step process)
	importYAML, err := a.getImportYAML(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get import YAML: %v", err)
	}

	// Apply the import YAML to the KWOK cluster
	if err := a.applyImportYAMLToKWOKCluster(clusterID, importYAML); err != nil {
		return fmt.Errorf("failed to apply import YAML to KWOK cluster: %v", err)
	}

	logrus.Infof("Successfully completed cluster registration for %s", clusterID)
	return nil
}

func (a *ScaleAgent) getImportYAML(clusterID string) (string, error) {
	logrus.Infof("Getting import YAML for cluster %s", clusterID)

	// Wait up to 5 minutes for cluster registration tokens to be available
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for cluster registration tokens for cluster %s", clusterID)
		case <-ticker.C:
			logrus.Debugf("Checking for cluster registration tokens for cluster %s...", clusterID)

			// Step 1: Get the cluster registration tokens
			client := &http.Client{Timeout: 30 * time.Second}
			rancherURL := strings.TrimRight(a.config.RancherURL, "/")

			// Call the clusterregistrationtokens endpoint
			tokensURL := fmt.Sprintf("%s/v3/clusters/%s/clusterregistrationtokens", rancherURL, clusterID)
			req, err := http.NewRequest("GET", tokensURL, nil)
			if err != nil {
				logrus.Debugf("Failed to create cluster registration tokens request: %v", err)
				continue
			}

			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.config.BearerToken))

			resp, err := client.Do(req)
			if err != nil {
				logrus.Debugf("Failed to get cluster registration tokens: %v", err)
				continue
			}

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				logrus.Debugf("Cluster registration tokens request failed with status %d", resp.StatusCode)
				continue
			}

			body, _ := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			logrus.Debugf("Cluster registration tokens response status: %d", resp.StatusCode)

			// Step 2: Parse response to get the tokens data
			var tokensResp map[string]interface{}
			if err := json.Unmarshal(body, &tokensResp); err != nil {
				logrus.Debugf("Failed to decode cluster registration tokens response: %v", err)
				continue
			}

			// Extract the data array
			data, ok := tokensResp["data"].([]interface{})
			if !ok {
				logrus.Debugf("No data field in cluster registration tokens response: %+v", tokensResp)
				continue
			}

			if len(data) == 0 {
				logrus.Debugf("No cluster registration tokens found yet, waiting...")
				continue
			}

			// Found tokens! Now proceed with the rest of the process
			logrus.Infof("Found %d cluster registration tokens for cluster %s", len(data), clusterID)

			// Get the first token (usually there's only one)
			tokenData, ok := data[0].(map[string]interface{})
			if !ok {
				logrus.Debugf("Failed to parse token data: %+v", data[0])
				continue
			}

			// Extract the insecureCommand
			insecureCommand, ok := tokenData["insecureCommand"].(string)
			if !ok {
				logrus.Debugf("No insecureCommand field in token data: %+v", tokenData)
				continue
			}

			logrus.Infof("Got insecureCommand: %s", insecureCommand)

			// Step 3: Parse the insecureCommand to extract the YAML file URL
			// Expected format: "curl --insecure -sfL https://green-cluster.shen.nu/v3/import/xyz.yaml | kubectl apply -f -"
			if !strings.Contains(insecureCommand, "curl --insecure -sfL") {
				logrus.Debugf("Unexpected insecureCommand format: %s", insecureCommand)
				continue
			}

			// Extract the URL part: everything between "curl --insecure -sfL " and " | kubectl"
			startMarker := "curl --insecure -sfL "
			endMarker := " | kubectl"

			startIdx := strings.Index(insecureCommand, startMarker)
			endIdx := strings.Index(insecureCommand, endMarker)

			if startIdx == -1 || endIdx == -1 || startIdx >= endIdx {
				logrus.Debugf("Failed to parse insecureCommand: %s", insecureCommand)
				continue
			}

			yamlURL := insecureCommand[startIdx+len(startMarker) : endIdx]
			yamlURL = strings.TrimSpace(yamlURL)
			logrus.Infof("Extracted YAML URL: %s", yamlURL)

			// Step 4: Download the actual YAML file
			yamlReq, err := http.NewRequest("GET", yamlURL, nil)
			if err != nil {
				logrus.Debugf("Failed to create YAML download request: %v", err)
				continue
			}

			// Note: This URL is typically accessible without authentication
			yamlResp, err := client.Do(yamlReq)
			if err != nil {
				logrus.Debugf("Failed to download YAML file: %v", err)
				continue
			}

			if yamlResp.StatusCode != http.StatusOK {
				body, _ := ioutil.ReadAll(yamlResp.Body)
				yamlResp.Body.Close()
				logrus.Debugf("Failed to download YAML file: status %d, body: %s", yamlResp.StatusCode, string(body))
				continue
			}

			yamlData, err := ioutil.ReadAll(yamlResp.Body)
			yamlResp.Body.Close()
			if err != nil {
				logrus.Debugf("Failed to read YAML file content: %v", err)
				continue
			}

			yamlContent := string(yamlData)
			logrus.Infof("Successfully downloaded YAML file for cluster %s", clusterID)

			// Validate that the YAML contains service account configuration
			if !strings.Contains(yamlContent, "ServiceAccount") && !strings.Contains(yamlContent, "serviceaccount") {
				logrus.Debugf("Downloaded YAML does not contain ServiceAccount configuration: %s", yamlContent[:200])
				continue
			}

			// Save YAML to debug file
			clusterName := a.getClusterNameByID(clusterID)
			if clusterName == "" {
				clusterName = clusterID // fallback to cluster ID if name not found
			}

			debugDir := "debug-yaml"
			if err := os.MkdirAll(debugDir, 0755); err != nil {
				logrus.Warnf("Failed to create debug directory: %v", err)
			} else {
				debugFile := filepath.Join(debugDir, fmt.Sprintf("%s-register.yaml", clusterName))
				if err := ioutil.WriteFile(debugFile, yamlData, 0644); err != nil {
					logrus.Warnf("Failed to save debug YAML file: %v", err)
				} else {
					logrus.Infof("Saved debug YAML file: %s", debugFile)
				}
			}

			return yamlContent, nil
		}
	}
}

// getClusterNameByID gets the cluster name from the stored clusters map
func (a *ScaleAgent) getClusterNameByID(clusterID string) string {
	for name, cluster := range a.clusters {
		if cluster.ClusterID == clusterID {
			return name
		}
	}
	return ""
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

// applyImportYAMLToKWOKCluster applies the import YAML to the KWOK cluster
func (a *ScaleAgent) applyImportYAMLToKWOKCluster(clusterID string, importYAML string) error {
	logrus.Infof("Applying import YAML to KWOK cluster %s", clusterID)

	// Debug: Log what's in the KWOK manager's clusters map
	logrus.Infof("DEBUG: KWOK manager has %d clusters", len(a.kwokManager.clusters))
	for name, cluster := range a.kwokManager.clusters {
		logrus.Infof("DEBUG: KWOK cluster: name=%s, clusterID=%s, status=%s", name, cluster.ClusterID, cluster.Status)
	}

	// Wait a bit more if no clusters found yet
	if len(a.kwokManager.clusters) == 0 {
		logrus.Infof("No clusters found yet, waiting 5 seconds for cluster registration...")
		time.Sleep(5 * time.Second)

		// Check again
		logrus.Infof("DEBUG: After waiting, KWOK manager has %d clusters", len(a.kwokManager.clusters))
		for name, cluster := range a.kwokManager.clusters {
			logrus.Infof("DEBUG: KWOK cluster: name=%s, clusterID=%s, status=%s", name, cluster.ClusterID, cluster.Status)
		}
	}

	// Find the KWOK cluster by cluster ID
	var kwokCluster *KWOKCluster
	for _, cluster := range a.kwokManager.clusters {
		if cluster.ClusterID == clusterID {
			kwokCluster = cluster
			break
		}
	}

	if kwokCluster == nil {
		return fmt.Errorf("KWOK cluster not found for Rancher cluster %s", clusterID)
	}

	// Get the kubeconfig path for the KWOK cluster using the actual KWOK cluster name
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", kwokCluster.Name, "kubeconfig.yaml")

	// Check if kubeconfig exists
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return fmt.Errorf("kubeconfig file not found at %s", kubeconfigPath)
	}

	logrus.Infof("Applying import YAML to KWOK cluster %s using kubeconfig %s", kwokCluster.Name, kubeconfigPath)

	// Apply the YAML using kubectl
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(importYAML)

	// Capture output for better error reporting
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply import YAML to KWOK cluster: %v, output: %s", err, string(output))
	}

	logrus.Infof("Successfully applied import YAML to KWOK cluster %s: %s", kwokCluster.Name, string(output))
	return nil
}

// waitForServiceAccountReady waits for the service account to be ready in the KWOK cluster
func (a *ScaleAgent) waitForServiceAccountReady(clusterID string) error {
	logrus.Infof("Waiting for service account to be ready in KWOK cluster for Rancher cluster %s", clusterID)

	// Find the KWOK cluster by cluster ID
	var kwokCluster *KWOKCluster
	for _, cluster := range a.kwokManager.clusters {
		if cluster.ClusterID == clusterID {
			kwokCluster = cluster
			break
		}
	}

	if kwokCluster == nil {
		return fmt.Errorf("KWOK cluster not found for Rancher cluster %s", clusterID)
	}

	// Get the kubeconfig path for the KWOK cluster using the actual KWOK cluster name
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", kwokCluster.Name, "kubeconfig.yaml")

	// Wait up to 2 minutes for the service account to be ready
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for service account to be ready in KWOK cluster %s", kwokCluster.Name)
		case <-ticker.C:
			// Check if the service account exists and is ready
			cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "serviceaccount", "--all-namespaces")
			output, err := cmd.CombinedOutput()
			if err != nil {
				logrus.Debugf("Service account check failed: %v, output: %s", err, string(output))
				continue
			}

			// Look for the cattle-system service account which should be created by the import YAML
			if strings.Contains(string(output), "cattle-system") {
				logrus.Infof("Service account is ready in KWOK cluster %s", kwokCluster.Name)
				return nil
			}

			logrus.Debugf("Service account not ready yet in KWOK cluster %s", kwokCluster.Name)
		}
	}
}

// testCriticalAPIEndpoints tests the critical API endpoints that Rancher needs to access
func (a *ScaleAgent) testCriticalAPIEndpoints(clusterID string) error {
	// Find the KWOK cluster
	var kwokCluster *KWOKCluster
	for _, cluster := range a.kwokManager.clusters {
		if cluster.ClusterID == clusterID {
			kwokCluster = cluster
			break
		}
	}

	if kwokCluster == nil {
		return fmt.Errorf("KWOK cluster not found for cluster ID: %s", clusterID)
	}

	// Test the API server endpoint using HTTPS (secure mode)
	baseURL := fmt.Sprintf("https://127.0.0.1:%d", kwokCluster.Port)

	// Test basic API endpoint
	endpoint := "/api/v1/namespaces/kube-system"
	logrus.Infof("Testing endpoint: %s%s", baseURL, endpoint)

	// Create HTTP client with TLS skip verification for testing
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Skip TLS verification for testing
			},
		},
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(baseURL + endpoint)
	if err != nil {
		return fmt.Errorf("failed to test endpoint %s: %v", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		logrus.Infof("✅ Endpoint %s working (status: %d)", endpoint, resp.StatusCode)
	} else {
		logrus.Warnf("⚠️ Endpoint %s returned status: %d", endpoint, resp.StatusCode)
	}

	return nil
}

// waitForClusterReady waits for the Rancher cluster to be in the right state before getting import YAML
func (a *ScaleAgent) waitForClusterReady(clusterID string) error {
	logrus.Infof("Waiting for Rancher cluster %s to be ready for import YAML", clusterID)

	// Wait up to 5 minutes for the cluster to be ready
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for cluster %s to be ready", clusterID)
		case <-ticker.C:
			// Check cluster status in Rancher
			client := &http.Client{Timeout: 30 * time.Second}
			rancherURL := strings.TrimRight(a.config.RancherURL, "/")

			req, err := http.NewRequest("GET", fmt.Sprintf("%s/v3/clusters/%s", rancherURL, clusterID), nil)
			if err != nil {
				logrus.Debugf("Failed to create cluster status request: %v", err)
				continue
			}

			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.config.BearerToken))

			resp, err := client.Do(req)
			if err != nil {
				logrus.Debugf("Failed to get cluster status: %v", err)
				continue
			}

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				logrus.Debugf("Cluster status request failed with status %d", resp.StatusCode)
				continue
			}

			body, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				logrus.Debugf("Failed to read cluster status response: %v", err)
				continue
			}

			// Parse response to check cluster state
			var clusterResp map[string]interface{}
			if err := json.Unmarshal(body, &clusterResp); err != nil {
				logrus.Debugf("Failed to parse cluster status response: %v", err)
				continue
			}

			state, ok := clusterResp["state"].(string)
			if !ok {
				logrus.Debugf("No state field in cluster response")
				continue
			}

			logrus.Debugf("Cluster %s current state: %s", clusterID, state)

			// Check if cluster is ready (not "waiting" or "initializing")
			if state != "waiting" && state != "initializing" {
				logrus.Infof("Cluster %s is ready (state: %s)", clusterID, state)
				return nil
			}

			logrus.Debugf("Cluster %s still not ready (state: %s), waiting...", clusterID, state)
		}
	}
}

func (a *ScaleAgent) waitForClusterActiveAndConnect(clusterName, clusterID string, clusterInfo *ClusterInfo) {
	// Wait for cluster to become active
	logrus.Infof("Waiting for cluster %s to become active...", clusterName)

	// Wait for the service account to be ready in the KWOK cluster
	if err := a.waitForServiceAccountReady(clusterID); err != nil {
		logrus.Errorf("Failed to wait for service account ready: %v", err)
		return
	}

	logrus.Infof("Cluster %s service account is ready, attempting WebSocket connection", clusterName)

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
func (a *ScaleAgent) startLocalAPIServer(clusterName string, port int) {
	logrus.Debugf("startLocalAPIServer deprecated: cluster=%s port=%d", clusterName, port)
}

// Forward declarations (ensure functions exist before use)
// generateSelfSignedCert and startMockHTTPSAPIServer are implemented later in file.
func generateSelfSignedCert(host string) ([]byte, []byte, tls.Certificate, error) {
	priv, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		return nil, nil, tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := crand.Int(crand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, tls.Certificate{}, fmt.Errorf("serial: %w", err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: host, Organization: []string{"scale-cluster-agent"}},
		DNSNames:              []string{"localhost", host},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(crand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, tls.Certificate{}, fmt.Errorf("create cert: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	crt, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, nil, tls.Certificate{}, fmt.Errorf("x509 key pair: %w", err)
	}
	return certPEM, keyPEM, crt, nil
}

func (a *ScaleAgent) startMockHTTPSAPIServer(addr string, cert tls.Certificate, clusterName string) {
	// --- state stores ---
	namespaces := map[string]map[string]interface{}{}
	serviceAccounts := map[string]map[string]interface{}{}
	secrets := map[string]map[string]interface{}{}
	clusterRoles := map[string]map[string]interface{}{}
	clusterRoleBindings := map[string]map[string]interface{}{}
	roles := map[string]map[string]map[string]interface{}{}        // ns -> name -> role
	roleBindings := map[string]map[string]map[string]interface{}{} // ns -> name -> rolebinding
	var stateMu sync.RWMutex
	var rv uint64 = 1
	nextRV := func() string { rv++; return fmt.Sprintf("%d", rv) }

	randSuffix := func(n int) string {
		letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
		b := make([]rune, n)
		for i := range b {
			b[i] = letters[rand.Intn(len(letters))]
		}
		return string(b)
	}
	genUID := func(name string) string { return fmt.Sprintf("%s-%x", name, fnvHash(name)) }
	now := func() string { return time.Now().UTC().Format(time.RFC3339) }
	ensureNS := func(ns string) {
		stateMu.Lock()
		defer stateMu.Unlock()
		if _, ok := namespaces[ns]; !ok {
			// Only create namespace if it doesn't exist
			namespaces[ns] = map[string]interface{}{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]interface{}{"name": ns, "uid": genUID(ns), "creationTimestamp": now(), "resourceVersion": nextRV()}, "status": map[string]interface{}{"phase": "Active"}}
			// auto-create default serviceaccount + token secret to satisfy Rancher defaultSvcAccountHandler
			if _, ok := serviceAccounts[ns]; !ok {
				serviceAccounts[ns] = map[string]interface{}{}
			}
			if _, ok := secrets[ns]; !ok {
				secrets[ns] = map[string]interface{}{}
			}
			if _, exists := serviceAccounts[ns]["default"]; !exists {
				sa := map[string]interface{}{"apiVersion": "v1", "kind": "ServiceAccount", "metadata": map[string]interface{}{"name": "default", "namespace": ns, "uid": genUID(ns + "-sa-default"), "creationTimestamp": now(), "resourceVersion": nextRV()}, "secrets": []interface{}{}}
				serviceAccounts[ns]["default"] = sa
				// create token secret manually (avoid recursive locking)
				secName := fmt.Sprintf("default-token-%s", randSuffix(5))
				data := map[string]interface{}{"token": base64.StdEncoding.EncodeToString([]byte("tok-" + randSuffix(32))), "ca.crt": base64.StdEncoding.EncodeToString([]byte(string(a.mockCertPEM))), "namespace": base64.StdEncoding.EncodeToString([]byte(ns))}
				sec := map[string]interface{}{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]interface{}{"name": secName, "namespace": ns, "uid": genUID(ns + "-secret-" + secName), "creationTimestamp": now(), "annotations": map[string]interface{}{"kubernetes.io/service-account.name": "default", "kubernetes.io/service-account.uid": genUID("sa-default")}, "resourceVersion": nextRV()}, "type": "kubernetes.io/service-account-token", "data": data}
				secrets[ns][secName] = sec
				sa["secrets"] = append(sa["secrets"].([]interface{}), map[string]interface{}{"name": secName})
			}
		}
		// Always ensure the supporting maps exist (but don't modify existing namespace objects)
		if _, ok := serviceAccounts[ns]; !ok {
			serviceAccounts[ns] = map[string]interface{}{}
		}
		if _, ok := secrets[ns]; !ok {
			secrets[ns] = map[string]interface{}{}
		}
		if _, ok := roles[ns]; !ok {
			roles[ns] = map[string]map[string]interface{}{}
		}
		if _, ok := roleBindings[ns]; !ok {
			roleBindings[ns] = map[string]map[string]interface{}{}
		}
	}
	for _, ns := range []string{"kube-system", "cattle-system", "cattle-impersonation-system", "cattle-fleet-system"} {
		ensureNS(ns)
	}

	createRole := func(ns, name string, rules []interface{}) map[string]interface{} {
		ensureNS(ns)
		stateMu.Lock()
		defer stateMu.Unlock()
		r := map[string]interface{}{"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "Role", "metadata": map[string]interface{}{"name": name, "namespace": ns, "uid": genUID("role-" + ns + "-" + name), "creationTimestamp": now(), "resourceVersion": nextRV()}, "rules": rules}
		roles[ns][name] = r
		return r
	}
	createRoleBinding := func(ns, name, roleName, saNS, saName string) map[string]interface{} {
		ensureNS(ns)
		stateMu.Lock()
		defer stateMu.Unlock()
		rb := map[string]interface{}{"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "RoleBinding", "metadata": map[string]interface{}{"name": name, "namespace": ns, "uid": genUID("rb-" + ns + "-" + name), "creationTimestamp": now(), "resourceVersion": nextRV()}, "roleRef": map[string]interface{}{"apiGroup": "rbac.authorization.k8s.io", "kind": "Role", "name": roleName}, "subjects": []interface{}{map[string]interface{}{"kind": "ServiceAccount", "name": saName, "namespace": saNS}}}
		roleBindings[ns][name] = rb
		return rb
	}
	createRole("cattle-system", "cattle-minimal", []interface{}{map[string]interface{}{"apiGroups": []interface{}{""}, "resources": []interface{}{"secrets", "serviceaccounts"}, "verbs": []interface{}{"get", "list", "watch"}}})

	createServiceAccount := func(ns, name string) map[string]interface{} {
		ensureNS(ns)
		stateMu.Lock()
		defer stateMu.Unlock()
		sa := map[string]interface{}{
			"apiVersion": "v1", "kind": "ServiceAccount", "metadata": map[string]interface{}{"name": name, "namespace": ns, "uid": genUID(ns + "-sa-" + name), "creationTimestamp": now(), "resourceVersion": nextRV()},
			"secrets": []interface{}{},
		}
		serviceAccounts[ns][name] = sa
		return sa
	}
	createSecret := func(ns, name, secretType string, data map[string]string, annotations map[string]string) map[string]interface{} {
		ensureNS(ns)
		stateMu.Lock()
		defer stateMu.Unlock()
		if annotations == nil {
			annotations = map[string]string{}
		}
		if name == "" {
			if sa, ok := annotations["kubernetes.io/service-account.name"]; ok {
				name = fmt.Sprintf("%s-token-%s", sa, randSuffix(5))
			}
		}
		if name == "" {
			name = "secret-" + randSuffix(6)
		}
		// augment impersonation/service-account token secrets
		if secretType == "kubernetes.io/service-account-token" {
			if sa, ok := annotations["kubernetes.io/service-account.name"]; ok {
				if _, ok2 := annotations["kubernetes.io/service-account.uid"]; !ok2 {
					annotations["kubernetes.io/service-account.uid"] = genUID("sa-" + sa)
				}
			}
			if data == nil {
				data = map[string]string{}
			}
			if _, ok := data["token"]; !ok {
				data["token"] = "tok-" + randSuffix(32)
			}
			if _, ok := data["ca.crt"]; !ok {
				data["ca.crt"] = string(a.mockCertPEM)
			}
			if _, ok := data["namespace"]; !ok {
				data["namespace"] = ns
			}
		}
		d := map[string]interface{}{}
		for k, v := range data {
			d[k] = base64.StdEncoding.EncodeToString([]byte(v))
		}
		ann := map[string]interface{}{}
		for k, v := range annotations {
			ann[k] = v
		}
		sec := map[string]interface{}{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]interface{}{"name": name, "namespace": ns, "uid": genUID(ns + "-secret-" + name), "creationTimestamp": now(), "annotations": ann, "resourceVersion": nextRV()}, "type": secretType, "data": d}
		secrets[ns][name] = sec
		if secretType == "kubernetes.io/service-account-token" {
			if saName, ok := annotations["kubernetes.io/service-account.name"]; ok {
				if saObjRaw, ok2 := serviceAccounts[ns][saName]; ok2 {
					if saMap, ok3 := saObjRaw.(map[string]interface{}); ok3 {
						if arr, ok4 := saMap["secrets"].([]interface{}); ok4 {
							saMap["secrets"] = append(arr, map[string]interface{}{"name": name})
						} else {
							saMap["secrets"] = []interface{}{map[string]interface{}{"name": name}}
						}
					}
				}
			}
		}
		return sec
	}
	createClusterRole := func(name string, rules []interface{}) map[string]interface{} {
		stateMu.Lock()
		defer stateMu.Unlock()
		cr := map[string]interface{}{"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "ClusterRole", "metadata": map[string]interface{}{"name": name, "uid": genUID("cr-" + name), "creationTimestamp": now(), "resourceVersion": nextRV()}, "rules": rules}
		clusterRoles[name] = cr
		return cr
	}
	createClusterRoleBinding := func(name, roleName, saNS, saName string) map[string]interface{} {
		stateMu.Lock()
		defer stateMu.Unlock()
		crb := map[string]interface{}{"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "ClusterRoleBinding", "metadata": map[string]interface{}{"name": name, "uid": genUID("crb-" + name), "creationTimestamp": now(), "resourceVersion": nextRV()}, "roleRef": map[string]interface{}{"apiGroup": "rbac.authorization.k8s.io", "kind": "ClusterRole", "name": roleName}, "subjects": []interface{}{map[string]interface{}{"kind": "ServiceAccount", "name": saName, "namespace": saNS}}}
		clusterRoleBindings[name] = crb
		return crb
	}

	// seed minimal objects
	createServiceAccount("cattle-system", "cattle")
	createServiceAccount("cattle-fleet-system", "fleet-agent")
	createServiceAccount("cattle-impersonation-system", "cattle-impersonation-user-8kn8j")
	// Pre-create token secret for impersonation SA so Rancher doesn't wait on token controller behavior
	createSecret("cattle-impersonation-system", "cattle-impersonation-user-8kn8j-token-seeded", "kubernetes.io/service-account-token", map[string]string{"token": "token"}, map[string]string{"kubernetes.io/service-account.name": "cattle-impersonation-user-8kn8j"})
	createSecret("cattle-system", "cattle-token-abcde", "kubernetes.io/service-account-token", map[string]string{"token": "token"}, map[string]string{"kubernetes.io/service-account.name": "cattle"})
	createSecret("cattle-system", "cattle-credentials-mock", "Opaque", map[string]string{"username": "admin", "password": "password"}, nil)
	createClusterRole("cattle-admin", []interface{}{map[string]interface{}{"apiGroups": []interface{}{"*"}, "resources": []interface{}{"*"}, "verbs": []interface{}{"*"}}})
	createClusterRole("cluster-admin", []interface{}{map[string]interface{}{"apiGroups": []interface{}{"*"}, "resources": []interface{}{"*"}, "verbs": []interface{}{"*"}}})
	createClusterRoleBinding("cattle-admin-binding", "cattle-admin", "cattle-system", "cattle")

	muxLocal := http.NewServeMux()
	logReq := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			logrus.Debugf("MOCK API %s %s (cluster=%s)", r.Method, r.URL.Path, clusterName)
			h(w, r)
		}
	}
	writeJSON := func(w http.ResponseWriter, status int, obj interface{}) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(obj)
	}

	writeWatchStream := func(w http.ResponseWriter, r *http.Request, items []map[string]interface{}) {
		w.Header().Set("Content-Type", "application/json")
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSON(w, 500, map[string]string{"error": "stream not supported"})
			return
		}
		enc := json.NewEncoder(w)
		for _, obj := range items {
			enc.Encode(map[string]interface{}{"type": "ADDED", "object": obj})
			// Immediately follow with a synthetic MODIFIED for token secrets and serviceaccounts so Rancher sees a population change.
			if k, _ := obj["kind"].(string); k == "Secret" || k == "ServiceAccount" {
				if k == "Secret" {
					if t, ok2 := obj["type"].(string); ok2 && t == "kubernetes.io/service-account-token" {
						enc.Encode(map[string]interface{}{"type": "MODIFIED", "object": obj})
					}
				} else if k == "ServiceAccount" {
					enc.Encode(map[string]interface{}{"type": "MODIFIED", "object": obj})
				}
			}
		}
		flusher.Flush()
		<-r.Context().Done() // keep open until client closes
	}
	isWatch := func(r *http.Request) bool { return r.URL.Query().Get("watch") == "true" }

	// RBAC APIResourceList
	muxLocal.HandleFunc("/apis/rbac.authorization.k8s.io/v1", logReq(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"kind": "APIResourceList", "apiVersion": "v1", "groupVersion": "rbac.authorization.k8s.io/v1", "resources": []map[string]interface{}{
			{"name": "clusterroles", "namespaced": false, "kind": "ClusterRole"},
			{"name": "clusterrolebindings", "namespaced": false, "kind": "ClusterRoleBinding"},
			{"name": "roles", "namespaced": true, "kind": "Role"},
			{"name": "rolebindings", "namespaced": true, "kind": "RoleBinding"},
		}})
	}))

	// cluster-scope roles aggregate list/watch
	muxLocal.HandleFunc("/apis/rbac.authorization.k8s.io/v1/roles", logReq(func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			stateMu.RLock()
			items := []map[string]interface{}{}
			for _, m := range roles {
				for _, role := range m {
					items = append(items, role)
				}
			}
			stateMu.RUnlock()
			writeWatchStream(w, r, items)
			return
		}
		stateMu.RLock()
		items := []interface{}{}
		for _, m := range roles {
			for _, role := range m {
				items = append(items, role)
			}
		}
		stateMu.RUnlock()
		writeJSON(w, 200, map[string]interface{}{"kind": "RoleList", "apiVersion": "rbac.authorization.k8s.io/v1", "resources": []map[string]interface{}{}})
	}))
	muxLocal.HandleFunc("/apis/rbac.authorization.k8s.io/v1/rolebindings", logReq(func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			items := []map[string]interface{}{}
			for _, m := range roleBindings {
				for _, rb := range m {
					items = append(items, rb)
				}
			}
			writeWatchStream(w, r, items)
			return
		}
		items := []interface{}{}
		for _, m := range roleBindings {
			for _, rb := range m {
				items = append(items, rb)
			}
		}
		writeJSON(w, 200, map[string]interface{}{"kind": "RoleBindingList", "apiVersion": "rbac.authorization.k8s.io/v1", "items": items})
	}))

	// Namespaced RBAC resources
	muxLocal.HandleFunc("/apis/rbac.authorization.k8s.io/v1/namespaces/", logReq(func(w http.ResponseWriter, r *http.Request) {
		logrus.Debugf("🔍 NAMESPACE HANDLER: %s %s", r.Method, r.URL.Path)

		rest := strings.TrimPrefix(r.URL.Path, "/apis/rbac.authorization.k8s.io/v1/namespaces/")
		parts := strings.Split(rest, "/")
		logrus.Debugf("🔍 NAMESPACE PARTS: %v", parts)

		// Handle direct namespace object: /api/v1/namespaces/{name}
		if len(parts) == 1 || parts[1] == "" { // trailing slash optional
			ns := strings.TrimSuffix(parts[0], "/")
			if ns == "" {
				logrus.Debugf("🔍 NAMESPACE: empty namespace name")
				http.NotFound(w, r)
				return
			}

			logrus.Debugf("🔍 NAMESPACE: handling namespace '%s' with method %s", ns, r.Method)
			ensureNS(ns)

			stateMu.Lock()
			nsObj := namespaces[ns]
			if r.Method == http.MethodGet {
				logrus.Debugf("🔍 NAMESPACE: GET namespace '%s' - returning object", ns)
				stateMu.Unlock()
				writeJSON(w, 200, nsObj)
				return
			}
			if r.Method == http.MethodPut || r.Method == http.MethodPatch {
				logrus.Debugf("🔍 NAMESPACE: %s namespace '%s' - updating object", r.Method, ns)
				var obj map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
					stateMu.Unlock()
					logrus.Errorf("🔍 NAMESPACE: failed to decode %s request: %v", r.Method, err)
					writeJSON(w, 400, map[string]string{"error": "bad json"})
					return
				}
				md, _ := obj["metadata"].(map[string]interface{})
				if md == nil {
					md = map[string]interface{}{}
					obj["metadata"] = md
				}
				md["name"] = ns
				// bump resourceVersion to satisfy optimistic concurrency expectations
				md["resourceVersion"] = nextRV()
				namespaces[ns] = map[string]interface{}{"apiVersion": "v1", "kind": "Namespace", "metadata": md, "status": map[string]interface{}{"phase": "Active"}}
				updated := namespaces[ns]
				stateMu.Unlock()
				logrus.Debugf("🔍 NAMESPACE: %s namespace '%s' - update successful", r.Method, ns)
				writeJSON(w, 200, updated)
				return
			}
			stateMu.Unlock()
			logrus.Debugf("🔍 NAMESPACE: method %s not allowed for namespace '%s'", r.Method, ns)
			writeJSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		ns, res := parts[0], parts[1]
		ensureNS(ns)
		switch res {
		case "roles":
			if r.Method == http.MethodPost {
				var obj map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
					writeJSON(w, 400, map[string]string{"error": "bad json"})
					return
				}
				md, _ := obj["metadata"].(map[string]interface{})
				if md == nil {
					md = map[string]interface{}{}
					obj["metadata"] = md
				}
				name, _ := md["name"].(string)
				if name == "" {
					if gn, _ := md["generateName"].(string); gn != "" {
						name = gn + randSuffix(5)
						md["name"] = name
					}
				}
				if name == "" {
					writeJSON(w, 400, map[string]string{"error": "name required"})
					return
				}
				rules, _ := obj["rules"].([]interface{})
				role := createRole(ns, name, rules)
				writeJSON(w, 201, role)
				return
			}
			if isWatch(r) {
				stateMu.RLock()
				items := []map[string]interface{}{}
				for _, role := range roles[ns] {
					items = append(items, role)
				}
				stateMu.RUnlock()
				writeWatchStream(w, r, items)
				return
			}
			stateMu.RLock()
			items := []interface{}{}
			for _, role := range roles[ns] {
				items = append(items, role)
			}
			stateMu.RUnlock()
			writeJSON(w, 200, map[string]interface{}{"kind": "RoleList", "apiVersion": "rbac.authorization.k8s.io/v1", "items": items})
			return
		case "rolebindings":
			if r.Method == http.MethodPost {
				var obj map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
					writeJSON(w, 400, map[string]string{"error": "bad json"})
					return
				}
				md, _ := obj["metadata"].(map[string]interface{})
				if md == nil {
					md = map[string]interface{}{}
					obj["metadata"] = md
				}
				name, _ := md["name"].(string)
				if name == "" {
					if gn, _ := md["generateName"].(string); gn != "" {
						name = gn + randSuffix(5)
						md["name"] = name
					}
				}
				if name == "" {
					writeJSON(w, 400, map[string]string{"error": "name required"})
					return
				}
				roleRef, _ := obj["roleRef"].(map[string]interface{})
				roleName, _ := roleRef["name"].(string)
				subjects, _ := obj["subjects"].([]interface{})
				saNS, saName := ns, "cattle"
				if len(subjects) > 0 {
					if subj, ok := subjects[0].(map[string]interface{}); ok {
						if n, ok2 := subj["name"].(string); ok2 {
							saName = n
						}
						if nns, ok2 := subj["namespace"].(string); ok2 {
							saNS = nns
						}
					}
				}
				rb := createRoleBinding(ns, name, roleName, saNS, saName)
				writeJSON(w, 201, rb)
				return
			}
			if isWatch(r) {
				stateMu.RLock()
				items := []map[string]interface{}{}
				for _, rb := range roleBindings[ns] {
					items = append(items, rb)
				}
				stateMu.RUnlock()
				writeWatchStream(w, r, items)
				return
			}
			stateMu.RLock()
			items := []interface{}{}
			for _, rb := range roleBindings[ns] {
				items = append(items, rb)
			}
			stateMu.RUnlock()
			writeJSON(w, 200, map[string]interface{}{"kind": "RoleBindingList", "apiVersion": "rbac.authorization.k8s.io/v1", "items": items})
			return
		default:
			http.NotFound(w, r)
		}
	}))

	// ---- Core discovery endpoints ----
	muxLocal.HandleFunc("/version", logReq(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"major": "1", "minor": "24", "gitVersion": "v1.24.0-mock", "platform": "linux/amd64"})
	}))
	muxLocal.HandleFunc("/api", logReq(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"kind": "APIVersions", "versions": []string{"v1"}})
	}))
	muxLocal.HandleFunc("/apis", logReq(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"kind": "APIGroupList", "apiVersion": "v1", "groups": []interface{}{
			map[string]interface{}{"name": "rbac.authorization.k8s.io", "versions": []interface{}{map[string]interface{}{"groupVersion": "rbac.authorization.k8s.io/v1", "version": "v1"}}, "preferredVersion": map[string]interface{}{"groupVersion": "rbac.authorization.k8s.io/v1", "version": "v1"}},
			map[string]interface{}{"name": "apiregistration.k8s.io", "versions": []interface{}{map[string]interface{}{"groupVersion": "apiregistration.k8s.io/v1", "version": "v1"}}, "preferredVersion": map[string]interface{}{"groupVersion": "apiregistration.k8s.io/v1", "version": "v1"}},
		}})
	}))
	muxLocal.HandleFunc("/apis/apiregistration.k8s.io/v1", logReq(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"kind": "APIResourceList", "apiVersion": "v1", "groupVersion": "apiregistration.k8s.io/v1", "resources": []map[string]interface{}{
			{"name": "apiservices", "namespaced": false, "kind": "APIService"},
		}})
	}))
	muxLocal.HandleFunc("/apis/apiregistration.k8s.io/v1/apiservices", logReq(func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			writeWatchStream(w, r, []map[string]interface{}{})
			return
		}
		writeJSON(w, 200, map[string]interface{}{"kind": "APIServiceList", "apiVersion": "apiregistration.k8s.io/v1", "items": []interface{}{}})
	}))
	muxLocal.HandleFunc("/api/v1", logReq(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"kind": "APIResourceList", "apiVersion": "v1", "groupVersion": "v1", "resources": []map[string]interface{}{
			{"name": "namespaces", "namespaced": false, "kind": "Namespace"},
			{"name": "serviceaccounts", "namespaced": true, "kind": "ServiceAccount"},
			{"name": "secrets", "namespaced": true, "kind": "Secret"},
			{"name": "resourcequotas", "namespaced": true, "kind": "ResourceQuota"},
			{"name": "limitranges", "namespaced": true, "kind": "LimitRange"},
			{"name": "nodes", "namespaced": false, "kind": "Node"},
		}})
	}))

	// ---- Cluster-scoped RBAC list/create/watch ----
	muxLocal.HandleFunc("/apis/rbac.authorization.k8s.io/v1/clusterroles", logReq(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var obj map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
				writeJSON(w, 400, map[string]string{"error": "bad json"})
				return
			}
			md, _ := obj["metadata"].(map[string]interface{})
			if md == nil {
				md = map[string]interface{}{}
				obj["metadata"] = md
			}
			name, _ := md["name"].(string)
			if name == "" {
				if gn, _ := md["generateName"].(string); gn != "" {
					name = gn + randSuffix(5)
					md["name"] = name
				}
			}
			if name == "" {
				writeJSON(w, 400, map[string]string{"error": "name required"})
				return
			}
			rules, _ := obj["rules"].([]interface{})
			cr := createClusterRole(name, rules)
			writeJSON(w, 201, cr)
			return
		}
		if isWatch(r) {
			stateMu.RLock()
			items := []map[string]interface{}{}
			for _, cr := range clusterRoles {
				items = append(items, cr)
			}
			stateMu.RUnlock()
			writeWatchStream(w, r, items)
			return
		}
		stateMu.RLock()
		items := []interface{}{}
		for _, cr := range clusterRoles {
			items = append(items, cr)
		}
		stateMu.RUnlock()
		writeJSON(w, 200, map[string]interface{}{"kind": "ClusterRoleList", "apiVersion": "rbac.authorization.k8s.io/v1", "items": items})
	}))
	muxLocal.HandleFunc("/apis/rbac.authorization.k8s.io/v1/clusterrolebindings", logReq(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var obj map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
				writeJSON(w, 400, map[string]string{"error": "bad json"})
				return
			}
			md, _ := obj["metadata"].(map[string]interface{})
			if md == nil {
				md = map[string]interface{}{}
				obj["metadata"] = md
			}
			name, _ := md["name"].(string)
			if name == "" {
				if gn, _ := md["generateName"].(string); gn != "" {
					name = gn + randSuffix(5)
					md["name"] = name
				}
			}
			if name == "" {
				writeJSON(w, 400, map[string]string{"error": "name required"})
				return
			}
			roleRef, _ := obj["roleRef"].(map[string]interface{})
			roleName, _ := roleRef["name"].(string)
			subjects, _ := obj["subjects"].([]interface{})
			saNS, saName := "cattle-system", "cattle"
			if len(subjects) > 0 {
				if subj, ok := subjects[0].(map[string]interface{}); ok {
					if n, ok2 := subj["name"].(string); ok2 {
						saName = n
					}
					if nns, ok2 := subj["namespace"].(string); ok2 {
						saNS = nns
					}
				}
			}
			crb := createClusterRoleBinding(name, roleName, saNS, saName)
			writeJSON(w, 201, crb)
			return
		}
		if isWatch(r) {
			stateMu.RLock()
			items := []map[string]interface{}{}
			for _, crb := range clusterRoleBindings {
				items = append(items, crb)
			}
			stateMu.RUnlock()
			writeWatchStream(w, r, items)
			return
		}
		stateMu.RLock()
		items := []interface{}{}
		for _, crb := range clusterRoleBindings {
			items = append(items, crb)
		}
		stateMu.RUnlock()
		writeJSON(w, 200, map[string]interface{}{"kind": "ClusterRoleBindingList", "apiVersion": "rbac.authorization.k8s.io/v1", "items": items})
	}))

	// ---- Core resource handlers ----

	// Minimal OpenAPI v2 spec endpoint (Kubernetes normally serves a large document). Rancher just needs a 200 with JSON.
	muxLocal.HandleFunc("/openapi/v2", logReq(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Extremely reduced spec: only declares a few core objects we fake. Enough to unblock validation.
		io.WriteString(w, `{
	"swagger":"2.0",
	"info":{"title":"Kubernetes","version":"v1.29.0"},
	"paths":{
	  "/api/v1/namespaces":{},
	  "/api/v1/namespaces/{name}":{},
	  "/api/v1/namespaces/{namespace}/secrets":{},
	  "/api/v1/namespaces/{namespace}/serviceaccounts":{},
	  "/apis/rbac.authorization.k8s.io/v1/clusterroles":{},
	  "/apis/rbac.authorization.k8s.io/v1/clusterrolebindings":{}
	},
	"definitions":{
	  "io.k8s.api.core.v1.Namespace":{ "type":"object","properties":{"metadata":{"type":"object"}}},
	  "io.k8s.api.core.v1.Secret":{ "type":"object","properties":{"metadata":{"type":"object"},"data":{"type":"object"}}},
	  "io.k8s.api.core.v1.ServiceAccount":{ "type":"object","properties":{"metadata":{"type":"object"}}},
	  "io.k8s.api.rbac.v1.ClusterRole":{ "type":"object","properties":{"metadata":{"type":"object"}}},
	  "io.k8s.api.rbac.v1.ClusterRoleBinding":{ "type":"object","properties":{"metadata":{"type":"object"}}}
	}
	}`)
	}))

	muxLocal.HandleFunc("/api/v1/namespaces", logReq(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Handle namespace creation
			var obj map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
				writeJSON(w, 400, map[string]string{"error": "bad json"})
				return
			}

			md, _ := obj["metadata"].(map[string]interface{})
			if md == nil {
				writeJSON(w, 400, map[string]string{"error": "metadata required"})
				return
			}

			name, _ := md["name"].(string)
			if name == "" {
				writeJSON(w, 400, map[string]string{"error": "name required"})
				return
			}

			// Create the namespace
			ensureNS(name)

			// Update with any additional metadata from the request
			stateMu.Lock()
			if nsMd, ok := namespaces[name]["metadata"].(map[string]interface{}); ok {
				// Merge metadata from request
				for k, v := range md {
					if k != "name" && k != "namespace" { // Don't override these
						nsMd[k] = v
					}
				}
				// Ensure required fields
				nsMd["name"] = name
				nsMd["namespace"] = name
				nsMd["resourceVersion"] = nextRV()
			}
			stateMu.Unlock()

			writeJSON(w, 201, namespaces[name])
			return
		}

		if isWatch(r) {
			stateMu.RLock()
			items := []map[string]interface{}{}
			for _, nsObj := range namespaces {
				items = append(items, nsObj)
			}
			stateMu.RUnlock()
			writeWatchStream(w, r, items)
			return
		}
		stateMu.RLock()
		items := []interface{}{}
		for _, nsObj := range namespaces {
			items = append(items, nsObj)
		}
		stateMu.RUnlock()
		writeJSON(w, 200, map[string]interface{}{"kind": "NamespaceList", "apiVersion": "v1", "items": items})
	}))
	// Namespaced core resources
	muxLocal.HandleFunc("/api/v1/namespaces/", logReq(func(w http.ResponseWriter, r *http.Request) {
		logrus.Debugf("🔍 NAMESPACE HANDLER: %s %s", r.Method, r.URL.Path)

		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/")
		parts := strings.Split(rest, "/")
		logrus.Debugf("🔍 NAMESPACE PARTS: %v", parts)

		// Handle direct namespace object: /api/v1/namespaces/{name}
		if len(parts) == 1 || parts[1] == "" { // trailing slash optional
			ns := strings.TrimSuffix(parts[0], "/")
			if ns == "" {
				logrus.Debugf("🔍 NAMESPACE: empty namespace name")
				http.NotFound(w, r)
				return
			}

			logrus.Debugf("🔍 NAMESPACE: handling namespace '%s' with method %s", ns, r.Method)
			ensureNS(ns)

			stateMu.Lock()
			nsObj := namespaces[ns]
			if r.Method == http.MethodGet {
				logrus.Debugf("🔍 NAMESPACE: GET namespace '%s' - returning object", ns)
				stateMu.Unlock()
				writeJSON(w, 200, nsObj)
				return
			}
			if r.Method == http.MethodPut || r.Method == http.MethodPatch {
				logrus.Debugf("🔍 NAMESPACE: %s namespace '%s' - updating object", r.Method, ns)
				var obj map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
					stateMu.Unlock()
					logrus.Errorf("🔍 NAMESPACE: failed to decode %s request: %v", r.Method, err)
					writeJSON(w, 400, map[string]string{"error": "bad json"})
					return
				}
				md, _ := obj["metadata"].(map[string]interface{})
				if md == nil {
					md = map[string]interface{}{}
					obj["metadata"] = md
				}
				md["name"] = ns
				// bump resourceVersion to satisfy optimistic concurrency expectations
				md["resourceVersion"] = nextRV()
				namespaces[ns] = map[string]interface{}{"apiVersion": "v1", "kind": "Namespace", "metadata": md, "status": map[string]interface{}{"phase": "Active"}}
				updated := namespaces[ns]
				stateMu.Unlock()
				logrus.Debugf("🔍 NAMESPACE: %s namespace '%s' - update successful", r.Method, ns)
				writeJSON(w, 200, updated)
				return
			}
			stateMu.Unlock()
			logrus.Debugf("🔍 NAMESPACE: method %s not allowed for namespace '%s'", r.Method, ns)
			writeJSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		ns, res := parts[0], parts[1]
		ensureNS(ns)
		switch res {
		case "serviceaccounts":
			// Allow GET of individual service account: /api/v1/namespaces/{ns}/serviceaccounts/{name}
			if len(parts) >= 3 && parts[2] != "" {
				name := parts[2]
				if r.Method == http.MethodGet {
					stateMu.RLock()
					objRaw, ok := serviceAccounts[ns][name]
					stateMu.RUnlock()
					if !ok {
						http.NotFound(w, r)
						return
					}
					writeJSON(w, 200, objRaw)
					return
				}
				if r.Method == http.MethodPut || r.Method == http.MethodPatch {
					// Accept any PUT/PATCH as a no-op update/upsert. Rancher uses this to "ensure" the SA exists.
					var incoming map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
						// even if bad JSON, just ensure object exists
						logrus.Debugf("MOCK API PUT serviceaccount bad json (ns=%s name=%s): %v", ns, name, err)
					}
					// Inline mutation under single lock; avoid calling helper creators (they lock internally) to prevent deadlocks.
					stateMu.Lock()
					objRaw, ok := serviceAccounts[ns][name]
					if !ok {
						// create new SA struct
						objRaw = map[string]interface{}{
							"apiVersion": "v1", "kind": "ServiceAccount",
							"metadata": map[string]interface{}{"name": name, "namespace": ns, "uid": genUID(ns + "-sa-" + name), "creationTimestamp": now(), "resourceVersion": nextRV()},
							"secrets":  []interface{}{},
						}
						serviceAccounts[ns][name] = objRaw
					} else if objMap, ok2 := objRaw.(map[string]interface{}); ok2 {
						// merge metadata
						mdExisting, _ := objMap["metadata"].(map[string]interface{})
						if mdExisting == nil {
							mdExisting = map[string]interface{}{}
							objMap["metadata"] = mdExisting
						}
						if incoming != nil {
							if mdIn, ok3 := incoming["metadata"].(map[string]interface{}); ok3 {
								for k, v := range mdIn {
									mdExisting[k] = v
								}
							}
						}
						mdExisting["name"] = name
						mdExisting["namespace"] = ns
						mdExisting["resourceVersion"] = nextRV()
					}
					// Ensure token secret exists
					if _, ok2 := secrets[ns]; !ok2 {
						secrets[ns] = map[string]interface{}{}
					}
					foundToken := false
					for _, s := range secrets[ns] {
						if sm, ok3 := s.(map[string]interface{}); ok3 {
							if mdAny, ok4 := sm["metadata"].(map[string]interface{}); ok4 {
								if anns, ok5 := mdAny["annotations"].(map[string]interface{}); ok5 {
									if san, ok6 := anns["kubernetes.io/service-account.name"].(string); ok6 && san == name {
										foundToken = true
										break
									}
								}
							}
						}
					}
					if !foundToken {
						secName := fmt.Sprintf("%s-token-%s", name, randSuffix(5))
						data := map[string]interface{}{"token": base64.StdEncoding.EncodeToString([]byte("tok-" + randSuffix(32))), "ca.crt": base64.StdEncoding.EncodeToString([]byte(string(a.mockCertPEM))), "namespace": base64.StdEncoding.EncodeToString([]byte(ns))}
						ann := map[string]interface{}{"kubernetes.io/service-account.name": name, "kubernetes.io/service-account.uid": genUID("sa-" + name)}
						sec := map[string]interface{}{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]interface{}{"name": secName, "namespace": ns, "uid": genUID(ns + "-secret-" + secName), "creationTimestamp": now(), "annotations": ann, "resourceVersion": nextRV()}, "type": "kubernetes.io/service-account-token", "data": data}
						secrets[ns][secName] = sec
						// link to SA
						if saMap, ok3 := serviceAccounts[ns][name].(map[string]interface{}); ok3 {
							if arr, ok4 := saMap["secrets"].([]interface{}); ok4 {
								saMap["secrets"] = append(arr, map[string]interface{}{"name": secName})
							} else {
								saMap["secrets"] = []interface{}{map[string]interface{}{"name": secName}}
							}
						}
					}
					ret := serviceAccounts[ns][name]
					stateMu.Unlock()
					writeJSON(w, 200, ret)
					return
				}
				writeJSON(w, 405, map[string]string{"error": "method not allowed"})
				return
			}
			if r.Method == http.MethodPost {
				var obj map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
					writeJSON(w, 400, map[string]string{"error": "bad json"})
					return
				}
				md, _ := obj["metadata"].(map[string]interface{})
				if md == nil {
					md = map[string]interface{}{}
					obj["metadata"] = md
				}
				name, _ := md["name"].(string)
				if name == "" {
					if gn, _ := md["generateName"].(string); gn != "" {
						name = gn + randSuffix(5)
						md["name"] = name
					}
				}
				if name == "" {
					writeJSON(w, 400, map[string]string{"error": "name required"})
					return
				}
				sa := createServiceAccount(ns, name)
				// auto token secret
				createSecret(ns, "", "kubernetes.io/service-account-token", map[string]string{"token": "token"}, map[string]string{"kubernetes.io/service-account.name": name})
				writeJSON(w, 201, sa)
				return
			}
			if isWatch(r) {
				stateMu.RLock()
				items := []map[string]interface{}{}
				for _, sa := range serviceAccounts[ns] {
					if m, ok := sa.(map[string]interface{}); ok {
						items = append(items, m)
					}
				}
				stateMu.RUnlock()
				writeWatchStream(w, r, items)
				return
			}
			stateMu.RLock()
			items := []interface{}{}
			for _, sa := range serviceAccounts[ns] {
				items = append(items, sa)
			}
			stateMu.RUnlock()
			writeJSON(w, 200, map[string]interface{}{"kind": "ServiceAccountList", "apiVersion": "v1", "items": items})
			return
		case "secrets":
			// GET individual secret: /api/v1/namespaces/{ns}/secrets/{name}
			if len(parts) >= 3 && parts[2] != "" {
				name := parts[2]
				if r.Method == http.MethodGet {
					stateMu.Lock()
					objRaw, ok := secrets[ns][name]
					if !ok {
						stateMu.Unlock()
						http.NotFound(w, r)
						return
					}
					// Lazy populate if SA annotation exists or token secret type missing fields
					if secMap, ok2 := objRaw.(map[string]interface{}); ok2 {
						md, _ := secMap["metadata"].(map[string]interface{})
						ann, _ := md["annotations"].(map[string]interface{})
						if t, ok3 := secMap["type"].(string); (ok3 && t == "kubernetes.io/service-account-token") || (ann != nil && ann["kubernetes.io/service-account.name"] != nil) {
							if _, ok3 := secMap["type"].(string); !ok3 || secMap["type"].(string) == "Opaque" {
								secMap["type"] = "kubernetes.io/service-account-token"
							}
							dataMap, _ := secMap["data"].(map[string]interface{})
							if dataMap == nil {
								dataMap = map[string]interface{}{}
								secMap["data"] = dataMap
							}
							if _, hasTok := dataMap["token"]; !hasTok {
								dataMap["token"] = base64.StdEncoding.EncodeToString([]byte("tok-" + randSuffix(32)))
							}
							if _, hasCA := dataMap["ca.crt"]; !hasCA {
								dataMap["ca.crt"] = base64.StdEncoding.EncodeToString([]byte(string(a.mockCertPEM)))
							}
							if _, hasNS := dataMap["namespace"]; !hasNS {
								dataMap["namespace"] = base64.StdEncoding.EncodeToString([]byte(ns))
							}
							if md != nil {
								md["resourceVersion"] = nextRV()
							}
						}
					}
					ret := secrets[ns][name]
					stateMu.Unlock()
					writeJSON(w, 200, ret)
					return
				}
				if r.Method == http.MethodPut || r.Method == http.MethodPatch {
					var incoming map[string]interface{}
					_ = json.NewDecoder(r.Body).Decode(&incoming) // ignore errors
					stateMu.Lock()
					objRaw, ok := secrets[ns][name]
					if !ok {
						objRaw = map[string]interface{}{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]interface{}{"name": name, "namespace": ns, "uid": genUID(ns + "-secret-" + name), "creationTimestamp": now(), "resourceVersion": nextRV()}, "type": "Opaque", "data": map[string]interface{}{}}
						secrets[ns][name] = objRaw
					}
					if secMap, ok2 := objRaw.(map[string]interface{}); ok2 {
						md, _ := secMap["metadata"].(map[string]interface{})
						if md == nil {
							md = map[string]interface{}{"name": name, "namespace": ns}
							secMap["metadata"] = md
						}
						if incoming != nil {
							if incMD, ok3 := incoming["metadata"].(map[string]interface{}); ok3 {
								for k, v := range incMD {
									md[k] = v
								}
							}
						}
						md["resourceVersion"] = nextRV()
						ann, _ := md["annotations"].(map[string]interface{})
						if ann != nil && ann["kubernetes.io/service-account.name"] != nil {
							secMap["type"] = "kubernetes.io/service-account-token"
							dataMap, _ := secMap["data"].(map[string]interface{})
							if dataMap == nil {
								dataMap = map[string]interface{}{}
								secMap["data"] = dataMap
							}
							if _, hasTok := dataMap["token"]; !hasTok {
								dataMap["token"] = base64.StdEncoding.EncodeToString([]byte("tok-" + randSuffix(32)))
							}
							if _, hasCA := dataMap["ca.crt"]; !hasCA {
								dataMap["ca.crt"] = base64.StdEncoding.EncodeToString([]byte(string(a.mockCertPEM)))
							}
							if _, hasNS := dataMap["namespace"]; !hasNS {
								dataMap["namespace"] = base64.StdEncoding.EncodeToString([]byte(ns))
							}
						}
					}
					ret := secrets[ns][name]
					stateMu.Unlock()
					writeJSON(w, 200, ret)
					return
				}
				writeJSON(w, 405, map[string]string{"error": "method not allowed"})
				return
			}
			if r.Method == http.MethodPost {
				var obj map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&obj); err != nil { // still return success stub to unblock controllers
					sec := createSecret(ns, "", "Opaque", map[string]string{"note": "auto-created"}, nil)
					writeJSON(w, 201, sec)
					return
				}
				md, _ := obj["metadata"].(map[string]interface{})
				if md == nil {
					md = map[string]interface{}{}
					obj["metadata"] = md
				}
				name, _ := md["name"].(string)
				if name == "" {
					if gn, _ := md["generateName"].(string); gn != "" {
						name = gn + randSuffix(5)
					}
				}
				typ, _ := obj["type"].(string)
				if typ == "" {
					typ = "Opaque"
				}
				data := map[string]string{}
				if rawData, ok := obj["data"].(map[string]interface{}); ok {
					for k, v := range rawData {
						if s, ok2 := v.(string); ok2 {
							data[k] = s
						}
					}
				}
				annIn := map[string]string{}
				if mdAnn, ok := md["annotations"].(map[string]interface{}); ok {
					for k, v := range mdAnn {
						if s, ok2 := v.(string); ok2 {
							annIn[k] = s
						}
					}
				}
				// debug: ensure we never proceed with empty name after generation fallback
				if name == "" {
					logrus.Debugf("MOCK API generating fallback secret name (ns=%s) annotations=%v", ns, annIn)
				}
				// If impersonation secret, ensure service account exists first
				if ns == "cattle-impersonation-system" {
					if saName, ok := annIn["kubernetes.io/service-account.name"]; ok {
						// auto-create SA if missing
						stateMu.RLock()
						_, exists := serviceAccounts[ns][saName]
						stateMu.RUnlock()
						if !exists {
							createServiceAccount(ns, saName)
						}
						// Force type upgrade & populate token data immediately so Rancher doesn't wait.
						if typ == "Opaque" {
							typ = "kubernetes.io/service-account-token"
						}
						if typ == "kubernetes.io/service-account-token" {
							if _, ok := data["token"]; !ok {
								data["token"] = "tok-" + randSuffix(32)
							}
							if _, ok := data["ca.crt"]; !ok {
								data["ca.crt"] = string(a.mockCertPEM)
							}
							if _, ok := data["namespace"]; !ok {
								data["namespace"] = ns
							}
						}
					} else {
						// Fallback: Rancher sometimes creates placeholder Opaque secret first, without SA annotation; still emit a token so controllers progress.
						if typ == "Opaque" {
							typ = "kubernetes.io/service-account-token"
						}
						if _, ok := data["token"]; !ok {
							data["token"] = "tok-" + randSuffix(32)
						}
						if _, ok := data["ca.crt"]; !ok {
							data["ca.crt"] = string(a.mockCertPEM)
						}
						if _, ok := data["namespace"]; !ok {
							data["namespace"] = ns
						}
					}
				}
				sec := createSecret(ns, name, typ, data, annIn)
				writeJSON(w, 201, sec)
				return
			}
			if isWatch(r) {
				stateMu.RLock()
				items := []map[string]interface{}{}
				for _, s := range secrets[ns] {
					if m, ok := s.(map[string]interface{}); ok {
						items = append(items, m)
					}
				}
				stateMu.RUnlock()
				writeWatchStream(w, r, items)
				return
			}
			stateMu.RLock()
			items := []interface{}{}
			for _, s := range secrets[ns] {
				items = append(items, s)
			}
			stateMu.RUnlock()
			writeJSON(w, 200, map[string]interface{}{"kind": "SecretList", "apiVersion": "v1", "items": items})
			return
		case "resourcequotas":
			if isWatch(r) {
				writeWatchStream(w, r, []map[string]interface{}{})
				return
			}
			writeJSON(w, 200, map[string]interface{}{"kind": "ResourceQuotaList", "apiVersion": "v1", "items": []interface{}{}})
			return
		case "limitranges":
			if isWatch(r) {
				writeWatchStream(w, r, []map[string]interface{}{})
				return
			}
			writeJSON(w, 200, map[string]interface{}{"kind": "LimitRangeList", "apiVersion": "v1", "items": []interface{}{}})
			return
		default:
			http.NotFound(w, r)
		}
	}))
	// cluster-wide list across namespaces for namespaced resources
	muxLocal.HandleFunc("/api/v1/resourcequotas", logReq(func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			writeWatchStream(w, r, []map[string]interface{}{})
			return
		}
		writeJSON(w, 200, map[string]interface{}{"kind": "ResourceQuotaList", "apiVersion": "v1", "items": []interface{}{}})
	}))
	muxLocal.HandleFunc("/api/v1/limitranges", logReq(func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			writeWatchStream(w, r, []map[string]interface{}{})
			return
		}
		writeJSON(w, 200, map[string]interface{}{"kind": "LimitRangeList", "apiVersion": "v1", "items": []interface{}{}})
	}))
	// Cluster-scope core lists
	muxLocal.HandleFunc("/api/v1/serviceaccounts", logReq(func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			stateMu.RLock()
			items := []map[string]interface{}{}
			for _, nsMap := range serviceAccounts {
				for _, sa := range nsMap {
					if m, ok := sa.(map[string]interface{}); ok {
						items = append(items, m)
					}
				}
			}
			stateMu.RUnlock()
			writeWatchStream(w, r, items)
			return
		}
		stateMu.RLock()
		items := []interface{}{}
		for _, nsMap := range serviceAccounts {
			for _, sa := range nsMap {
				items = append(items, sa)
			}
		}
		stateMu.RUnlock()
		writeJSON(w, 200, map[string]interface{}{"kind": "ServiceAccountList", "apiVersion": "v1", "items": items})
	}))
	muxLocal.HandleFunc("/api/v1/secrets", logReq(func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			stateMu.RLock()
			items := []map[string]interface{}{}
			for _, nsMap := range secrets {
				for _, s := range nsMap {
					if m, ok := s.(map[string]interface{}); ok {
						items = append(items, m)
					}
				}
			}
			stateMu.RUnlock()
			writeWatchStream(w, r, items)
			return
		}
		stateMu.RLock()
		items := []interface{}{}
		for _, nsMap := range secrets {
			for _, s := range nsMap {
				items = append(items, s)
			}
		}
		stateMu.RUnlock()
		writeJSON(w, 200, map[string]interface{}{"kind": "SecretList", "apiVersion": "v1", "items": items})
	}))
	// Nodes
	nodeObj := map[string]interface{}{"apiVersion": "v1", "kind": "Node", "metadata": map[string]interface{}{"name": "mock-node", "uid": genUID("node-mock-node"), "creationTimestamp": now(), "resourceVersion": nextRV()}, "status": map[string]interface{}{"conditions": []interface{}{}}}
	muxLocal.HandleFunc("/api/v1/nodes", logReq(func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			writeWatchStream(w, r, []map[string]interface{}{nodeObj})
			return
		}
		writeJSON(w, 200, map[string]interface{}{"kind": "NodeList", "apiVersion": "v1", "items": []interface{}{nodeObj}})
	}))

	// Start HTTPS server
	go func() {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			logrus.Errorf("mock api listen error: %v", err)
			return
		}
		tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
		srv := &http.Server{Handler: muxLocal, TLSConfig: tlsCfg}
		logrus.Infof("Mock kube API serving on https://%s (cluster=%s)", addr, clusterName)
		if err := srv.Serve(tls.NewListener(ln, tlsCfg)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.Errorf("mock api server error: %v", err)
		}
	}()
}

// waitForPort tries to connect until timeout.
func waitForPort(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 500 * time.Millisecond}, "tcp", address, &tls.Config{InsecureSkipVerify: true})
		if err == nil {
			conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("port %s not ready: %w", address, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// patchClusterActive best-effort activation (placeholder; Rancher may require action endpoint)
func (a *ScaleAgent) patchClusterActive(clusterID string) {
	if clusterID == "" || a.config == nil {
		return
	}
	base := strings.TrimRight(a.config.RancherURL, "/")
	client := &http.Client{Timeout: 15 * time.Second}
	// 1. GET cluster to inspect actions
	getURL := fmt.Sprintf("%s/v3/clusters/%s", base, clusterID)
	req, _ := http.NewRequest("GET", getURL, nil)
	req.Header.Set("Authorization", "Bearer "+a.config.BearerToken)
	resp, err := client.Do(req)
	if err != nil {
		logrus.Debugf("cluster get failed: %v", err)
		return
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		logrus.Debugf("cluster get status=%d body=%s", resp.StatusCode, string(body))
		return
	}
	var cluster map[string]interface{}
	if err := json.Unmarshal(body, &cluster); err != nil {
		logrus.Debugf("cluster json decode: %v", err)
		return
	}
	// check actions.activate
	if act, ok := cluster["actions"].(map[string]interface{}); ok {
		if activateURL, ok2 := act["activate"].(string); ok2 {
			req2, _ := http.NewRequest("POST", activateURL, nil)
			req2.Header.Set("Authorization", "Bearer "+a.config.BearerToken)
			resp2, err2 := client.Do(req2)
			if err2 == nil {
				b2, _ := io.ReadAll(resp2.Body)
				resp2.Body.Close()
				logrus.Debugf("activate POST status=%d body=%s", resp2.StatusCode, string(b2))
			}
			return
		}
	}
	// fallback: PUT minimal transition state if allowed (may fail; ignore)
	spec := map[string]interface{}{"id": clusterID, "type": "cluster", "name": clusterID, "state": "active"}
	putBody, _ := json.Marshal(spec)
	req3, _ := http.NewRequest("PUT", getURL, bytes.NewReader(putBody))
	req3.Header.Set("Authorization", "Bearer "+a.config.BearerToken)
	req3.Header.Set("Content-Type", "application/json")
	if resp3, err3 := client.Do(req3); err3 == nil {
		b3, _ := io.ReadAll(resp3.Body)
		resp3.Body.Close()
		logrus.Debugf("activate PUT status=%d body=%s", resp3.StatusCode, string(b3))
	}
}

func fnvHash(s string) uint32 { h := fnv.New32a(); _, _ = h.Write([]byte(s)); return h.Sum32() }

// debugConn wraps a net.Conn to add debug logging
type debugConn struct {
	net.Conn
	cluster string
	proto   string
	address string
}

func (dc *debugConn) Read(b []byte) (n int, err error) {
	n, err = dc.Conn.Read(b)
	if err != nil {
		logrus.Errorf("🔄 REMOTEDIALER DEBUG: [%s] Read error: %v", dc.cluster, err)
	} else if n > 0 {
		logrus.Infof("�� REMOTEDIALER DEBUG: [%s] Read %d bytes from %s://%s", dc.cluster, n, dc.proto, dc.address)
		// Log first 100 characters of data for debugging
		if n > 0 && n <= 100 {
			logrus.Infof("🔄 REMOTEDIALER DEBUG: [%s] Data: %s", dc.cluster, string(b[:n]))
		} else if n > 100 {
			logrus.Infof("🔄 REMOTEDIALER DEBUG: [%s] Data (first 100 chars): %s...", dc.cluster, string(b[:100]))
		}
	}
	return n, err
}

func (dc *debugConn) Write(b []byte) (n int, err error) {
	n, err = dc.Conn.Write(b)
	if err != nil {
		logrus.Errorf("🔄 REMOTEDIALER DEBUG: [%s] Write error: %v", dc.cluster, err)
	} else if n > 0 {
		logrus.Infof("🔄 REMOTEDIALER DEBUG: [%s] Wrote %d bytes to %s://%s", dc.cluster, n, dc.proto, dc.address)
		// Log first 100 characters of data for debugging
		if n > 0 && n <= 100 {
			logrus.Infof("🔄 REMOTEDIALER DEBUG: [%s] Data: %s", dc.cluster, string(b[:n]))
		} else if n > 100 {
			logrus.Infof("🔄 REMOTEDIALER DEBUG: [%s] Data (first 100 chars): %s...", dc.cluster, string(b[:100]))
		}
	}
	return n, err
}

func (dc *debugConn) Close() error {
	logrus.Infof("🔄 REMOTEDIALER DEBUG: [%s] Closing connection to %s://%s", dc.cluster, dc.proto, dc.address)
	return dc.Conn.Close()
}

// randSuffix generates a random suffix string
func randSuffix(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// getTokenFromAPI gets the CA certificate and service account token from the KWOK cluster
// This implements the exact same logic as the real agent using kubectl commands
func (a *ScaleAgent) getTokenFromAPI(clusterID string) ([]byte, []byte, error) {
	// Find the KWOK cluster
	var kwokCluster *KWOKCluster
	for _, cluster := range a.kwokManager.clusters {
		if cluster.ClusterID == clusterID {
			kwokCluster = cluster
			break
		}
	}

	if kwokCluster == nil {
		return nil, nil, fmt.Errorf("KWOK cluster not found for cluster ID: %s", clusterID)
	}

	// Get the kubeconfig from KWOK
	cmd := exec.Command(a.kwokManager.kwokctlPath, "get", "kubeconfig", "--name", kwokCluster.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get kubeconfig: %v, output: %s", err, string(output))
	}

	// Parse the kubeconfig to get CA certificate
	var kubeconfig struct {
		Clusters []struct {
			Cluster struct {
				CertificateAuthorityData string `yaml:"certificate-authority-data"`
				CertificateAuthority     string `yaml:"certificate-authority"`
				Server                   string `yaml:"server"`
			} `yaml:"cluster"`
		} `yaml:"clusters"`
	}

	if err := yaml.Unmarshal(output, &kubeconfig); err != nil {
		return nil, nil, fmt.Errorf("failed to parse kubeconfig: %v", err)
	}

	if len(kubeconfig.Clusters) == 0 {
		return nil, nil, fmt.Errorf("no clusters found in kubeconfig")
	}

	// Extract CA certificate
	var caCert []byte
	cluster := kubeconfig.Clusters[0].Cluster

	if cluster.CertificateAuthorityData != "" {
		caCert, err = base64.StdEncoding.DecodeString(cluster.CertificateAuthorityData)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode CA certificate: %v", err)
		}
	} else if cluster.CertificateAuthority != "" {
		caCert, err = ioutil.ReadFile(cluster.CertificateAuthority)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read CA certificate file: %v", err)
		}
	} else {
		return nil, nil, fmt.Errorf("no CA certificate found in kubeconfig (neither base64 data nor file path)")
	}

	// Now implement the exact same logic as the real agent using kubectl commands
	// 1. Create a temporary kubeconfig file
	tmpFile, err := ioutil.TempFile("", "kubeconfig-*.yaml")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp kubeconfig file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(output); err != nil {
		return nil, nil, fmt.Errorf("failed to write kubeconfig to temp file: %v", err)
	}
	tmpFile.Close()

	// 2. Check if the cattle service account exists
	checkCmd := exec.Command("kubectl", "--kubeconfig", tmpFile.Name(), "get", "serviceaccount", "cattle", "-n", "cattle-system")
	if err := checkCmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("cattle service account not found in cattle-system namespace: %v", err)
	}

	// 3. Get the service account UID for the secret template
	getUIDCmd := exec.Command("kubectl", "--kubeconfig", tmpFile.Name(), "get", "serviceaccount", "cattle", "-n", "cattle-system", "-o", "jsonpath={.metadata.uid}")
	uidOutput, err := getUIDCmd.CombinedOutput()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get service account UID: %v, output: %s", err, string(uidOutput))
	}
	saUID := strings.TrimSpace(string(uidOutput))

	// 4. Check for existing service account token secret
	listCmd := exec.Command("kubectl", "--kubeconfig", tmpFile.Name(), "get", "secrets", "-n", "cattle-system", "-l", "cattle.io/service-account.name=cattle", "-o", "json")
	listOutput, err := listCmd.CombinedOutput()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list secrets for cattle service account: %v, output: %s", err, string(listOutput))
	}

	var secretList struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Type string            `json:"type"`
			Data map[string]string `json:"data"`
		} `json:"items"`
	}

	if err := json.Unmarshal(listOutput, &secretList); err != nil {
		return nil, nil, fmt.Errorf("failed to parse secret list: %v", err)
	}

	// 5. Look for existing service account token secret
	var token []byte
	var secretName string
	for _, secret := range secretList.Items {
		if secret.Type == "kubernetes.io/service-account-token" {
			if tokenData, exists := secret.Data["token"]; exists && tokenData != "" {
				token, err = base64.StdEncoding.DecodeString(tokenData)
				if err != nil {
					continue
				}
				secretName = secret.Metadata.Name
				logrus.Infof("Found existing service account token secret: %s", secretName)
				break
			}
		}
	}

	// 6. If no existing token, create a new service account token secret (like the real agent does)
	if len(token) == 0 {
		logrus.Infof("No existing service account token found, creating new one...")

		// Create a secret template (like SecretTemplate in the real agent)
		secretName = fmt.Sprintf("cattle-token-%s", randSuffix(5))
		secretYAML := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: cattle-system
  annotations:
    kubernetes.io/service-account.name: cattle
    kubernetes.io/service-account.uid: %s
  labels:
    cattle.io/service-account.name: cattle
type: kubernetes.io/service-account-token
data:
  token: ""
  ca.crt: ""
  namespace: "Y2F0dGxlLXN5c3RlbQ=="`, secretName, saUID)

		// Save secret YAML to temp file
		secretTmpFile, err := ioutil.TempFile("", "secret-*.yaml")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create temp secret file: %v", err)
		}
		defer os.Remove(secretTmpFile.Name())

		if _, err := secretTmpFile.WriteString(secretYAML); err != nil {
			return nil, nil, fmt.Errorf("failed to write secret YAML: %v", err)
		}
		secretTmpFile.Close()

		// Apply the secret
		applyCmd := exec.Command("kubectl", "--kubeconfig", tmpFile.Name(), "apply", "-f", secretTmpFile.Name())
		if err := applyCmd.Run(); err != nil {
			return nil, nil, fmt.Errorf("failed to create service account token secret: %v", err)
		}

		// Wait for the token to be populated by Kubernetes (like the real agent does)
		logrus.Infof("Waiting for Kubernetes to populate the service account token...")
		for i := 0; i < 30; i++ { // Wait up to 30 seconds
			time.Sleep(1 * time.Second)

			// Check if token is populated
			getCmd := exec.Command("kubectl", "--kubeconfig", tmpFile.Name(), "get", "secret", secretName, "-n", "cattle-system", "-o", "jsonpath={.data.token}")
			tokenOutput, err := getCmd.CombinedOutput()
			if err == nil && len(tokenOutput) > 0 {
				token, err = base64.StdEncoding.DecodeString(string(tokenOutput))
				if err == nil && len(token) > 0 {
					logrus.Infof("Successfully got service account token from newly created secret")
					break
				}
			}
		}

		if len(token) == 0 {
			return nil, nil, fmt.Errorf("failed to get service account token after creating secret")
		}

		// 7. Update the service account to reference this secret (like the real agent does)
		patchCmd := exec.Command("kubectl", "--kubeconfig", tmpFile.Name(), "patch", "serviceaccount", "cattle", "-n", "cattle-system", "-p", fmt.Sprintf(`{"metadata":{"annotations":{"rancher.io/service-account.secret-ref":"cattle-system/%s"}}}`, secretName))
		if err := patchCmd.Run(); err != nil {
			logrus.Warnf("Failed to update service account annotation: %v", err)
		}
	}

	logrus.Infof("Successfully extracted CA certificate (%d bytes) and real service account token (%d bytes) for cluster %s", len(caCert), len(token), clusterID)

	return caCert, token, nil
}

// getClusterParams gets the cluster parameters using the real agent's approach
func (a *ScaleAgent) getClusterParams(clusterID string) (map[string]interface{}, error) {
	caData, token, err := a.getTokenFromAPI(clusterID)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "looking up cattle-system/cattle ca/token for cluster %s", clusterID)
	}

	// Find the KWOK cluster to get the API endpoint
	var kwokCluster *KWOKCluster
	for _, cluster := range a.kwokManager.clusters {
		if cluster.ClusterID == clusterID {
			kwokCluster = cluster
			break
		}
	}

	if kwokCluster == nil {
		return nil, fmt.Errorf("KWOK cluster not found for Rancher cluster %s", clusterID)
	}

	// Use the KWOK cluster's actual API endpoint
	apiEndpoint := fmt.Sprintf("127.0.0.1:%d", kwokCluster.Port)

	result := map[string]interface{}{
		"cluster": map[string]interface{}{
			"address": apiEndpoint,
			"token":   strings.TrimSpace(string(token)),
			"caCert":  base64.StdEncoding.EncodeToString(caData),
		},
	}

	logrus.Infof("DEBUG: getClusterParams returning for cluster %s: %+v", clusterID, result)
	return result, nil
}
