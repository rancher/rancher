package telemetry

import (
	"fmt"
	"regexp"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/invopop/jsonschema"
)

var semVerCoreRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)

const (
	SccSecretName = "rancher-scc-telemetry"

	archUnknown              = "unknown"
	rancherProductIdentifier = "rancher-prime"
)

// SccPayload represents the canonical golang implementation of `schemas/scc-RMSSubscription.json`
type SccPayload struct {
	Version         string          `json:"version" jsonschema:"pattern=^\\d+\\.\\d+\\.\\d+$,description=Product Version normalized for SCC - must be semver. https://semver.org/"`
	Subscription    SccSubscription `json:"subscription"`
	FeatureFlags    []string        `json:"feature_flags,omitempty" jsonschema:"description=Feature flags enabled on RMS https://ranchermanager.docs.rancher.com/getting-started/installation-and-upgrade/installation-references/feature-flags"`
	ManagedSystems  []SccSystem     `json:"managedSystems" jsonschema:"description=Active systems under management and their details; to be expanded"`
	ManagedClusters []SccCluster    `json:"managedClusters"`
	Timestamp       time.Time       `json:"timestamp"`
}

type SccSubscription struct {
	InstallUUID string `json:"installuuid" jsonschema:"format=uuid,description=The UID of the k8s kube-system namespace."`
	ClusterUUID string `json:"clusteruuid" jsonschema:"format=uuid,description=Rancher's unique cluster identifier - based on InstallUUID setting."`
	Product     string `json:"product" jsonschema:"description=cpe of the product"`
	Version     string `json:"version" jsonschema:"description=Rancher's raw build version"`
	Arch        string `json:"arch"`
	Git         string `json:"git" jsonschema:"description=Rancher's build git SHA"`
	ServerURL   string `json:"server_url"`
}

type SccSystem struct {
	Arch     string `json:"arch"`
	Cpu      int    `json:"cpu" jsonschema:"minimum=0,description=https://kubernetes.io/docs/tasks/configure-pod-container/assign-cpu-resource/#cpu-units status.internalNodeStatus.capacity.cpu"`
	Memory   int    `json:"memory" jsonschema:"minimum=0,description=Capacity of the node(s): status.internalNodeStatus.capacity.memory (converted to MiB)"`
	Count    int    `json:"count" jsonschema:"minimum=1,default=1,description=Count of systems with these attributes (deduplication to shrink data size)"`
	Upstream bool   `json:"upstream,omitempty" jsonschema:"description=Identifies the cluster hosting RMS itself"`
}

// JSONSchemaExtend allows SccSystem to accept additional properties
func (SccSystem) JSONSchemaExtend(schema *jsonschema.Schema) {
	schema.AdditionalProperties = jsonschema.TrueSchema
}

type sccSystemKey struct {
	Arch     string
	Cpu      int
	Memory   int
	Upstream bool `json:"upstream,omitempty"`
}

type SccCluster struct {
	Count    int  `json:"count" jsonschema:"minimum=1,description=De-duplication of identical clusters"`
	Nodes    int  `json:"nodes" jsonschema:"minimum=0"`
	Upstream bool `json:"upstream,omitempty" jsonschema:"description=Identifies the cluster hosting RMS itself,default=false"`
}

// JSONSchemaExtend allows SccCluster to accept additional properties
func (SccCluster) JSONSchemaExtend(schema *jsonschema.Schema) {
	schema.AdditionalProperties = jsonschema.TrueSchema
}

const (
	KB  = 1024
	MiB = 1024 * KB // 1 MiB = 1,048,576 bytes
)

func bytesToMiBRounded(bytes int) int {
	if bytes <= 0 {
		return 0
	}
	return (bytes + MiB - 1) / MiB
}

type nodeCount int

func GenerateSCCPayload(telG RancherManagerTelemetry) (*SccPayload, error) {
	now := time.Now()
	systemsMap := map[sccSystemKey]int{}
	clustersMap := map[nodeCount]int{}
	var systems []SccSystem
	var clusters []SccCluster

	localCluster := telG.LocalClusterTelemetry()
	localNodeCount := 0
	for _, localNode := range localCluster.PerNodeTelemetry() {
		localNodeCount++
		cores, _ := localNode.CpuCores()
		mem, _ := localNode.MemoryCapacityBytes()
		k := sccSystemKey{
			Arch:     localNode.CpuArchitecture(),
			Cpu:      cores,
			Memory:   bytesToMiBRounded(mem),
			Upstream: true,
		}
		if _, ok := systemsMap[k]; !ok {
			systemsMap[k] = 0
		}
		systemsMap[k]++
	}

	clusters = append(clusters, SccCluster{
		Nodes:    localNodeCount,
		Upstream: true,
		Count:    1,
	})

	for _, cluster := range telG.PerManagedClusterTelemetry() {
		nCount := 0
		for _, node := range cluster.PerNodeTelemetry() {
			cores, _ := node.CpuCores()
			mem, _ := node.MemoryCapacityBytes()
			k := sccSystemKey{
				Arch:   node.CpuArchitecture(),
				Cpu:    cores,
				Memory: bytesToMiBRounded(mem),
			}
			if _, ok := systemsMap[k]; !ok {
				systemsMap[k] = 0
			}
			systemsMap[k]++
			nCount++
		}

		if _, ok := clustersMap[nodeCount(nCount)]; !ok {
			clustersMap[nodeCount(nCount)] = 0
		}
		clustersMap[nodeCount(nCount)]++

	}
	for system, count := range systemsMap {
		systems = append(systems, SccSystem{
			Arch:     system.Arch,
			Cpu:      system.Cpu,
			Memory:   system.Memory,
			Count:    count,
			Upstream: system.Upstream,
		})
	}

	for cl, count := range clustersMap {
		clusters = append(clusters, SccCluster{
			Nodes:    int(cl),
			Upstream: false,
			Count:    count,
		})
	}

	// Remove pre-release and build metadata from the version.
	// Try strict semver first (handles 1.2.0-alpha.4), then fall back to a
	// regex to handle non-standard prerelease formats like 1.2.0-alpha4.
	productVersion := telG.RancherVersion()
	if semVer, err := semver.StrictNewVersion(productVersion); err == nil {
		productVersion = fmt.Sprintf("%d.%d.%d", semVer.Major(), semVer.Minor(), semVer.Patch())
	} else if m := semVerCoreRe.FindStringSubmatch(productVersion); len(m) == 4 {
		productVersion = fmt.Sprintf("%s.%s.%s", m[1], m[2], m[3])
	}

	return &SccPayload{
		Version:         productVersion,
		FeatureFlags:    telG.FeatureFlags(),
		ManagedSystems:  systems,
		ManagedClusters: clusters,
		Subscription: SccSubscription{
			InstallUUID: telG.InstallUUID(),
			ClusterUUID: telG.ClusterUUID(),
			Version:     telG.RancherVersion(),
			Arch:        archUnknown,
			Product:     rancherProductIdentifier,
			Git:         telG.RancherGitHash(),
			ServerURL:   telG.ServerURL(),
		},
		Timestamp: now,
	}, nil
}

// GenerateSccSchema generates the JSON schema for SccPayload
func GenerateSccSchema() (*jsonschema.Schema, error) {
	reflector := jsonschema.Reflector{
		ExpandedStruct: true,
		DoNotReference: true,
	}
	schema := reflector.Reflect(&SccPayload{})
	schema.Title = "RMSSubscription"
	schema.Definitions = nil
	return schema, nil
}
