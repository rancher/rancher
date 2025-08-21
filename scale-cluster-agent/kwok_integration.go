package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
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

// KWOKClusterManager manages multiple kwok clusters
type KWOKClusterManager struct {
	clusters     map[string]*KWOKCluster
	clusterMutex sync.RWMutex
	basePort     int
	kwokctlPath  string
	kwokPath     string
}

// KWOKCluster represents a single kwok-managed cluster
type KWOKCluster struct {
	Name       string
	ClusterID  string
	Port       int
	Kubeconfig string
	Status     string
	CreatedAt  time.Time
	Config     *ClusterInfo
}

// ClusterConfig represents the cluster configuration from the config file
type ClusterConfig struct {
	Name  string `yaml:"name"`
	Nodes []Node `yaml:"nodes"`
	Pods  []Pod  `yaml:"pods"`
}

// Node represents a node in the cluster configuration
type Node struct {
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

// Pod represents a pod in the cluster configuration
type Pod struct {
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

// NewKWOKClusterManager creates a new KWOK cluster manager
func NewKWOKClusterManager(kwokctlPath, kwokPath string, basePort int) *KWOKClusterManager {
	return &KWOKClusterManager{
		clusters:    make(map[string]*KWOKCluster),
		basePort:    basePort,
		kwokctlPath: kwokctlPath,
		kwokPath:    kwokPath,
	}
}

// readClusterConfig reads the cluster configuration from the config file
func (km *KWOKClusterManager) readClusterConfig(clusterName string) (*ClusterConfig, error) {
	configPath := filepath.Join(os.Getenv("HOME"), ".scale-cluster-agent", "config", "cluster.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster config: %v", err)
	}

	// Replace template variables with actual cluster name
	configContent := strings.ReplaceAll(string(data), "{{cluster-name}}", clusterName)

	var config ClusterConfig
	if err := yaml3.Unmarshal([]byte(configContent), &config); err != nil {
		return nil, fmt.Errorf("failed to parse cluster config: %v", err)
	}

	return &config, nil
}

// CreateCluster creates a new kwok cluster with the given configuration
func (km *KWOKClusterManager) CreateCluster(clusterName, clusterID string, config *ClusterInfo) (*KWOKCluster, error) {
	logrus.Infof("DEBUG: KWOKClusterManager.CreateCluster called with clusterName=%s, clusterID=%s", clusterName, clusterID)

	// Add timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First, check if cluster already exists and find available port
	km.clusterMutex.Lock()

	// Check if cluster already exists
	if _, exists := km.clusters[clusterID]; exists {
		km.clusterMutex.Unlock()
		logrus.Infof("DEBUG: Cluster %s already exists, returning error", clusterID)
		return nil, fmt.Errorf("cluster %s already exists", clusterID)
	}

	// Find next available port while holding the lock
	port := km.findNextAvailablePortLocked()
	if port == 0 {
		km.clusterMutex.Unlock()
		logrus.Errorf("DEBUG: No available ports for new cluster")
		return nil, fmt.Errorf("no available ports for new cluster")
	}
	logrus.Infof("DEBUG: Found available port %d for cluster %s", port, clusterID)

	// Check context before proceeding
	select {
	case <-ctx.Done():
		km.clusterMutex.Unlock()
		return nil, fmt.Errorf("context cancelled while creating cluster")
	default:
		// Continue with cluster creation
	}

	// Create cluster name that includes port to avoid conflicts
	kwokClusterName := fmt.Sprintf("rancher-%s-%d", clusterID, port)

	logrus.Infof("Creating KWOK cluster %s for Rancher cluster %s on port %d", kwokClusterName, clusterID, port)

	// Release the lock before calling external commands
	km.clusterMutex.Unlock()

	// Create the kwok cluster
	logrus.Infof("DEBUG: About to call kwokctl create cluster for %s", kwokClusterName)
	if err := km.createKWOKCluster(kwokClusterName, port); err != nil {
		logrus.Errorf("DEBUG: Failed to create kwok cluster: %v", err)
		return nil, fmt.Errorf("failed to create kwok cluster: %v", err)
	}
	logrus.Infof("DEBUG: Successfully created kwok cluster %s", kwokClusterName)

	// Wait for cluster to be ready
	if err := km.waitForClusterReady(kwokClusterName); err != nil {
		// Clean up on failure
		km.deleteKWOKCluster(kwokClusterName)
		return nil, fmt.Errorf("cluster failed to become ready: %v", err)
	}

	// Test cluster connectivity before proceeding
	if err := km.testClusterConnectivity(kwokClusterName); err != nil {
		// Clean up on failure
		km.deleteKWOKCluster(kwokClusterName)
		return nil, fmt.Errorf("cluster connectivity test failed: %v", err)
	}
	logrus.Infof("DEBUG: Cluster %s connectivity test passed!", kwokClusterName)


	// Apply anonymous service account AFTER readiness to avoid validation/openapi errors
	if err := km.applyAnonymousServiceAccount(kwokClusterName); err != nil {
		logrus.Warnf("Failed to apply anonymous service account: %v", err)
	}

	// Get kubeconfig
	kubeconfig, err := km.getKubeconfig(kwokClusterName)
	if err != nil {
		km.deleteKWOKCluster(kwokClusterName)
		return nil, fmt.Errorf("failed to get kubeconfig: %v", err)
	}

	// Create cluster record - re-acquire lock
	km.clusterMutex.Lock()
	defer km.clusterMutex.Unlock()

	cluster := &KWOKCluster{
		Name:       kwokClusterName,
		ClusterID:  clusterID,
		Port:       port,
		Kubeconfig: kubeconfig,
		Status:     "Ready",
		CreatedAt:  time.Now(),
		Config:     config,
	}

	km.clusters[clusterID] = cluster

	// Populate cluster with static data - this MUST succeed
	// Use the original clusterName, not the KWOK cluster name
	if err := km.populateClusterData(cluster, clusterName); err != nil {
		logrus.Errorf("Failed to populate cluster data for %s: %v", clusterID, err)
		// Clean up the failed cluster
		km.deleteKWOKCluster(kwokClusterName)
		return nil, fmt.Errorf("failed to populate cluster data: %v", err)
	}

	logrus.Infof("Successfully created KWOK cluster %s for Rancher cluster %s", kwokClusterName, clusterID)
	return cluster, nil
}

// ApplyPostReadyTuning restarts the cluster with low-memory flags after registration to minimize downtime impact.
func (km *KWOKClusterManager) ApplyPostReadyTuning(clusterID string) {
	cluster, ok := km.GetCluster(clusterID)
	if !ok || cluster == nil {
		logrus.Debugf("ApplyPostReadyTuning: cluster %s not found", clusterID)
		return
	}
	name := cluster.Name
	// Ensure kwok.yaml has desired flags and components removed
	if err := km.modifyKWOKConfigForLowMemory(name); err != nil {
		logrus.Warnf("Post-ready tuning: modify kwok.yaml failed: %v", err)
		return
	}
	// Stop/start cluster to apply apiserver flags
	logrus.Infof("Post-ready tuning: restarting cluster %s to apply low-memory apiserver flags", name)
	stopCmd := exec.Command(km.kwokctlPath, "stop", "cluster", "--name", name)
	stopCmd.Env = append(os.Environ(), "GOMEMLIMIT=100MiB", "GOGC=25")
	if out, err := stopCmd.CombinedOutput(); err != nil {
		logrus.Warnf("Post-ready tuning: stop failed: %v, out=%s", err, string(out))
	}
	startCmd := exec.Command(km.kwokctlPath, "start", "cluster", "--name", name)
	startCmd.Env = append(os.Environ(), "GOMEMLIMIT=100MiB", "GOGC=25")
	if out, err := startCmd.CombinedOutput(); err != nil {
		logrus.Warnf("Post-ready tuning: start failed: %v, out=%s", err, string(out))
	} else {
		logrus.Infof("Post-ready tuning applied and cluster %s restarted", name)
		// As a safety, kill scheduler/controller again if spawned
		if err := km.killSchedulerAndController(name); err != nil {
			logrus.Debugf("Post-ready process trim: %v", err)
		}
	}
}

// createKWOKCluster creates a kwok cluster using kwokctl
func (km *KWOKClusterManager) createKWOKCluster(clusterName string, port int) error {
	// Use secure mode instead of insecure mode with binary runtime
	cmd := exec.Command(km.kwokctlPath, "create", "cluster",
		"--name", clusterName,
		"--kube-apiserver-port", strconv.Itoa(port),
		"--runtime", "binary",
		"--kwok-controller-binary", km.kwokPath)

	// Let KWOK use default ports for other components
	// Remove other port definitions to let KWOK use defaults

	// Apply conservative Go memory limits to all child binaries spawned by kwokctl.
	// This helps bound kube-apiserver/controller/scheduler RSS on small laptops.
	// Target: apiserver ~100MB, controller ~40–60MB (actual will vary with load).
	cmd.Env = append(os.Environ(),
		"GOMEMLIMIT=100MiB", // cap Go heap for all spawned binaries
		"GOGC=25",           // aggressive GC to keep RSS low
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create KWOK cluster: %v, output: %s", err, string(output))
	}

	logrus.Infof("KWOK cluster created successfully: %s", string(output))

	// Defer tuning to post-ready stage to avoid disrupting initial readiness.

	return nil
}

// killSchedulerAndController finds and terminates kube-scheduler and kube-controller-manager
// processes that belong to the given KWOK cluster (by matching the cluster path in command line).
func (km *KWOKClusterManager) killSchedulerAndController(clusterName string) error {
	clusterPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", clusterName)
	psCmd := exec.Command("ps", "-axo", "pid,command")
	out, err := psCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ps failed: %w", err)
	}
	lines := strings.Split(string(out), "\n")
	var killed int
	for _, l := range lines {
		if !strings.Contains(l, clusterPath) {
			continue
		}
		if !(strings.Contains(l, "/bin/kube-scheduler") || strings.Contains(l, "/bin/kube-controller-manager")) {
			continue
		}
		fields := strings.Fields(l)
		if len(fields) < 1 {
			continue
		}
		pidStr := fields[0]
		pid, perr := strconv.Atoi(pidStr)
		if perr != nil {
			continue
		}
		// Prefer SIGTERM first
		if p, perr := os.FindProcess(pid); perr == nil {
			_ = p.Signal(os.Interrupt) // gentle
			time.Sleep(500 * time.Millisecond)
			// Hard kill if still present
			_ = p.Kill()
			killed++
			logrus.Infof("Terminated component PID %d for cluster %s: %s", pid, clusterName, l)
		}
	}
	if killed == 0 {
		return errors.New("no scheduler/controller processes matched; nothing to kill")
	}
	return nil
}

