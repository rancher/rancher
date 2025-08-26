package main

import (
	"crypto/tls"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	yaml3 "gopkg.in/yaml.v3"
)

// SimulatedVClusterManager manages multiple simulated-vcluster clusters
type SimulatedVClusterManager struct {
	clusters     map[string]*SimulatedVCluster
	clusterMutex sync.RWMutex
	basePort     int
	kwokctlPath  string
	nextPort     int
}

// SimulatedVCluster represents a single simulated-vcluster-managed cluster
type SimulatedVCluster struct {
	Name       string
	ClusterID  string
	Port       int
	Kubeconfig string
	Status     string
	CreatedAt  time.Time
	Config     *ClusterInfo
	Process    *exec.Cmd
}

// SimClusterConfig represents the cluster configuration from the config file
type SimClusterConfig struct {
	Name  string `yaml:"name"`
	Nodes []SimNode `yaml:"nodes"`
	Pods  []SimPod  `yaml:"pods"`
}

// SimNode represents a node in the cluster configuration
type SimNode struct {
	Name             string            `yaml:"name"`
	Status           string            `yaml:"status"`
	Roles            []string          `yaml:"roles"`
	Age              string            `yaml:"age"`
	Version          string            `yaml:"version"`
	InternalIP       string            `yaml:"internalIP"`
	ExternalIP       string            `yaml:"externalIP"`
	OSImage          string            `yaml:"osImage"`
	KernelVersion    string            `yaml:"kernelVersion"`
	ContainerRuntime string            `yaml:"containerRuntime"`
	Capacity         map[string]string `yaml:"capacity"`
	Allocatable      map[string]string `yaml:"allocatable"`
}

// SimPod represents a pod in the cluster configuration
type SimPod struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Status    string            `yaml:"status"`
	Ready     string            `yaml:"ready"`
	Restarts  int               `yaml:"restarts"`
	Age       string            `yaml:"age"`
	IP        string            `yaml:"ip"`
	Node      string            `yaml:"node"`
	Labels    map[string]string `yaml:"labels"`
}

// NewSimulatedVClusterManager creates a new simulated-vcluster manager
func NewSimulatedVClusterManager(kwokctlPath string, basePort int) *SimulatedVClusterManager {
	return &SimulatedVClusterManager{
		clusters:    make(map[string]*SimulatedVCluster),
		basePort:    basePort,
		kwokctlPath: kwokctlPath,
		nextPort:    basePort,
	}
}

// readClusterConfig reads the cluster configuration from the config file
func (svm *SimulatedVClusterManager) readClusterConfig(clusterName string) (*SimClusterConfig, error) {
	configPath := filepath.Join(os.Getenv("HOME"), ".scale-cluster-agent", "config", "cluster.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster config: %v", err)
	}

	// Replace template variables with actual cluster name
	configContent := strings.ReplaceAll(string(data), "{{cluster-name}}", clusterName)

	var config SimClusterConfig
	if err := yaml3.Unmarshal([]byte(configContent), &config); err != nil {
		return nil, fmt.Errorf("failed to parse cluster config: %v", err)
	}

	return &config, nil
}

