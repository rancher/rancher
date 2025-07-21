package telemetry

import (
	"fmt"
	"time"
)

const (
	SccSecretName      = "rancher-scc-telemetry"
	SccSecretNamespace = "cattle-system"

	archUnknown = "unknown"
)

type SccPayload struct {
	Version         string          `json:"version"`
	Subscription    SccSubscription `json:"subscription"`
	FeatureFlags    []string        `json:"feature_flags"`
	ManagedSystems  []SccSystem     `json:"managedSystems"`
	ManagedClusters []SccCluster    `json:"managedClusters"`
	Timestamp       time.Time       `json:"timestamp"`
}

type SccSubscription struct {
	InstallUUID string `json:"installuuid"`
	ClusterUUID string `json:"clusteruuid"`
	Product     string `json:"product"`
	Version     string `json:"version"`
	Arch        string `json:"arch"`
	Git         string `json:"git"`
	ServerURL   string `json:"server_url"`
}

type SccSystem struct {
	Arch   string `json:"arch"`
	Cpu    int    `json:"cpu"`
	Memory int    `json:"memory"`
	Count  int    `json:"count"`
}

type sccSystemKey struct {
	Arch   string
	Cpu    int
	Memory int
}

func (s sccSystemKey) Key() string {
	return fmt.Sprintf("%d-%d-%s", s.Cpu, s.Memory, s.Arch)
}

type SccCluster struct {
	Count    int  `json:"count"`
	Nodes    int  `json:"nodes"`
	Upstream bool `json:"upstream,omitempty"`
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
	systems := []SccSystem{}
	clusters := []SccCluster{}

	localCluster := telG.LocalClusterTelemetry()
	localNodeCount := 0
	for _, localNode := range localCluster.PerNodeTelemetry() {
		localNodeCount++
		cores, _ := localNode.CpuCores()
		mem, _ := localNode.MemoryCapacityBytes()
		k := sccSystemKey{
			Arch:   localNode.CpuArchitecture(),
			Cpu:    cores,
			Memory: bytesToMiBRounded(mem),
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
			Arch:   system.Arch,
			Cpu:    system.Cpu,
			Memory: system.Memory,
			Count:  count,
		})
	}

	for cl, count := range clustersMap {
		clusters = append(clusters, SccCluster{
			Nodes:    int(cl),
			Upstream: false,
			Count:    count,
		})
	}

	return &SccPayload{
		Version:         telG.RancherVersion(),
		FeatureFlags:    telG.FeatureFlags(),
		ManagedSystems:  systems,
		ManagedClusters: clusters,
		Subscription: SccSubscription{
			InstallUUID: telG.InstallUUID(),
			ClusterUUID: telG.ClusterUUID(),
			Version:     telG.RancherVersion(),
			Arch:        archUnknown,
			// TODO
			Product: "",
			// TODO
			Git:       "",
			ServerURL: telG.ServerURL(),
		},
		Timestamp: now,
	}, nil
}