// applyAnonymousServiceAccount applies the anonymous service account YAML to allow unauthenticated access
func (km *KWOKClusterManager) applyAnonymousServiceAccount(clusterName string) error {
	anonymousYAML := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: anonymous-access
rules:
- apiGroups: [""]
  resources: ["*"]
  verbs: ["*"]
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]
- nonResourceURLs: ["*"]
  verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: anonymous-access-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: anonymous-access
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: system:anonymous`

	// Save YAML to temporary file
	tmpFile, err := ioutil.TempFile("", "anonymous-access-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(anonymousYAML); err != nil {
		return fmt.Errorf("failed to write YAML: %v", err)
	}
	tmpFile.Close()

	// Get kubeconfig and save to temporary file
	cmd := exec.Command(km.kwokctlPath, "get", "kubeconfig", "--name", clusterName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %v, output: %s", err, string(output))
	}

	// Save kubeconfig to temporary file
	kubeconfigFile, err := ioutil.TempFile("", "kubeconfig-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create kubeconfig temp file: %v", err)
	}
	defer os.Remove(kubeconfigFile.Name())

	if _, err := kubeconfigFile.Write(output); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %v", err)
	}
	kubeconfigFile.Close()

	// Apply the YAML using the kubeconfig file with retries and relaxed validation
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		applyCmd := exec.Command("kubectl",
			"--kubeconfig", kubeconfigFile.Name(),
			"--insecure-skip-tls-verify",
			"apply", "-f", tmpFile.Name(),
			"--validate=false",
		)
		applyOutput, err := applyCmd.CombinedOutput()
		if err == nil {
			logrus.Infof("Anonymous service account applied successfully: %s", string(applyOutput))
			return nil
		}
		lastErr = fmt.Errorf("apply attempt %d failed: %v, output: %s", attempt, err, string(applyOutput))
		logrus.Debugf("Retrying anonymous SA apply: %v", lastErr)
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("failed to apply anonymous service account after retries: %v", lastErr)
}

// waitForClusterReady waits for the cluster to become ready
func (km *KWOKClusterManager) waitForClusterReady(clusterName string) error {
	logrus.Infof("DEBUG: Starting waitForClusterReady for cluster %s", clusterName)

	// Wait up to 2 minutes for cluster to be ready
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			logrus.Errorf("DEBUG: Timeout waiting for cluster %s to be ready", clusterName)
			return fmt.Errorf("timeout waiting for cluster %s to be ready", clusterName)
		case <-ticker.C:
			logrus.Infof("DEBUG: Checking if cluster %s is ready...", clusterName)
			// Check if cluster is ready by trying to get nodes using kubectl directly
			// with the cluster's kubeconfig and skip TLS verification for KWOK clusters
			kubeconfigPath := fmt.Sprintf("%s/.kwok/clusters/%s/kubeconfig.yaml", os.Getenv("HOME"), clusterName)
			cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "--insecure-skip-tls-verify", "get", "nodes")
			if err := cmd.Run(); err == nil {
				logrus.Infof("DEBUG: Cluster %s is now ready!", clusterName)
				return nil // Cluster is ready
			} else {
				logrus.Infof("DEBUG: Cluster %s not ready yet, error: %v", clusterName, err)
			}
		}
	}
}

// getKubeconfig gets the kubeconfig for the cluster
func (km *KWOKClusterManager) getKubeconfig(clusterName string) (string, error) {
	cmd := exec.Command(km.kwokctlPath, "get", "kubeconfig", "--name", clusterName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig: %v", err)
	}
	return string(output), nil
}

// testClusterConnectivity tests if the cluster is accessible and working
func (km *KWOKClusterManager) testClusterConnectivity(clusterName string) error {
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", clusterName, "kubeconfig.yaml")

	// Test basic kubectl access
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "nodes")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cluster connectivity test failed: %v, output: %s", err, string(output))
	}

	logrus.Debugf("Cluster connectivity test passed for %s: %s", clusterName, string(output))
	return nil
}

// deleteKWOKCluster deletes a kwok cluster (internal cleanup method)
func (km *KWOKClusterManager) deleteKWOKCluster(clusterName string) error {
	// Stop the cluster
	cmd := exec.Command(km.kwokctlPath, "stop", "cluster", "--name", clusterName)
	// use same env so child processes inherit limits (mostly no-op for stop)
	cmd.Env = append(os.Environ(), "GOMEMLIMIT=100MiB", "GOGC=25")
	if err := cmd.Run(); err != nil {
		logrus.Warnf("Failed to stop cluster %s during cleanup: %v", clusterName, err)
	}

	// Delete the cluster
	cmd = exec.Command(km.kwokctlPath, "delete", "cluster", "--name", clusterName)
	cmd.Env = append(os.Environ(), "GOMEMLIMIT=100MiB", "GOGC=25")
	if err := cmd.Run(); err != nil {
		logrus.Warnf("Failed to delete cluster %s during cleanup: %v", clusterName, err)
	}

	return nil
}

// modifyKWOKConfigForAnonymousAuth modifies the kwok.yaml file to enable anonymous authentication
func (km *KWOKClusterManager) modifyKWOKConfigForAnonymousAuth(clusterName string) error {
	kwokConfigPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", clusterName, "kwok.yaml")

	// Read the current config
	data, err := os.ReadFile(kwokConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read kwok.yaml: %v", err)
	}

	// Replace authorization-mode=Node,RBAC with authorization-mode=AlwaysAllow
	// and add anonymous-auth=true
	content := string(data)
	content = strings.ReplaceAll(content, "--authorization-mode=Node,RBAC", "--authorization-mode=AlwaysAllow")

	// Remove the client-ca-file line entirely to disable client certificate validation
	lines := strings.Split(content, "\n")
	var newLines []string
	for _, line := range lines {
		if !strings.Contains(line, "--client-ca-file=") {
			newLines = append(newLines, line)
		}
	}
	content = strings.Join(newLines, "\n")

	// Add anonymous-auth=true after the authorization-mode line
	content = strings.ReplaceAll(content,
		"--anonymous-auth=true",
		"--anonymous-auth=true\n  - --enable-bootstrap-token-auth=false\n  - --token-auth-file=")

	// Write the modified config back
	if err := os.WriteFile(kwokConfigPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write modified kwok.yaml: %v", err)
	}

	logrus.Infof("Successfully modified kwok.yaml for cluster %s to enable anonymous authentication and disable client certificate validation", clusterName)
	return nil
}

// createAnonymousRBAC creates RBAC rules to allow anonymous access to the cluster
func (km *KWOKClusterManager) createAnonymousRBAC(cluster *KWOKCluster) error {
	anonymousRBACYAML := `# Role that grants read-only access
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: anonymous-reader
rules:
- apiGroups: [""]
  resources: ["pods", "services", "configmaps", "secrets", "nodes", "namespaces", "componentstatuses"]
  verbs: ["get", "list", "watch"]

