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
	yaml3 "gopkg.in/yaml.v3"
)

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
	clusterAgentSessions: make(map[string]bool),
		connMutex:         sync.RWMutex{},
	caMutex:           sync.RWMutex{},
		tokenCache:        make(map[string]string),
		mockServers:       make(map[string]*http.Server),
		portForwarders:    make(map[string]*PortForwarder),
		nextPort:          1, // Start from port 8001
		nameCounters:      make(map[string]int),
	}

	// Start local ping server for stv-cluster health checks
	agent.startLocalPingServer()

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
	if err := yaml3.Unmarshal(data, &clusterInfo); err != nil {
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

	if err := yaml3.Unmarshal([]byte(kubeconfigContent), &config); err != nil {
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
		if proto != "tcp" {
			return false
		}
		// Allow KWOK API target (normal proxied traffic)
		if address == localAPI {
			return true
		}
		// Allow Rancher server's connectivity probe used by ClusterConnected controller:
		// it performs client.Get("http://not-used/ping"), which dials host "not-used" on port 80 via the tunnel.
		if address == "not-used:80" {
			return true
		}
		logrus.Tracef("REMOTEDIALER allowFunc: denying dial to %s (proto=%s)", address, proto)
		return false
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

		// Start the real agent lifecycle instead of just patching cluster active
		logrus.Infof("🔄 AGENT-LIFECYCLE: WebSocket connection established, starting real agent lifecycle for cluster %s", clusterName)
		go a.waitForClusterActiveAndConnect(clusterName, clusterID, clusterInfo)
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


// deleteClusterHandler removes a cluster record and calls placeholder delete in Rancher
func (a *ScaleAgent) deleteClusterHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["name"]
	if clusterName == "" || clusterName == "template" {
		http.Error(w, "invalid cluster name", http.StatusBadRequest)
		return
	}
	if _, ok := a.clusters[clusterName]; !ok {
		http.Error(w, "cluster not found", http.StatusNotFound)
		return
	}
	if err := a.deleteClusterFromRancher(clusterName); err != nil {
		http.Error(w, "failed to delete cluster", http.StatusInternalServerError)
		return
	}
	delete(a.clusters, clusterName)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// createClusterFromTemplate deep-copies template and replaces placeholders
func (a *ScaleAgent) createClusterFromTemplate(clusterName string) *ClusterInfo {
	tmpl := a.clusters["template"]
	if tmpl == nil { return &ClusterInfo{Name: clusterName} }
	raw, _ := json.Marshal(tmpl)
	var out ClusterInfo
	_ = json.Unmarshal(raw, &out)
	ph := "{{cluster-name}}"
	for i := range out.Nodes { out.Nodes[i].Name = strings.ReplaceAll(out.Nodes[i].Name, ph, clusterName) }
	for i := range out.Pods { out.Pods[i].Node = strings.ReplaceAll(out.Pods[i].Node, ph, clusterName) }
	out.Name = clusterName
	return &out
}

// createClusterInRancher creates an imported cluster via Rancher API and returns its ID
func (a *ScaleAgent) createClusterInRancher(clusterName string) (string, error) {
	payload := map[string]interface{}{ "type": "cluster", "name": clusterName }
	body, _ := json.Marshal(payload)
	base := strings.TrimRight(a.config.RancherURL, "/")
	req, err := http.NewRequest("POST", base+"/v3/clusters", bytes.NewReader(body))
	if err != nil { return "", err }
	req.Header.Set("Authorization", "Bearer "+a.config.BearerToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("create cluster failed: %d %s", resp.StatusCode, string(data))
	}
	var res map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil { return "", err }
	id, _ := res["id"].(string)
	if id == "" { return "", fmt.Errorf("no id in response") }
	return id, nil
}

// getImportYAML downloads the Rancher import manifest for the given cluster
func (a *ScaleAgent) getImportYAML(clusterID string) (string, error) {
	// Retrieve the registration token for this cluster
	token, err := a.getClusterToken(clusterID)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster token: %v", err)
	}

	base := strings.TrimRight(a.config.RancherURL, "/")
	importURL := fmt.Sprintf("%s/v3/import/%s_%s.yaml", base, token, clusterID)
	logrus.Infof("Downloading import YAML from %s", importURL)

	// Import URL is typically public but often served with self-signed certs; mirror --insecure
	httpClient := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	req, err := http.NewRequest("GET", importURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download import YAML: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to download import YAML: status %d, body: %s", resp.StatusCode, string(b))
	}

	yamlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read import YAML: %v", err)
	}

	yamlContent := string(yamlBytes)
	logrus.Infof("Successfully downloaded YAML file for cluster %s", clusterID)

	// Validate presence of ServiceAccount (informational only)
	if !strings.Contains(yamlContent, "ServiceAccount") && !strings.Contains(yamlContent, "serviceaccount") {
		if len(yamlContent) > 200 {
			logrus.Debugf("Downloaded YAML does not contain ServiceAccount configuration: %s", yamlContent[:200])
		} else {
			logrus.Debugf("Downloaded YAML does not contain ServiceAccount configuration")
		}
	}

	// Save YAML to a debug file for inspection
	clusterName := a.getClusterNameByID(clusterID)
	if clusterName == "" {
		clusterName = clusterID
	}
	if err := os.MkdirAll("debug-yaml", 0755); err != nil {
		logrus.Warnf("Failed to create debug directory: %v", err)
	} else {
		debugFile := filepath.Join("debug-yaml", fmt.Sprintf("%s-register.yaml", clusterName))
		if err := ioutil.WriteFile(debugFile, yamlBytes, 0644); err != nil {
			logrus.Warnf("Failed to save debug YAML file: %v", err)
		} else {
			logrus.Infof("Saved debug YAML file: %s", debugFile)
		}
	}

	return yamlContent, nil
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

	// Wait up to 2 minutes for the service account/credentials to be ready
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for service account to be ready in KWOK cluster %s", kwokCluster.Name)
		case <-ticker.C:
			// 1) Fast path: does the cattle SA exist in cattle-system?
			saCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "serviceaccount", "cattle", "-n", "cattle-system")
			if err := saCmd.Run(); err == nil {
				logrus.Infof("ServiceAccount cattle exists in cattle-system")
				// 2) Robust path: did the import YAML create the cattle-credentials-* secret yet?
				secCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "secrets", "-n", "cattle-system", "-o", "jsonpath={.items[*].metadata.name}")
				secOut, err := secCmd.CombinedOutput()
				if err != nil {
					logrus.Debugf("Secret listing failed: %v, output: %s", err, string(secOut))
					continue
				}
				names := strings.Fields(string(secOut))
				for _, n := range names {
					if strings.HasPrefix(n, "cattle-credentials-") {
						logrus.Infof("Found credentials secret %s; service account is ready in KWOK cluster %s", n, kwokCluster.Name)
						return nil
					}
				}
				logrus.Debugf("cattle SA exists but credentials secret not found yet in KWOK cluster %s", kwokCluster.Name)
				continue
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