// CreateSimulatedVCluster creates a new simulated-vcluster cluster
func (svm *SimulatedVClusterManager) CreateSimulatedVCluster(clusterID, clusterName string) (*SimulatedVCluster, error) {
	logrus.Infof("DEBUG: CreateSimulatedVCluster called with clusterID=%s, clusterName=%s", clusterID, clusterName)

	// Fast path: if exists, return
	svm.clusterMutex.RLock()
	if existing, exists := svm.clusters[clusterID]; exists {
		svm.clusterMutex.RUnlock()
		logrus.Infof("DEBUG: Cluster already exists for clusterID=%s, returning existing", clusterID)
		return existing, nil
	}
	svm.clusterMutex.RUnlock()

	// Create simulated-vcluster cluster name (unique identifier)
	simulatedClusterName := fmt.Sprintf("simulated-vcluster-%s", clusterID)
	logrus.Infof("DEBUG: Creating simulated cluster with name: %s", simulatedClusterName)

	// Create the simulated-vcluster cluster infrastructure (dirs, certs, kubeconfig with placeholder)
	if err := svm.createSimulatedVCluster(simulatedClusterName); err != nil {
		logrus.Errorf("DEBUG: Failed to create simulated-vcluster cluster infrastructure: %v", err)
		return nil, fmt.Errorf("failed to create simulated-vcluster cluster: %v", err)
	}

	// Start the simulated-apiserver and get the port and process
	port, proc, err := svm.startSimulatedVCluster(simulatedClusterName)
	if err != nil {
		logrus.Errorf("DEBUG: Failed to start simulated-vcluster cluster: %v", err)
		// Clean up failed cluster
		_ = svm.deleteSimulatedVCluster(simulatedClusterName)
		return nil, fmt.Errorf("failed to start simulated-vcluster cluster: %v", err)
	}

	// Regenerate kubeconfig with the actual server URL to avoid placeholder issues
	clusterDir := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", simulatedClusterName)
	serverURL := fmt.Sprintf("https://127.0.0.1:%d", port)
	if err := svm.generateKubeconfigWithServer(simulatedClusterName, clusterDir, serverURL); err != nil {
		logrus.Warnf("Failed to regenerate kubeconfig for %s: %v", simulatedClusterName, err)
	}
	kubeconfigPath := filepath.Join(clusterDir, "kubeconfig.yaml")

	// Build cluster object and store
	cluster := &SimulatedVCluster{
		Name:       simulatedClusterName,
		ClusterID:  clusterID,
		Port:       port,
		Kubeconfig: kubeconfigPath,
		Status:     "creating",
		CreatedAt:  time.Now(),
		Process:    proc,
	}

	svm.clusterMutex.Lock()
	svm.clusters[clusterID] = cluster
	logrus.Infof("DEBUG: After storing, clusters map has %d entries", len(svm.clusters))
	svm.clusterMutex.Unlock()

	// Wait until the HTTPS endpoint is reachable to avoid race with immediate kubectl calls
	ready := false
	for i := 0; i < 20; i++ { // ~4s total
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, Timeout: 200 * time.Millisecond}
		resp, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d/healthz", port))
		if err == nil {
			if resp.Body != nil { resp.Body.Close() }
			ready = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !ready {
		logrus.Warnf("simulated-apiserver on port %d did not report ready; continuing optimistically", port)
	}

	// Populate cluster with static data (in-memory metadata)
	if err := svm.populateClusterData(cluster, clusterName); err != nil {
		logrus.Errorf("DEBUG: Failed to populate cluster data: %v", err)
		// Clean up the failed cluster
		_ = svm.deleteSimulatedVCluster(simulatedClusterName)
		svm.clusterMutex.Lock()
		delete(svm.clusters, clusterID)
		svm.clusterMutex.Unlock()
		return nil, fmt.Errorf("failed to populate cluster data: %v", err)
	}

	logrus.Infof("Successfully created simulated-vcluster cluster %s for Rancher cluster %s", simulatedClusterName, clusterID)
	return cluster, nil
}

// createSimulatedVCluster creates a simulated-vcluster cluster using our custom lightweight server
func (svm *SimulatedVClusterManager) createSimulatedVCluster(clusterName string) error {
	// Create cluster directory
	clusterDir := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		return fmt.Errorf("failed to create cluster directory: %v", err)
	}

	// Create kubeconfig directory
	kubeconfigDir := filepath.Join(clusterDir, "kubeconfig")
	if err := os.MkdirAll(kubeconfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create kubeconfig directory: %v", err)
	}

	// Generate self-signed certificates
	if err := svm.generateCertificates(clusterName, clusterDir); err != nil {
		return fmt.Errorf("failed to generate certificates: %v", err)
	}

	// Generate kubeconfig
	if err := svm.generateKubeconfig(clusterName, clusterDir); err != nil {
		return fmt.Errorf("failed to generate kubeconfig: %v", err)
	}

	logrus.Infof("Simulated-vcluster cluster filesystem prepared: %s", clusterName)
	return nil
}