---

# Binds the anonymous role to the "system:anonymous" user
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: anonymous-reader-binding
subjects:
- kind: User
  name: system:anonymous
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: anonymous-reader
  apiGroup: rbac.authorization.k8s.io
`

	// Use regular kubectl with the cluster's kubeconfig
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", cluster.Name, "kubeconfig.yaml")

	// Check if kubeconfig exists
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return fmt.Errorf("kubeconfig file not found at %s", kubeconfigPath)
	}

	logrus.Infof("Creating anonymous RBAC rules for cluster %s", cluster.Name)
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(anonymousRBACYAML)

	// Capture output for better error reporting
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create anonymous RBAC rules: %v, output: %s", err, string(output))
	}

	logrus.Infof("Successfully created anonymous RBAC rules for cluster %s: %s", cluster.Name, string(output))
	return nil
}

// CleanupOldClusters removes KWOK clusters that are no longer in the agent's memory
func (km *KWOKClusterManager) CleanupOldClusters(activeClusterIDs map[string]bool) error {
	km.clusterMutex.Lock()
	defer km.clusterMutex.Unlock()

	// Get all KWOK clusters on disk
	kwokClustersDir := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters")
	entries, err := os.ReadDir(kwokClustersDir)
	if err != nil {
		return fmt.Errorf("failed to read KWOK clusters directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		clusterName := entry.Name()
		// Check if this cluster is still active
		isActive := false

		// Extract cluster ID from KWOK cluster name (format: rancher-{clusterID}-{port})
		// Example: rancher-c-kxp7j-8001 -> c-kxp7j
		if strings.HasPrefix(clusterName, "rancher-") {
			parts := strings.Split(clusterName, "-")
			if len(parts) >= 3 {
				// Reconstruct the cluster ID (parts[1] to parts[len(parts)-2])
				clusterID := strings.Join(parts[1:len(parts)-1], "-")
				if activeClusterIDs[clusterID] {
					isActive = true
				}
			}
		}

		if !isActive {
			logrus.Infof("Cleaning up inactive KWOK cluster: %s", clusterName)
			if err := km.deleteKWOKCluster(clusterName); err != nil {
				logrus.Warnf("Failed to cleanup KWOK cluster %s: %v", clusterName, err)
			}
		}
	}

	return nil
}

// populateClusterData populates the cluster with static data using kubectl
func (km *KWOKClusterManager) populateClusterData(cluster *KWOKCluster, clusterName string) error {
	logrus.Infof("Starting to populate cluster %s with resources from config", clusterName)

	// Read the cluster configuration from the config file
	clusterConfig, err := km.readClusterConfig(clusterName)
	if err != nil {
		return fmt.Errorf("failed to read cluster config: %v", err)
	}

	logrus.Infof("Successfully read cluster config: %d nodes, %d pods", len(clusterConfig.Nodes), len(clusterConfig.Pods))

	// Create namespaces
	logrus.Infof("Creating namespaces for cluster %s", clusterName)
	for _, ns := range []string{"default", "kube-system", "cattle-system", "cattle-impersonation-system", "monitoring"} {
		if err := km.createNamespace(cluster, ns); err != nil {
			return fmt.Errorf("failed to create namespace %s: %v", ns, err)
		}
		logrus.Infof("Successfully created namespace %s", ns)
	}

	// Create nodes from the config
	logrus.Infof("Creating %d nodes for cluster %s", len(clusterConfig.Nodes), clusterName)
	for _, node := range clusterConfig.Nodes {
		if err := km.createNodeFromConfig(cluster, node); err != nil {
			return fmt.Errorf("failed to create node %s: %v", node.Name, err)
		}
		logrus.Infof("Successfully created node %s", node.Name)
	}

	// Create pods from the config
	logrus.Infof("Creating %d pods for cluster %s", len(clusterConfig.Pods), clusterName)
	for _, pod := range clusterConfig.Pods {
		if err := km.createPodFromConfig(cluster, pod); err != nil {
			return fmt.Errorf("failed to create pod %s: %v", pod.Name, err)
		}
		logrus.Infof("Successfully created pod %s", pod.Name)
	}

	// Create anonymous RBAC rules to allow unauthenticated access
	if err := km.createAnonymousRBAC(cluster); err != nil {
		logrus.Warnf("Failed to create anonymous RBAC rules: %v", err)
		// Continue anyway, as this is not critical
	}

	logrus.Infof("Successfully populated cluster %s with all resources", clusterName)
	return nil
}

// createNamespace creates a namespace in the cluster
func (km *KWOKClusterManager) createNamespace(cluster *KWOKCluster, name string) error {
	nsYAML := fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    name: %s
`, name, name)

	// Use regular kubectl with the cluster's kubeconfig instead of kwokctl kubectl
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", cluster.Name, "kubeconfig.yaml")

	// Check if kubeconfig exists
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return fmt.Errorf("kubeconfig file not found at %s", kubeconfigPath)
	}

	logrus.Debugf("Creating namespace %s using kubeconfig %s", name, kubeconfigPath)
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(nsYAML)

	// Capture output for better error reporting
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %v, output: %s", name, err, string(output))
	}

	logrus.Debugf("Successfully created namespace %s: %s", name, string(output))
	return nil
}