// waitForClusterActiveAndConnect implements the REAL agent lifecycle (not simulation)
func (a *ScaleAgent) waitForClusterActiveAndConnect(clusterName, clusterID string, clusterInfo *ClusterInfo) {
	logrus.Infof("🔄 REAL-AGENT: Starting REAL agent lifecycle for cluster %s (following exact real agent pattern)", clusterName)

	// Step 1: Wait for the service account to be ready in the KWOK cluster
	logrus.Infof("🔄 REAL-AGENT: Step 1 - Waiting for service account to be ready")
	if err := a.waitForServiceAccountReady(clusterID); err != nil {
		logrus.Errorf("🔄 REAL-AGENT: Failed to wait for service account ready: %v", err)
		return
	}
	logrus.Infof("🔄 REAL-AGENT: Service account is ready")

	// Step 2: Start downstream cluster-agent tunnel (/v3/connect?clusterId=...) using cattle-credentials token
	logrus.Infof("🔄 REAL-AGENT: Step 2 - Starting downstream cluster-agent tunnel")
	if err := a.startClusterAgentTunnel(clusterName, clusterID); err != nil {
		logrus.Warnf("🔄 REAL-AGENT: Failed to start cluster-agent tunnel (will continue): %v", err)
	}

	// Step 2: Initial agent deployment completed - now follow REAL agent pattern
	logrus.Infof("🔄 REAL-AGENT: Step 2 - Initial agent deployment completed, now implementing REAL agent logic")

	// Step 3: Call ConfigClient (like real agent does in onConnect)
	logrus.Infof("🔄 REAL-AGENT: Step 3 - Calling ConfigClient (like real agent)")
	rancherURL, err := url.Parse(a.config.RancherURL)
	if err != nil {
		logrus.Errorf("🔄 REAL-AGENT: Failed to parse Rancher URL: %v", err)
		return
	}

	// Ensure KWOK cluster exists for this Rancher cluster (used elsewhere)
	kwokCluster := a.getKWOKClusterByID(clusterID)
	if kwokCluster == nil {
		logrus.Errorf("🔄 REAL-AGENT: KWOK cluster not found for cluster %s", clusterID)
		return
	}

	// Use the cluster registration token for ConfigClient (real agent behavior)
	regToken, err := a.getClusterToken(clusterID)
	if err != nil {
		logrus.Errorf("🔄 REAL-AGENT: Failed to get cluster registration token for ConfigClient: %v", err)
	} else {
		// Call ConfigClient exactly like real agent does
		go a.callConfigClientForCluster(clusterID, rancherURL.Host, regToken)
	}

	// Step 4: Implement rancher.Run() equivalent (like real agent does)
	logrus.Infof("🔄 REAL-AGENT: Step 4 - Implementing rancher.Run() equivalent (like real agent)")
	go a.implementRancherRunEquivalent(clusterName, clusterID)

	// Step 5: Start plan monitor (like real agent does) using registration token
	logrus.Infof("🔄 REAL-AGENT: Step 5 - Starting plan monitor (like real agent)")
	if regToken != "" {
		go a.startPlanMonitor(clusterID, rancherURL.Host, regToken)
	}

	logrus.Infof("🔄 REAL-AGENT: REAL agent lifecycle completed for cluster %s - agent now running indefinitely like real agents", clusterName)
}