// startSimulatedVCluster starts a simulated-vcluster cluster using our custom lightweight server
func (svm *SimulatedVClusterManager) startSimulatedVCluster(clusterName string) (int, *exec.Cmd, error) {
	// Find an available port
	port := svm.findAvailablePort()

	// Start the custom simulated-apiserver
	// The --db parameter should be a file path, not a directory
	// Ensure the parent directory for the DB exists
	if err := os.MkdirAll("clusters", 0755); err != nil {
		return 0, nil, fmt.Errorf("failed to create clusters directory: %v", err)
	}

	// Resolve binary path and check existence for clearer errors
	binPath := "./simulated-apiserver"
	if abs, err := filepath.Abs(binPath); err == nil {
		binPath = abs
	}
	if _, err := os.Stat(binPath); err != nil {
		return 0, nil, fmt.Errorf("simulated-apiserver binary not found at %s: %v", binPath, err)
	}
	cmd := exec.Command(binPath,
		"--port", strconv.Itoa(port),
		"--db", fmt.Sprintf("clusters/%s.db", clusterName))

	// Apply conservative Go memory limits
	cmd.Env = append(os.Environ(),
		"GOMEMLIMIT=50MiB", // Ultra-low memory target
		"GOGC=25",          // Aggressive GC
	)

	// Start the process
	if err := cmd.Start(); err != nil {
		return 0, nil, fmt.Errorf("failed to start simulated-apiserver: %v", err)
	}
	// Caller will record cmd and port into the cluster map entry
	// Small readiness delay to let the listener come up before first kubectl calls
	time.Sleep(300 * time.Millisecond)
	logrus.Infof("Simulated-vcluster cluster started successfully on port %d", port)
	return port, cmd, nil
}

// getSimulatedVClusterKubeconfig gets the kubeconfig for a simulated-vcluster cluster
func (svm *SimulatedVClusterManager) getSimulatedVClusterKubeconfig(clusterName string) (string, error) {
	// Read the kubeconfig file we generated
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", clusterName, "kubeconfig.yaml")

	data, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to read kubeconfig: %v", err)
	}

	// Replace the PORT placeholder with the actual port
	// We need to find the cluster by name to get its port
	svm.clusterMutex.RLock()
	var clusterPort int
	for _, cluster := range svm.clusters {
		if cluster.Name == clusterName {
			clusterPort = cluster.Port
			break
		}
	}
	svm.clusterMutex.RUnlock()

	if clusterPort > 0 {
		kubeconfig := strings.ReplaceAll(string(data), "PORT", strconv.Itoa(clusterPort))
		return kubeconfig, nil
	}

	return string(data), nil
}

// findAvailablePort finds an available port starting from basePort
func (svm *SimulatedVClusterManager) findAvailablePort() int {
	port := svm.nextPort
	svm.nextPort++
	return port
}