// createNode creates a node in the cluster
func (km *KWOKClusterManager) createNode(cluster *KWOKCluster, node NodeInfo) error {
	nodeYAML := fmt.Sprintf(`apiVersion: v1
kind: Node
metadata:
  name: %s
  labels:
    kubernetes.io/hostname: %s
    node-role.kubernetes.io/control-plane: "true"
spec:
  taints:
  - key: node-role.kubernetes.io/control-plane
    effect: NoSchedule
status:
  conditions:
  - type: Ready
    status: "True"
    lastHeartbeatTime: "%s"
    lastTransitionTime: "%s"
  capacity:
    cpu: "%s"
    memory: "%s"
    pods: "%s"
  allocatable:
    cpu: "%s"
    memory: "%s"
    pods: "%s"
  nodeInfo:
    kubeletVersion: "%s"
    osImage: "%s"
    kernelVersion: "%s"
    containerRuntimeVersion: "%s"
`,
		node.Name, node.Name,
		time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339),
		node.Capacity["cpu"], node.Capacity["memory"], node.Capacity["pods"],
		node.Allocatable["cpu"], node.Allocatable["memory"], node.Allocatable["pods"],
		node.Version, node.OSImage, node.KernelVer, node.ContainerRuntime)

	// Use regular kubectl with the cluster's kubeconfig instead of kwokctl kubectl
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", cluster.Name, "kubeconfig.yaml")
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(nodeYAML)
	return cmd.Run()
}