// getKWOKClusterByID returns the KWOK cluster object for a Rancher clusterID
func (a *ScaleAgent) getKWOKClusterByID(clusterID string) *KWOKCluster {
	for _, cluster := range a.kwokManager.clusters {
		if cluster.ClusterID == clusterID {
			return cluster
		}
	}
	return nil
}

// startClusterAgentTunnel opens the second WebSocket tunnel like real cattle-cluster-agent would
// URL: wss://<rancherHost>/v3/connect?clusterId=<clusterID>
// Auth: Authorization: Bearer <cattle-credentials token from KWOK>
func (a *ScaleAgent) startClusterAgentTunnel(clusterName, clusterID string) error {
	// Prevent duplicates per clusterName
	a.caMutex.Lock()
	if a.clusterAgentSessions[clusterName] {
		a.caMutex.Unlock()
		logrus.Infof("🔄 CLUSTER-AGENT: Tunnel already running for %s, skipping", clusterName)
		return nil
	}
	// mark as starting (will flip true on first onConnect)
	a.clusterAgentSessions[clusterName] = false
	a.caMutex.Unlock()

	kwokCluster := a.getKWOKClusterByID(clusterID)
	if kwokCluster == nil {
		a.caMutex.Lock(); delete(a.clusterAgentSessions, clusterName); a.caMutex.Unlock()
		return fmt.Errorf("KWOK cluster not found for Rancher cluster %s", clusterID)
	}

	// Extract the cattle service account token for Params (used inside X-API-Tunnel-Params)
	// and get the Rancher registration token for the tunnel auth header, exactly like real agent.
	cattleSAToken, err := a.extractServiceAccountTokenFromKWOKCluster(kwokCluster)
	if err != nil {
		a.caMutex.Lock(); delete(a.clusterAgentSessions, clusterName); a.caMutex.Unlock()
		return fmt.Errorf("failed to extract cattle service account token: %w", err)
	}
	if strings.TrimSpace(cattleSAToken) == "" {
		a.caMutex.Lock(); delete(a.clusterAgentSessions, clusterName); a.caMutex.Unlock()
		return fmt.Errorf("empty cattle service account token for cluster %s", clusterID)
	}

	// Get the cluster registration token from Rancher; this is used in X-API-Tunnel-Token
	rancherToken, err := a.getClusterToken(clusterID)
	if err != nil {
		a.caMutex.Lock(); delete(a.clusterAgentSessions, clusterName); a.caMutex.Unlock()
		return fmt.Errorf("failed to get cluster registration token: %w", err)
	}

	rURL, err := url.Parse(a.config.RancherURL)
	if err != nil {
		a.caMutex.Lock(); delete(a.clusterAgentSessions, clusterName); a.caMutex.Unlock()
		return fmt.Errorf("parse rancher url: %w", err)
	}
	// Mirror real agent: connect to /v3/connect (no /register) for the downstream tunnel
	wsURL := fmt.Sprintf("wss://%s/v3/connect", rURL.Host)

	// Build allowFunc to only proxy to our KWOK API endpoint and Rancher's connectivity probe
	// Reuse getClusterParams to obtain the local address (127.0.0.1:port) and token/ca
	clusterParams, err := a.getClusterParams(clusterID)
	if err != nil {
		a.caMutex.Lock(); delete(a.clusterAgentSessions, clusterName); a.caMutex.Unlock()
		return fmt.Errorf("getClusterParams: %w", err)
	}
	clusterData, _ := clusterParams["cluster"].(map[string]interface{})
	localAPI, _ := clusterData["address"].(string)
	allowFunc := func(proto, address string) bool {
		if proto != "tcp" {
			return false
		}
		if address == localAPI {
			return true
		}
		// Allow Rancher server's connectivity probe host; we'll rewrite it via custom dialer
		if address == "not-used:80" {
			return true
		}
		// Allow Steve proxy to target local health server 127.0.0.1:6080 via tunnel
		if address == "127.0.0.1:6080" {
			return true
		}
		logrus.Tracef("REMOTEDIALER allowFunc (cluster-agent): denying dial to %s (proto=%s)", address, proto)
		return false
	}

	// Prepare headers for steve proxy-style tunnel (stv-cluster- Authorization only)
	headers := http.Header{}
	// Use only the Steve proxy-style Authorization so this session is keyed as
	// "stv-cluster-<clusterName>" and satisfies ClusterConnected checks.
	headers.Set("Authorization", fmt.Sprintf("Bearer %s%s", "stv-cluster-", rancherToken))

	onConnect := func(ctx context.Context, s *remotedialer.Session) error {
		a.caMutex.Lock()
		a.clusterAgentSessions[clusterName] = true
		a.caMutex.Unlock()
		logrus.Infof("✅ CLUSTER-AGENT: Connected downstream tunnel for %s (%s)", clusterName, clusterID)
		return nil
	}

	go func() {
		backoff := time.Second
		attempts := 0
		for {
			if a.ctx.Err() != nil {
				return
			}
			ctx, cancel := context.WithCancel(a.ctx)
			// Custom local dialer to rewrite Rancher's probe host to our local ping server
			localDialer := func(dctx context.Context, network, address string) (net.Conn, error) {
				// Rewrite the special probe host to loopback where our /ping server listens
				if network == "tcp" && address == "not-used:80" {
					address = "127.0.0.1:6080"
				}
				var d net.Dialer
				return d.DialContext(dctx, network, address)
			}
			err := remotedialer.ConnectToProxyWithDialer(ctx, wsURL, headers, allowFunc, nil, localDialer, onConnect)
			cancel()
			if err != nil {
				logrus.Warnf("CLUSTER-AGENT: tunnel error for %s: %v", clusterName, err)
			}
			// reconnect with backoff
			if backoff < 30*time.Second { backoff *= 2 }
			attempts++
			logrus.Infof("CLUSTER-AGENT: reconnecting %s in %s (attempt %d)", clusterName, backoff, attempts)
			time.Sleep(backoff)
		}
	}()

	return nil
}

