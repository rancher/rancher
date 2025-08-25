package main

import (
	"context"
	"net/http"
	"sync"
	"time"
)

const (
	version = "1.0.0"
)

// Config holds agent configuration
type Config struct {
	RancherURL  string `yaml:"RancherURL"`
	BearerToken string `yaml:"BearerToken"`
	ListenPort  int    `yaml:"ListenPort"`
	LogLevel    string `yaml:"LogLevel"`
}

// ClusterInfo describes a simulated cluster's high-level objects
type ClusterInfo struct {
	Name        string           `json:"name"`
	ClusterID   string           `json:"cluster_id,omitempty"`
	Status      string           `json:"status,omitempty"`
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

// PortForwarder keeps a tiny HTTP forwarder if needed
type PortForwarder struct {
	clusterName string
	localPort   int
	server      *http.Server
	ctx         context.Context
	cancel      context.CancelFunc
}

type CreateClusterRequest struct {
	Name string `json:"name"`
}

type CreateClusterResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	ClusterID string `json:"cluster_id,omitempty"`
}

// ScaleAgent is the long-lived process state
type ScaleAgent struct {
	config   *Config
	clusters map[string]*ClusterInfo
	ctx      context.Context
	cancel   context.CancelFunc

	// HTTP endpoints
	httpServer     *http.Server
	httpServerOnce sync.Once

	// Connections/tunnels bookkeeping
	activeConnections     map[string]bool
	clusterAgentSessions  map[string]bool
	connMutex             sync.RWMutex
	caMutex               sync.RWMutex
	firstClusterConnected bool
	// Prevent duplicate connection attempts and add simple throttling
	connecting           map[string]bool
	lastConnectAttempt   map[string]time.Time
	lastCAConnectAttempt map[string]time.Time
	// Per-cluster cancel funcs to stop active tunnels on deletion
	connectCancels map[string]context.CancelFunc
	// Per-cluster feature flags and log throttling
	// Suppress noisy ConfigClient checks when cluster is already Active
	configClientSuppressed map[string]bool // key: clusterID
	// Last time we emitted a noisy info log per key (e.g., dupcon-<clusterName>)
	lastNoisyLog map[string]time.Time

	// Tokens, mock servers, etc.
	tokenCache     map[string]string
	mockServers    map[string]*http.Server
	portForwarders map[string]*PortForwarder
	nextPort       int
	nameCounters   map[string]int

	// Cert material used by mock API if/when needed
	mockCertPEM []byte

	// Simulated-vcluster manager for ultra-low memory clusters
	kwokManager *SimulatedVClusterManager
}