// createNodeFromConfig creates a node from the cluster configuration
func (km *KWOKClusterManager) createNodeFromConfig(cluster *KWOKCluster, node Node) error {
	// Build node labels
	var labels []string
	for _, role := range node.Roles {
		labels = append(labels, fmt.Sprintf("node-role.kubernetes.io/%s: \"true\"", role))
	}
	labelsStr := strings.Join(labels, "\n    ")

	nodeYAML := fmt.Sprintf(`apiVersion: v1
kind: Node
metadata:
  name: %s
  labels:
    kubernetes.io/hostname: %s
    %s
spec:
  taints:
  - key: node-role.kubernetes.io/control-plane
    effect: NoSchedule
status:
  conditions:
  - type: Ready
    status: "%s"
    lastHeartbeatTime: "%s"
    lastTransitionTime: "%s"
  capacity:
    cpu: "%s"
    memory: "%s"
    pods: "%s"
  allocatable:
    cpu: "%s"
    memory: "%s"
    pods: "%s"
  nodeInfo:
    kubeletVersion: "%s"
    osImage: "%s"
    kernelVersion: "%s"
    containerRuntimeVersion: "%s"
  addresses:
  - type: InternalIP
    address: "%s"
  - type: ExternalIP
    address: "%s"
`,
		node.Name, node.Name, labelsStr,
		node.Status, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339),
		node.Capacity["cpu"], node.Capacity["memory"], node.Capacity["pods"],
		node.Allocatable["cpu"], node.Allocatable["memory"], node.Allocatable["pods"],
		node.Version, node.OSImage, node.KernelVersion, node.ContainerRuntime,
		node.InternalIP, node.ExternalIP)

	// Use regular kubectl with the cluster's kubeconfig
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", cluster.Name, "kubeconfig.yaml")
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(nodeYAML)
	return cmd.Run()
}