// generateCertificates generates self-signed certificates for the cluster
func (svm *SimulatedVClusterManager) generateCertificates(clusterName, clusterDir string) error {
	certDir := filepath.Join(clusterDir, "pki")
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("failed to create cert directory: %v", err)
	}

	// Generate a small self-signed CA certificate
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate CA private key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          bigIntOne(),
		Subject:               pkix.Name{CommonName: "Simulated Cluster CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(3650 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %v", err)
	}
	caCrtPath := filepath.Join(certDir, "ca.crt")
	caKeyPath := filepath.Join(certDir, "ca.key")
	crtOut, err := os.Create(caCrtPath)
	if err != nil { return fmt.Errorf("failed to create ca.crt: %v", err) }
	if err := pem.Encode(crtOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil { crtOut.Close(); return fmt.Errorf("failed to write ca.crt: %v", err) }
	crtOut.Close()
	keyOut, err := os.Create(caKeyPath)
	if err != nil { return fmt.Errorf("failed to create ca.key: %v", err) }
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil { keyOut.Close(); return fmt.Errorf("failed to write ca.key: %v", err) }
	keyOut.Close()

	// Admin placeholders (optional)
	_ = os.WriteFile(filepath.Join(certDir, "admin.crt"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(certDir, "admin.key"), []byte(""), 0644)
	return nil
}

// bigIntOne returns big.NewInt(1) without extra alloc at call site
func bigIntOne() *big.Int { return big.NewInt(1) }

// generateKubeconfig generates a kubeconfig file for the cluster
func (svm *SimulatedVClusterManager) generateKubeconfig(clusterName, clusterDir string) error {
	// Generate a kubeconfig via YAML marshaling to ensure correctness
	kubeconfigPath := filepath.Join(clusterDir, "kubeconfig.yaml")

	type clusterBlock struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server                   string `yaml:"server"`
			InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify"`
			CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
		} `yaml:"cluster"`
	}
	type contextBlock struct {
		Name    string `yaml:"name"`
		Context struct {
			Cluster string `yaml:"cluster"`
			User    string `yaml:"user"`
		} `yaml:"context"`
	}
	type userBlock struct {
		Name string            `yaml:"name"`
		User map[string]string `yaml:"user"`
	}
	cfg := struct {
		APIVersion     string         `yaml:"apiVersion"`
		Kind           string         `yaml:"kind"`
		Preferences    map[string]any `yaml:"preferences"`
		Clusters       []clusterBlock `yaml:"clusters"`
		Contexts       []contextBlock `yaml:"contexts"`
		CurrentContext string         `yaml:"current-context"`
		Users          []userBlock    `yaml:"users"`
	}{
		APIVersion:  "v1",
		Kind:        "Config",
		Preferences: map[string]any{},
		Clusters:       []clusterBlock{{Name: "cluster"}},
		Contexts:       []contextBlock{{Name: "context"}},
		CurrentContext: "context",
		Users:          []userBlock{{Name: "user", User: map[string]string{"token": "placeholder-token"}}},
	}
	cfg.Clusters[0].Cluster.Server = "https://127.0.0.1:PORT"
	// Use insecure-skip-tls-verify for the simulator and DO NOT set CA data simultaneously to avoid kubectl error
	cfg.Clusters[0].Cluster.InsecureSkipTLSVerify = true
	cfg.Contexts[0].Context.Cluster = cfg.Clusters[0].Name
	cfg.Contexts[0].Context.User = cfg.Users[0].Name

	b, err := yaml3.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal kubeconfig: %v", err)
	}
	if err := os.WriteFile(kubeconfigPath, b, 0644); err != nil {
		return fmt.Errorf("failed to create kubeconfig: %v", err)
	}
	// Debug-log the generated kubeconfig (first few lines) to diagnose YAML issues
	if len(b) > 0 {
		preview := string(b)
		if len(preview) > 400 {
			preview = preview[:400]
		}
		logrus.Infof("DEBUG: generated kubeconfig at %s:\n%s", kubeconfigPath, preview)
	}
	return nil
}