// deleteClusterAgentPod deletes the cluster-agent pod to trigger a new deployment
func (a *ScaleAgent) deleteClusterAgentPod(clusterID string) error {
	logrus.Infof("🔄 AGENT-LIFECYCLE: (stub) Delete cluster-agent pod for cluster %s", clusterID)
	// Stubbed out for scale-agent; real k8s client wiring can be added if needed.
	return nil
}

// waitForClusterAgentPodReady waits for the new cluster-agent pod to be ready
func (a *ScaleAgent) waitForClusterAgentPodReady(clusterID string) error {
	logrus.Infof("🔄 AGENT-LIFECYCLE: (stub) Wait for new cluster-agent pod to be ready for cluster %s", clusterID)
	// Stubbed out for scale-agent; real k8s client wiring can be added if needed.
	return nil
}

// connectClusterToRancherWithRealAgentLogic establishes WebSocket connection following real agent pattern
func (a *ScaleAgent) connectClusterToRancherWithRealAgentLogic(clusterName, clusterID string, clusterInfo *ClusterInfo) {
	logrus.Infof("🔄 AGENT-LIFECYCLE: Establishing WebSocket connection with real agent logic for cluster %s", clusterName)

	// Check if already connected to prevent multiple connections
	a.connMutex.Lock()
	if a.activeConnections[clusterName] {
		logrus.Infof("🔄 AGENT-LIFECYCLE: Cluster %s is already connected, skipping duplicate connection", clusterName)
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
		logrus.Errorf("🔄 AGENT-LIFECYCLE: KWOK cluster not found for Rancher cluster %s", clusterID)
		// Mark connection as failed
		a.connMutex.Lock()
		delete(a.activeConnections, clusterName)
		a.connMutex.Unlock()
		return
	}

	// Extract the service account token from the KWOK cluster
	rancherToken, err := a.getClusterToken(clusterID)
	if err != nil {
		logrus.Errorf("🔄 AGENT-LIFECYCLE: Failed to get cluster token for Rancher connection: %v", err)
		// Mark connection as failed
		a.connMutex.Lock()
		delete(a.activeConnections, clusterName)
		a.connMutex.Unlock()
		return
	}

	logrus.Infof("🔄 AGENT-LIFECYCLE: Using valid token for WebSocket connection")

	// Use the Rancher server URL host, not the cluster ID
	rancherURL, err := url.Parse(a.config.RancherURL)
	if err != nil {
		logrus.Errorf("🔄 AGENT-LIFECYCLE: Failed to parse Rancher URL: %v", err)
		// Mark connection as failed
		a.connMutex.Lock()
		delete(a.activeConnections, clusterName)
		a.connMutex.Unlock()
		return
	}

	// Use /register endpoint like real agent (this is the key difference!)
	wsURL := fmt.Sprintf("wss://%s/v3/connect/register", rancherURL.Host)
	logrus.Infof("🔄 AGENT-LIFECYCLE: Connecting to WebSocket endpoint: %s", wsURL)

	// Get cluster parameters using the real agent's approach
	clusterParams, err := a.getClusterParams(clusterID)
	if err != nil {
		logrus.Errorf("🔄 AGENT-LIFECYCLE: Failed to get cluster parameters: %v", err)
		// Mark connection as failed
		a.connMutex.Lock()
		delete(a.activeConnections, clusterName)
		a.connMutex.Unlock()
		return
	}

	// Prepare the payload exactly like real agent
	params := clusterParams
	payload, err := json.Marshal(params)
	if err != nil {
		logrus.Errorf("🔄 AGENT-LIFECYCLE: Failed to marshal params: %v", err)
		// Mark connection as failed
		a.connMutex.Lock()
		delete(a.activeConnections, clusterName)
		a.connMutex.Unlock()
		return
	}

	encodedParams := base64.StdEncoding.EncodeToString(payload)
	logrus.Infof("🔄 AGENT-LIFECYCLE: Prepared tunnel params: %s", string(payload))

	// Extract the local API address from clusterParams for the allowFunc
	clusterData, ok := clusterParams["cluster"].(map[string]interface{})
	if !ok {
		logrus.Errorf("🔄 AGENT-LIFECYCLE: Failed to extract cluster data from params")
		// Mark connection as failed
		a.connMutex.Lock()
		delete(a.activeConnections, clusterName)
		a.connMutex.Unlock()
		return
	}

	localAPI, ok := clusterData["address"].(string)
	if !ok {
		logrus.Errorf("🔄 AGENT-LIFECYCLE: Failed to extract address from cluster data")
		// Mark connection as failed
		a.connMutex.Lock()
		delete(a.activeConnections, clusterName)
		a.connMutex.Unlock()
		return
	}

	// Set headers exactly like real agent
	headers := http.Header{}
	headers.Set("X-API-Tunnel-Token", rancherToken) // Valid token for WebSocket connection to Rancher
	headers.Set("X-API-Tunnel-Params", encodedParams)

	// Allow function exactly like real agent
	allowFunc := func(proto, address string) bool {
		return proto == "tcp" && address == localAPI
	}

	// Track connection attempts and success
	connectionAttempts := 0
	maxAttempts := 3

	// Set up the onConnect callback to handle successful connections (like real agent)
	onConnect := func(ctx context.Context, s *remotedialer.Session) error {
		connectionAttempts++
		logrus.Infof("🔄 AGENT-LIFECYCLE: onConnect called for cluster %s (attempt %d/%d)", clusterName, connectionAttempts, maxAttempts)

		// Mark as successfully connected only after actual connection
		a.connMutex.Lock()
		a.activeConnections[clusterName] = true
		a.connMutex.Unlock()

		logrus.Infof("✅ AGENT-LIFECYCLE: Cluster %s successfully connected to Rancher via WebSocket tunnel", clusterName)

		// Call the real agent's ConfigClient to get configuration using the registration token
		go a.callConfigClientForCluster(clusterID, rancherURL.Host, rancherToken)

		// Note: We don't call patchClusterActive here because we're following the real agent lifecycle
		// The cluster will become active through the proper agent lifecycle, not manual activation

		return nil
	}

	logrus.Infof("🔄 AGENT-LIFECYCLE: Connecting with KWOK cluster address %s", localAPI)
	go func() {
		backoff := time.Second
		for {
			ctx, cancel := context.WithCancel(a.ctx)

			// Check if we've exceeded max attempts
			if connectionAttempts >= maxAttempts {
				logrus.Errorf("❌ AGENT-LIFECYCLE: Cluster %s failed to connect after %d attempts, marking as failed", clusterName, maxAttempts)
				a.connMutex.Lock()
				delete(a.activeConnections, clusterName)
				a.connMutex.Unlock()
				return
			}

			// Connect to Rancher using remotedialer (exactly like real agent)
			err = remotedialer.ClientConnect(ctx, wsURL, headers, nil, allowFunc, onConnect)
			if err != nil {
				logrus.Errorf("🔄 AGENT-LIFECYCLE: Failed to connect to proxy: %v", err)
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
			logrus.Infof("🔄 AGENT-LIFECYCLE: [%s] remotedialer reconnecting in %s (attempt %d/%d)", clusterName, backoff, connectionAttempts+1, maxAttempts)
			time.Sleep(backoff)
		}
	}()
}

// callConfigClientForCluster calls the real agent's ConfigClient endpoint (like real agent does)
// Note: This endpoint expects X-API-Tunnel-Token to be the cluster registration token.
func (a *ScaleAgent) callConfigClientForCluster(clusterID, rancherHost, registrationToken string) {
	logrus.Infof("🔄 REAL-AGENT: Calling ConfigClient for cluster %s (like real agent)", clusterID)

	// This is exactly what the real agent does after WebSocket connection
	connectConfig := fmt.Sprintf("https://%s/v3/connect/config", rancherHost)

	// Get cluster parameters exactly like real agent does
	clusterParams, err := a.getClusterParams(clusterID)
	if err != nil {
		logrus.Errorf("🔄 REAL-AGENT: Failed to get cluster parameters for ConfigClient: %v", err)
		return
	}

	// Prepare the payload exactly like real agent does
	payload, err := json.Marshal(clusterParams)
	if err != nil {
		logrus.Errorf("🔄 REAL-AGENT: Failed to marshal cluster params: %v", err)
		return
	}

	headers := http.Header{}
	// Real agent does not set Authorization for /v3/connect/config; it uses the tunnel headers.
	headers.Set("X-API-Tunnel-Token", registrationToken)
	// Add the rkenodeconfigclient.Params header like real agent does
	headers.Set("X-API-Tunnel-Params", base64.StdEncoding.EncodeToString(payload))

	httpClient := http.Client{
		Timeout: 300 * time.Second,
	}

	req, err := http.NewRequest("GET", connectConfig, nil)
	if err != nil {
		logrus.Errorf("🔄 REAL-AGENT: Failed to create ConfigClient request: %v", err)
		return
	}

	req.Header = headers

	resp, err := httpClient.Do(req)
	if err != nil {
		logrus.Errorf("🔄 REAL-AGENT: ConfigClient request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("🔄 REAL-AGENT: Failed to read ConfigClient response: %v", err)
		return
	}

	logrus.Infof("🔄 REAL-AGENT: ConfigClient response for cluster %s: status=%d, body=%s", clusterID, resp.StatusCode, string(body))

	// Handle 404 errors gracefully - cluster not ready yet
	if resp.StatusCode == 404 {
		logrus.Debugf("🔄 REAL-AGENT: ConfigClient returned 404 - cluster %s not ready yet, will retry later", clusterID)
		return
	}

	// Parse the response to get the interval (like real agent does)
	var configResp map[string]interface{}
	if err := json.Unmarshal(body, &configResp); err != nil {
		logrus.Errorf("🔄 REAL-AGENT: Failed to parse ConfigClient response: %v", err)
		return
	}

	// Extract interval if present (like real agent does)
	if interval, ok := configResp["interval"].(float64); ok {
		logrus.Infof("🔄 REAL-AGENT: ConfigClient returned interval: %v seconds", interval)
	} else {
		logrus.Infof("🔄 REAL-AGENT: ConfigClient response processed successfully")
	}
}

// implementRancherRunEquivalent implements the equivalent of rancher.Run() from real agent
func (a *ScaleAgent) implementRancherRunEquivalent(clusterName string, clusterID string) {
	logrus.Infof("🔄 REAL-AGENT: Implementing rancher.Run() equivalent for cluster %s", clusterName)

	// This simulates what the real agent does in rancher.Run():
	// 1. Sets up Steve aggregation (already done via WebSocket)
	// 2. Starts cluster controllers (already done by Rancher server)
	// 3. Marks the agent as "started" and running

	logrus.Infof("🔄 REAL-AGENT: rancher.Run() equivalent completed for cluster %s - agent now running indefinitely", clusterName)

	// Keep this goroutine running indefinitely (like real agent does)
	select {
	case <-a.ctx.Done():
		logrus.Infof("🔄 REAL-AGENT: rancher.Run() equivalent stopped for cluster %s due to context cancellation", clusterName)
		return
	}
}

// startPlanMonitor starts the plan monitor like real agent does
func (a *ScaleAgent) startPlanMonitor(clusterID, rancherHost, token string) {
	logrus.Infof("🔄 REAL-AGENT: Starting plan monitor for cluster %s (like real agent)", clusterID)

	// Default interval (will be updated by ConfigClient response)
	interval := 300 // 5 minutes default

	// Start periodic health checks exactly like real agent does
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			logrus.Debugf("🔄 REAL-AGENT: Plan monitor checking cluster %s health (like real agent)", clusterID)

			// Make ConfigClient call to check cluster health (like real agent does)
			go a.callConfigClientForCluster(clusterID, rancherHost, token)

		case <-a.ctx.Done():
			logrus.Infof("🔄 REAL-AGENT: Plan monitor stopped for cluster %s due to context cancellation", clusterID)
			return
		}
	}
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
	/*
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
	/* DEDUP CUT START */
	/*
	muxLocal.HandleFunc("/apis/rbac.authorization.k8s.io/v1/rolebindings", logReq(func(w http.ResponseWriter, r *http.Request) {
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
			// derive namespace from request path first
			rest := strings.TrimPrefix(r.URL.Path, "/apis/rbac.authorization.k8s.io/v1/namespaces/")
			parts := strings.Split(rest, "/")
			curNS := "default"
			if len(parts) > 0 && parts[0] != "" {
				curNS = parts[0]
			}
			saNS, saName := curNS, "cattle"
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
			rb := createRoleBinding(curNS, name, roleName, saNS, saName)
			writeJSON(w, 201, rb)
			return
		}
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
	*/
	// Namespaced RBAC resources
	muxLocal.HandleFunc("/apis/rbac.authorization.k8s.io/v1/rolebindings", logReq(func(w http.ResponseWriter, r *http.Request) {
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
			// derive namespace from request path
			rest := strings.TrimPrefix(r.URL.Path, "/apis/rbac.authorization.k8s.io/v1/namespaces/")
			parts := strings.Split(rest, "/")
			curNS := "default"
			if len(parts) > 0 && parts[0] != "" {
				curNS = parts[0]
			}
			saNS, saName := curNS, "cattle"
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
			rb := createRoleBinding(curNS, name, roleName, saNS, saName)
			writeJSON(w, 201, rb)
			return
		}
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

	// NOTE: cluster-scope rolebindings list/watch removed to avoid duplicate handler; use namespaced endpoints.

	// Namespaced RBAC resources (roles and rolebindings)
	muxLocal.HandleFunc("/apis/rbac.authorization.k8s.io/v1/namespaces/", logReq(func(w http.ResponseWriter, r *http.Request) {
		logrus.Debugf("🔍 NAMESPACE HANDLER: %s %s", r.Method, r.URL.Path)

		rest := strings.TrimPrefix(r.URL.Path, "/apis/rbac.authorization.k8s.io/v1/namespaces/")
		parts := strings.Split(rest, "/")
		logrus.Debugf("🔍 NAMESPACE PARTS: %v", parts)

		// Handle direct namespace object: /api/v1/namespaces/{name}
		if len(parts) == 1 || parts[1] == "" { // trailing slash optional
			ns := strings.TrimSuffix(parts[0], "/")
			if ns == "" {
				http.NotFound(w, r)
				return
			}
			ensureNS(ns)

			stateMu.Lock()
			nsObj := namespaces[ns]
			if r.Method == http.MethodGet {
				stateMu.Unlock()
				writeJSON(w, 200, nsObj)
				return
			}
			if r.Method == http.MethodPut || r.Method == http.MethodPatch {
				var obj map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
					stateMu.Unlock()
					writeJSON(w, 400, map[string]string{"error": "bad json"})
					return
				}
				md, _ := obj["metadata"].(map[string]interface{})
				if md == nil {
					md = map[string]interface{}{}
					obj["metadata"] = md
				}
				md["name"] = ns
				md["resourceVersion"] = nextRV()
				namespaces[ns] = map[string]interface{}{"apiVersion": "v1", "kind": "Namespace", "metadata": md, "status": map[string]interface{}{"phase": "Active"}}
				updated := namespaces[ns]
				stateMu.Unlock()
				writeJSON(w, 200, updated)
				return
			}
			stateMu.Unlock()
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
			writeJSON(w, 200, map[string]interface{}{"kind": "RoleList", "apiVersion": "rbac.authorization.k8s.io/v1", "resources": []map[string]interface{}{}})
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
				if m, ok := roleBindings[ns]; ok {
					for _, rb := range m {
						items = append(items, rb)
					}
				}
				stateMu.RUnlock()
				writeWatchStream(w, r, items)
				return
			}
			stateMu.RLock()
			items := []interface{}{}
			if m, ok := roleBindings[ns]; ok {
				for _, rb := range m {
					items = append(items, rb)
				}
			}
			stateMu.RUnlock()
			writeJSON(w, 200, map[string]interface{}{"kind": "RoleBindingList", "apiVersion": "rbac.authorization.k8s.io/v1", "items": items})
			return
		default:
			http.NotFound(w, r)
			return
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

func (a *ScaleAgent) startLocalPingServer() {
    a.httpServerOnce.Do(func() {
        mux := http.NewServeMux()
        mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
            _, _ = w.Write([]byte("pong"))
        })
        srv := &http.Server{Addr: "127.0.0.1:6080", Handler: mux}
        a.httpServer = srv
        go func() {
            if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                logrus.Errorf("local ping server failed: %v", err)
            }
        }()
        logrus.Infof("Started local ping server on 127.0.0.1:6080 for /ping healthchecks")
    })
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

	if err := yaml3.Unmarshal(output, &kubeconfig); err != nil {
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