// createPod creates a pod in the cluster
func (km *KWOKClusterManager) createPod(cluster *KWOKCluster, pod PodInfo) error {
	podYAML := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
%s
spec:
  containers:
  - name: %s
    image: busybox:latest
    command: ["sleep", "3600"]
  nodeName: %s
status:
  phase: Running
  podIP: %s
  conditions:
  - type: Ready
    status: "True"
`,
		pod.Name, pod.Namespace,
		formatLabels(pod.Labels),
		pod.Name, pod.Node, pod.IP)

	// Use regular kubectl with the cluster's kubeconfig instead of kwokctl kubectl
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", cluster.Name, "kubeconfig.yaml")
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(podYAML)
	return cmd.Run()
}

// createPodFromConfig creates a pod from the cluster configuration
func (km *KWOKClusterManager) createPodFromConfig(cluster *KWOKCluster, pod Pod) error {
	// Format labels
	labelsStr := formatLabels(pod.Labels)

	podYAML := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
%s
spec:
  containers:
  - name: %s
    image: busybox:latest
    command: ["sleep", "3600"]
  nodeName: %s
status:
  phase: %s
  podIP: %s
  conditions:
  - type: Ready
    status: "True"
`,
		pod.Name, pod.Namespace, labelsStr,
		pod.Name, pod.Node, pod.Status, pod.IP)

	// Use regular kubectl with the cluster's kubeconfig
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", cluster.Name, "kubeconfig.yaml")
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(podYAML)
	return cmd.Run()
}

// formatLabels formats labels for YAML
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	var result strings.Builder
	for k, v := range labels {
		result.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
	}
	return result.String()
}

// findNextAvailablePortLocked finds the next available port for a new cluster
// This function assumes the caller already holds the clusterMutex lock
func (km *KWOKClusterManager) findNextAvailablePortLocked() int {
	usedPorts := make(map[int]bool)

	// Check ports from in-memory clusters
	for _, cluster := range km.clusters {
		usedPorts[cluster.Port] = true
	}

	// Also check ports from actual KWOK clusters on disk
	// This handles cases where clusters exist on disk but not in memory
	homeDir := os.Getenv("HOME")
	kwokClustersDir := fmt.Sprintf("%s/.kwok/clusters", homeDir)

	if entries, err := os.ReadDir(kwokClustersDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "rancher-") {
				// Extract port from cluster name (e.g., "rancher-c-abc123-8001" -> 8001)
				parts := strings.Split(entry.Name(), "-")
				if len(parts) >= 4 {
					if port, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
						usedPorts[port] = true
						logrus.Debugf("Found existing KWOK cluster on port %d: %s", port, entry.Name())
					}
				}
			}
		}
	}

	// Start from base port and find first available
	for port := km.basePort; port < km.basePort+10000; port += 10 {
		if !usedPorts[port] {
			logrus.Debugf("Found available port %d for new cluster", port)
			return port
		}
	}
	return 0
}

// DeleteCluster deletes a kwok cluster
func (km *KWOKClusterManager) DeleteCluster(clusterID string) error {
	km.clusterMutex.Lock()
	defer km.clusterMutex.Unlock()

	cluster, exists := km.clusters[clusterID]
	if !exists {
		return fmt.Errorf("cluster %s not found", clusterID)
	}

	logrus.Infof("Deleting KWOK cluster %s for Rancher cluster %s", cluster.Name, clusterID)

	// Stop the cluster
	cmd := exec.Command(km.kwokctlPath, "stop", "cluster", "--name", cluster.Name)
	if err := cmd.Run(); err != nil {
		logrus.Warnf("Failed to stop cluster %s: %v", cluster.Name, err)
	}

	// Delete the cluster
	cmd = exec.Command(km.kwokctlPath, "delete", "cluster", "--name", cluster.Name)
	if err := cmd.Run(); err != nil {
		logrus.Warnf("Failed to delete cluster %s: %v", cluster.Name, err)
	}

	// Remove from our tracking
	delete(km.clusters, clusterID)

	logrus.Infof("Successfully deleted KWOK cluster %s for Rancher cluster %s", cluster.Name, clusterID)
	return nil
}

// GetCluster returns a cluster by ID
func (km *KWOKClusterManager) GetCluster(clusterID string) (*KWOKCluster, bool) {
	km.clusterMutex.RLock()
	defer km.clusterMutex.RUnlock()

	cluster, exists := km.clusters[clusterID]
	return cluster, exists
}

// ListClusters returns all managed clusters
func (km *KWOKClusterManager) ListClusters() []*KWOKCluster {
	km.clusterMutex.RLock()
	defer km.clusterMutex.RUnlock()

	clusters := make([]*KWOKCluster, 0, len(km.clusters))
	for _, cluster := range km.clusters {
		clusters = append(clusters, cluster)
	}
	return clusters
}

// GetClusterPort returns the port for a cluster
func (km *KWOKClusterManager) GetClusterPort(clusterID string) (int, bool) {
	cluster, exists := km.GetCluster(clusterID)
	if !exists {
		return 0, false
	}
	return cluster.Port, true
}