// generateKubeconfigWithServer writes a kubeconfig using the provided server URL
func (svm *SimulatedVClusterManager) generateKubeconfigWithServer(clusterName, clusterDir, serverURL string) error {
	// Same structure as generateKubeconfig, but with a real server URL
	kubeconfigPath := filepath.Join(clusterDir, "kubeconfig.yaml")

	type clusterBlock struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server                   string `yaml:"server"`
			InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify"`
			CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
		} `yaml:"cluster"`
	}
	type contextBlock struct {
		Name    string `yaml:"name"`
		Context struct {
			Cluster string `yaml:"cluster"`
			User    string `yaml:"user"`
		} `yaml:"context"`
	}
	type userBlock struct {
		Name string            `yaml:"name"`
		User map[string]string `yaml:"user"`
	}
	cfg := struct {
		APIVersion     string         `yaml:"apiVersion"`
		Kind           string         `yaml:"kind"`
		Preferences    map[string]any `yaml:"preferences"`
		Clusters       []clusterBlock `yaml:"clusters"`
		Contexts       []contextBlock `yaml:"contexts"`
		CurrentContext string         `yaml:"current-context"`
		Users          []userBlock    `yaml:"users"`
	}{
		APIVersion:     "v1",
		Kind:           "Config",
		Preferences:    map[string]any{},
		Clusters:       []clusterBlock{{Name: "cluster"}},
		Contexts:       []contextBlock{{Name: "context"}},
		CurrentContext: "context",
		Users:          []userBlock{{Name: "user", User: map[string]string{"token": "placeholder-token"}}},
	}
	cfg.Clusters[0].Cluster.Server = serverURL
	// Use insecure-skip-tls-verify for the simulator and DO NOT set CA data simultaneously to avoid kubectl error
	cfg.Clusters[0].Cluster.InsecureSkipTLSVerify = true
	cfg.Contexts[0].Context.Cluster = cfg.Clusters[0].Name
	cfg.Contexts[0].Context.User = cfg.Users[0].Name

	b, err := yaml3.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal kubeconfig: %v", err)
	}
	if err := os.WriteFile(kubeconfigPath, b, 0644); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %v", err)
	}
	preview := string(b)
	if len(preview) > 400 {
		preview = preview[:400]
	}
	logrus.Infof("DEBUG: regenerated kubeconfig at %s with server %s:\n%s", kubeconfigPath, serverURL, preview)
	return nil
}

// extractPortFromKubeconfig extracts the port number from the kubeconfig server URL
func (svm *SimulatedVClusterManager) extractPortFromKubeconfig(kubeconfig string) (int, error) {
	// Parse the kubeconfig to find the server URL
	lines := strings.Split(kubeconfig, "\n")
	for _, line := range lines {
		if strings.Contains(line, "server:") {
			// Extract the port from the server URL
			// Format: server: https://127.0.0.1:PORT
			if strings.Contains(line, "127.0.0.1:") {
				// Split by ":" and get the last part which should be the port
				parts := strings.Split(line, ":")
				if len(parts) >= 4 { // https://127.0.0.1:PORT
					portStr := strings.TrimSpace(parts[3])
					port, err := strconv.Atoi(portStr)
					if err == nil {
						return port, nil
					}
				}
			}
		}
	}
	return 0, fmt.Errorf("could not extract port from kubeconfig")
}

// populateClusterData populates the cluster with static data
func (svm *SimulatedVClusterManager) populateClusterData(cluster *SimulatedVCluster, clusterName string) error {
	// Read cluster configuration
	config, err := svm.readClusterConfig(clusterName)
	if err != nil {
		return fmt.Errorf("failed to read cluster config: %v", err)
	}

	// Convert config to ClusterInfo
	cluster.Config = &ClusterInfo{
		Name:        clusterName,
		Status:      "creating",
		Nodes:       make([]NodeInfo, 0),
		Pods:        make([]PodInfo, 0),
		Services:    make([]ServiceInfo, 0),
		Secrets:     make([]SecretInfo, 0),
		ConfigMaps:  make([]ConfigMapInfo, 0),
		Deployments: make([]DeploymentInfo, 0),
	}

	// Convert nodes
	for _, node := range config.Nodes {
		cluster.Config.Nodes = append(cluster.Config.Nodes, NodeInfo{
			Name:             node.Name,
			Status:           node.Status,
			Roles:            node.Roles,
			Age:              node.Age,
			Version:          node.Version,
			InternalIP:       node.InternalIP,
			ExternalIP:       node.ExternalIP,
			OSImage:          node.OSImage,
			KernelVer:        node.KernelVersion,
			ContainerRuntime: node.ContainerRuntime,
			Capacity:         node.Capacity,
			Allocatable:      node.Allocatable,
		})
	}

	// Convert pods
	for _, pod := range config.Pods {
		cluster.Config.Pods = append(cluster.Config.Pods, PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    pod.Status,
			Ready:     pod.Ready,
			Restarts:  pod.Restarts,
			Age:       pod.Age,
			IP:        pod.IP,
			Node:      pod.Node,
			Labels:    pod.Labels,
		})
	}

	// Add default resources
	svm.addDefaultResources(cluster.Config)

	cluster.Status = "ready"
	return nil
}