// Cleanup stops and deletes all clusters
func (km *KWOKClusterManager) Cleanup() {
	km.clusterMutex.Lock()
	defer km.clusterMutex.Unlock()

	for clusterID, cluster := range km.clusters {
		logrus.Infof("Cleaning up KWOK cluster %s for Rancher cluster %s", cluster.Name, clusterID)

		// Stop the cluster
	cmd := exec.Command(km.kwokctlPath, "stop", "cluster", "--name", cluster.Name)
	cmd.Env = append(os.Environ(), "GOMEMLIMIT=100MiB", "GOGC=25")
		cmd.Run() // Ignore errors during cleanup

		// Delete the cluster
	cmd = exec.Command(km.kwokctlPath, "delete", "cluster", "--name", cluster.Name)
	cmd.Env = append(os.Environ(), "GOMEMLIMIT=100MiB", "GOGC=25")
		cmd.Run() // Ignore errors during cleanup
	}

	km.clusters = make(map[string]*KWOKCluster)
}

// modifyKWOKConfigForLowMemory appends conservative low-memory flags to component args in kwok.yaml
// and leaves existing args intact. It aims to reduce kube-apiserver RSS (~100MB target) and
// controller/scheduler RSS with minimal functional impact.
func (km *KWOKClusterManager) modifyKWOKConfigForLowMemory(clusterName string) error {
	// Load kwok.yaml
	kwokConfigPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", clusterName, "kwok.yaml")
	data, err := os.ReadFile(kwokConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read kwok.yaml: %v", err)
	}

	// Unmarshal into a generic map to preserve unknown fields
	var doc map[string]interface{}
	if err := yaml3.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("failed to parse kwok.yaml: %v", err)
	}

	// Fetch components list
	compsVal, ok := doc["components"]
	if !ok {
		return fmt.Errorf("kwok.yaml missing 'components' section")
	}

	comps, ok := compsVal.([]interface{})
	if !ok {
		return fmt.Errorf("kwok.yaml 'components' is not a list")
	}

	// Desired flags for apiserver
	apiServerDesired := []string{
		"--profiling=false",
		"--watch-cache=false",
		"--event-ttl=5m",
		"--max-requests-inflight=100",
		"--max-mutating-requests-inflight=25",
		"--default-watch-cache-size=0",
		"--min-request-timeout=1800",
		"--enable-aggregator-routing=false",
		"--request-timeout=1m",
		"--runtime-config=api/all=false,core/v1=true,rbac.authorization.k8s.io/v1=true,apiextensions.k8s.io/v1=true,coordination.k8s.io/v1=true,authentication.k8s.io/v1=true,authorization.k8s.io/v1=true",
		"--enable-admission-plugins=NamespaceLifecycle",
		"--disable-admission-plugins=MutatingAdmissionWebhook,ValidatingAdmissionWebhook,NodeRestriction,ServiceAccount,ResourceQuota,LimitRanger,DefaultStorageClass,DefaultTolerationSeconds,PodSecurity,EventRateLimit,StorageObjectInUseProtection,PodNodeSelector,PodTolerationRestriction,PersistentVolumeLabel",
	}

	// Helper to normalize []interface{} to []string
	toStrings := func(in []interface{}) []string {
		out := make([]string, 0, len(in))
		for _, v := range in {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}

	// Helper to convert []string to []interface{}
	toInterfaces := func(in []string) []interface{} {
		out := make([]interface{}, 0, len(in))
		for _, s := range in {
			out = append(out, s)
		}
		return out
	}

	// Helper: ensure flags present, removing existing with same prefix to avoid conflicts
	ensureFlags := func(existing []string, desired []string) []string {
		// Build map of prefixes for desired flags (up to '=' if present)
		prefix := func(s string) string {
			if idx := strings.IndexByte(s, '='); idx > 0 {
				return s[:idx]
			}
			return s
		}
		desiredSet := map[string]string{}
		for _, d := range desired {
			desiredSet[prefix(d)] = d
		}
		// Keep existing that don't conflict
		merged := make([]string, 0, len(existing)+len(desired))
		for _, e := range existing {
			if _, conflict := desiredSet[prefix(e)]; !conflict {
				merged = append(merged, e)
			}
		}
		// Append desired in order
		merged = append(merged, desired...)
		return merged
	}

	// Build a new components list, removing scheduler and controller-manager
	newComps := make([]interface{}, 0, len(comps))
	for _, c := range comps {
		cm, ok := c.(map[string]interface{})
		if !ok {
			// Preserve unrecognized entries
			newComps = append(newComps, c)
			continue
		}
		nameAny, hasName := cm["name"]
		name, _ := nameAny.(string)
		if hasName {
			if name == "kube-scheduler" || name == "kube-controller-manager" {
				// drop it
				continue
			}
			if name == "kube-apiserver" {
				// Inject/ensure low-memory flags
				if argsAny, ok := cm["args"]; ok {
					if argsSlice, ok := argsAny.([]interface{}); ok {
						existing := toStrings(argsSlice)
						cm["args"] = toInterfaces(ensureFlags(existing, apiServerDesired))
					}
				} else {
					cm["args"] = toInterfaces(apiServerDesired)
				}
			}
		}
		newComps = append(newComps, cm)
	}
	doc["components"] = newComps

	// Marshal back to YAML
	out, err := yaml3.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal tuned kwok.yaml: %v", err)
	}
	if err := os.WriteFile(kwokConfigPath, out, 0644); err != nil {
		return fmt.Errorf("failed to write tuned kwok.yaml: %v", err)
	}

	logrus.Infof("Applied low-memory tuning to %s (removed scheduler/controller, tuned apiserver flags)", kwokConfigPath)

	// Additionally update metadata.yaml which controls component processes under kwokctl
	if err := km.tuneKWOKMetadata(clusterName); err != nil {
		logrus.Warnf("Failed tuning metadata.yaml: %v", err)
	}
	return nil
}