// addDefaultResources adds default resources to the cluster
func (svm *SimulatedVClusterManager) addDefaultResources(cluster *ClusterInfo) {
	// Add default namespace
	if len(cluster.Nodes) == 0 {
		cluster.Nodes = append(cluster.Nodes, NodeInfo{
			Name:             "node-1",
			Status:           "Ready",
			Roles:            []string{"worker"},
			Age:              "1d",
			Version:          "v1.28.0",
			InternalIP:       "10.0.0.1",
			ExternalIP:       "192.168.1.1",
			OSImage:          "Ubuntu 22.04.3 LTS",
			KernelVer:        "5.15.0-88-generic",
			ContainerRuntime: "containerd://1.7.0",
			Capacity: map[string]string{
				"cpu":    "4",
				"memory": "8Gi",
				"pods":   "110",
			},
			Allocatable: map[string]string{
				"cpu":    "4",
				"memory": "8Gi",
				"pods":   "110",
			},
		})
	}

	// Add default pods if none exist
	if len(cluster.Pods) == 0 {
		cluster.Pods = append(cluster.Pods, PodInfo{
			Name:      "nginx-pod",
			Namespace: "default",
			Status:    "Running",
			Ready:     "1/1",
			Restarts:  0,
			Age:       "1h",
			IP:        "10.244.0.2",
			Node:      "node-1",
			Labels: map[string]string{
				"app": "nginx",
			},
		})
	}

	// Add default services if none exist
	if len(cluster.Services) == 0 {
		cluster.Services = append(cluster.Services, ServiceInfo{
			Name:       "nginx-service",
			Namespace:  "default",
			Type:       "ClusterIP",
			ClusterIP:  "10.96.0.10",
			ExternalIP: "",
			Ports:      "80:80/TCP",
			Age:        "1h",
			Labels: map[string]string{
				"app": "nginx",
			},
		})
	}
}

// GetCluster gets a cluster by ID
func (svm *SimulatedVClusterManager) GetCluster(clusterID string) (*SimulatedVCluster, bool) {
	svm.clusterMutex.RLock()
	defer svm.clusterMutex.RUnlock()
	cluster, exists := svm.clusters[clusterID]
	return cluster, exists
}

// ListClusters lists all clusters
func (svm *SimulatedVClusterManager) ListClusters() []*SimulatedVCluster {
	svm.clusterMutex.RLock()
	defer svm.clusterMutex.RUnlock()

	clusters := make([]*SimulatedVCluster, 0, len(svm.clusters))
	for _, cluster := range svm.clusters {
		clusters = append(clusters, cluster)
	}
	return clusters
}

// DeleteCluster deletes a cluster
func (svm *SimulatedVClusterManager) DeleteCluster(clusterID string) error {
	svm.clusterMutex.Lock()
	defer svm.clusterMutex.Unlock()

	cluster, exists := svm.clusters[clusterID]
	if !exists {
		return fmt.Errorf("cluster %s not found", clusterID)
	}

	// Delete the simulated-vcluster cluster
	if err := svm.deleteSimulatedVCluster(cluster.Name); err != nil {
		return fmt.Errorf("failed to delete simulated-vcluster cluster: %v", err)
	}

	// Remove from our map
	delete(svm.clusters, clusterID)

	logrus.Infof("Successfully deleted simulated-vcluster cluster %s", clusterID)
	return nil
}

// deleteSimulatedVCluster deletes a simulated-vcluster cluster
func (svm *SimulatedVClusterManager) deleteSimulatedVCluster(clusterName string) error {
	// Find the cluster by name to get the process
	svm.clusterMutex.Lock()
	var clusterToDelete *SimulatedVCluster
	for _, cluster := range svm.clusters {
		if cluster.Name == clusterName {
			clusterToDelete = cluster
			break
		}
	}
	svm.clusterMutex.Unlock()

	// Stop the custom server process if it's running
	if clusterToDelete != nil && clusterToDelete.Process != nil {
		if err := clusterToDelete.Process.Process.Kill(); err != nil {
			logrus.Warnf("Failed to kill simulated-apiserver process for %s: %v", clusterName, err)
		}
	}

	// Remove the cluster directory
	clusterDir := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", clusterName)
	if err := os.RemoveAll(clusterDir); err != nil {
		logrus.Warnf("Failed to remove cluster directory %s: %v", clusterName, err)
	}

	logrus.Infof("Simulated-vcluster cluster deleted successfully: %s", clusterName)
	return nil
}

// CleanupOldClusters cleans up old clusters that are no longer active
func (svm *SimulatedVClusterManager) CleanupOldClusters(activeClusterIDs map[string]bool) error {
	svm.clusterMutex.Lock()
	defer svm.clusterMutex.Unlock()

	var clustersToDelete []string
	for clusterID := range svm.clusters {
		if !activeClusterIDs[clusterID] {
			clustersToDelete = append(clustersToDelete, clusterID)
		}
	}

	for _, clusterID := range clustersToDelete {
		logrus.Infof("Cleaning up old simulated-vcluster cluster: %s", clusterID)
		if err := svm.DeleteCluster(clusterID); err != nil {
			logrus.Warnf("Failed to cleanup old cluster %s: %v", clusterID, err)
		}
	}

	return nil
}

// GetClusterPort gets the port for a cluster
func (svm *SimulatedVClusterManager) GetClusterPort(clusterID string) (int, error) {
	cluster, exists := svm.GetCluster(clusterID)
	if !exists {
		return 0, fmt.Errorf("cluster %s not found", clusterID)
	}
	return cluster.Port, nil
}

// GetClusterKubeconfig gets the kubeconfig for a cluster
func (svm *SimulatedVClusterManager) GetClusterKubeconfig(clusterID string) (string, error) {
	cluster, exists := svm.GetCluster(clusterID)
	if !exists {
		return "", fmt.Errorf("cluster %s not found", clusterID)
	}
	return cluster.Kubeconfig, nil
}

// CreateCluster creates a cluster with the old interface for compatibility
func (svm *SimulatedVClusterManager) CreateCluster(name, clusterID string, clusterInfo *ClusterInfo) (*SimulatedVCluster, error) {
	return svm.CreateSimulatedVCluster(clusterID, name)
}

// RestoreClusterRecord restores a cluster record (compatibility method)
func (svm *SimulatedVClusterManager) RestoreClusterRecord(clusterID, kwokName string) (*SimulatedVCluster, error) {
	// For simulated-vcluster, we don't need to restore anything special
	// Just create the cluster if it doesn't exist
	if cluster, exists := svm.GetCluster(clusterID); exists {
		return cluster, nil
	}
	cluster, err := svm.CreateSimulatedVCluster(clusterID, kwokName)
	return cluster, err
}

// GetClustersMap returns the clusters map for compatibility
func (svm *SimulatedVClusterManager) GetClustersMap() map[string]*SimulatedVCluster {
	svm.clusterMutex.RLock()
	defer svm.clusterMutex.RUnlock()

	logrus.Infof("DEBUG: GetClustersMap called, returning %d clusters", len(svm.clusters))
	for clusterID, cluster := range svm.clusters {
		logrus.Infof("DEBUG: Cluster in map: ID=%s, Name=%s, Port=%d, Status=%s",
			clusterID, cluster.Name, cluster.Port, cluster.Status)
	}

	return svm.clusters
}