// tuneKWOKMetadata updates ~/.kwok/clusters/<name>/metadata.yaml to remove scheduler/controller
// and inject low-memory flags into kube-apiserver. It supports two shapes: components list, or
// top-level kubeApiserver/kubeControllerManager/kubeScheduler keys.
func (km *KWOKClusterManager) tuneKWOKMetadata(clusterName string) error {
	metaPath := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", clusterName, "metadata.yaml")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		// metadata.yaml may not exist in some KWOK versions; not fatal
		return fmt.Errorf("read metadata.yaml: %w", err)
	}

	var doc map[string]interface{}
	if err := yaml3.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse metadata.yaml: %w", err)
	}

	// helper conversions
	toStrings := func(in []interface{}) []string {
		out := make([]string, 0, len(in))
		for _, v := range in {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	toInterfaces := func(in []string) []interface{} {
		out := make([]interface{}, 0, len(in))
		for _, s := range in {
			out = append(out, s)
		}
		return out
	}
	prefix := func(s string) string {
		if idx := strings.IndexByte(s, '='); idx > 0 {
			return s[:idx]
		}
		return s
	}
	ensureFlags := func(existing []string, desired []string) []string {
		desiredSet := map[string]string{}
		for _, d := range desired {
			desiredSet[prefix(d)] = d
		}
		merged := make([]string, 0, len(existing)+len(desired))
		for _, e := range existing {
			if _, conflict := desiredSet[prefix(e)]; !conflict {
				merged = append(merged, e)
			}
		}
		merged = append(merged, desired...)
		return merged
	}

	apiServerDesired := []string{
		"--profiling=false",
		"--watch-cache=false",
		"--event-ttl=5m",
		"--max-requests-inflight=100",
		"--max-mutating-requests-inflight=25",
		"--default-watch-cache-size=0",
		"--min-request-timeout=1800",
		"--enable-aggregator-routing=false",
		"--request-timeout=1m",
		"--runtime-config=api/all=false,core/v1=true,rbac.authorization.k8s.io/v1=true,apiextensions.k8s.io/v1=true,coordination.k8s.io/v1=true,authentication.k8s.io/v1=true,authorization.k8s.io/v1=true",
		"--enable-admission-plugins=NamespaceLifecycle",
		"--disable-admission-plugins=MutatingAdmissionWebhook,ValidatingAdmissionWebhook,NodeRestriction,ServiceAccount,ResourceQuota,LimitRanger,DefaultStorageClass,DefaultTolerationSeconds,PodSecurity,EventRateLimit,StorageObjectInUseProtection,PodNodeSelector,PodTolerationRestriction,PersistentVolumeLabel",
	}

	updated := false

	// Case 1: components list
	if compsAny, ok := doc["components"]; ok {
		if comps, ok := compsAny.([]interface{}); ok {
			newComps := make([]interface{}, 0, len(comps))
			for _, c := range comps {
				cm, ok := c.(map[string]interface{})
				if !ok {
					newComps = append(newComps, c)
					continue
				}
				name, _ := cm["name"].(string)
				switch name {
				case "kube-scheduler", "kube-controller-manager":
					updated = true
					continue // drop
				case "kube-apiserver":
					if argsAny, ok := cm["args"]; ok {
						if argsSlice, ok := argsAny.([]interface{}); ok {
							existing := toStrings(argsSlice)
							cm["args"] = toInterfaces(ensureFlags(existing, apiServerDesired))
							updated = true
						}
					} else {
						cm["args"] = toInterfaces(apiServerDesired)
						updated = true
					}
				}
				newComps = append(newComps, cm)
			}
			doc["components"] = newComps
		}
	}

	// Case 2: top-level component keys
	if apAny, ok := doc["kubeApiserver"]; ok {
		if ap, ok := apAny.(map[string]interface{}); ok {
			if argsAny, ok := ap["args"]; ok {
				if argsSlice, ok := argsAny.([]interface{}); ok {
					existing := toStrings(argsSlice)
					ap["args"] = toInterfaces(ensureFlags(existing, apiServerDesired))
					doc["kubeApiserver"] = ap
					updated = true
				}
			} else {
				ap["args"] = toInterfaces(apiServerDesired)
				doc["kubeApiserver"] = ap
				updated = true
			}
		}
	}
	// Remove scheduler/controller if present
	if _, ok := doc["kubeScheduler"]; ok {
		delete(doc, "kubeScheduler")
		updated = true
	}
	if _, ok := doc["kubeControllerManager"]; ok {
		delete(doc, "kubeControllerManager")
		updated = true
	}

	if !updated {
		return fmt.Errorf("no changes applied to metadata.yaml (unexpected schema)")
	}

	out, err := yaml3.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal metadata.yaml: %w", err)
	}
	if err := os.WriteFile(metaPath, out, 0644); err != nil {
		return fmt.Errorf("write metadata.yaml: %w", err)
	}
	logrus.Infof("Applied low-memory tuning to %s (removed scheduler/controller, tuned apiserver)", metaPath)
	return nil
}
