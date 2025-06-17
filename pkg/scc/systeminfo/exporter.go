package systeminfo

import (
	"encoding/json"
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/pborman/uuid"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/rancher/pkg/wrangler"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type InfoExporter struct {
	infoProvider *InfoProvider

	clusterCache   v3.ClusterCache
	namespaceCache v1core.NamespaceCache
	isLocalReady   bool
}

type RancherSCCInfo struct {
	UUID       uuid.UUID `json:"uuid"`
	RancherUrl string    `json:"server_url"`
	Nodes      int       `json:"nodes"`
	Sockets    int       `json:"sockets"`
	Clusters   int       `json:"clusters"`
	Version    string    `json:"version"`
}

type ProductTriplet struct {
	Identifier string `json:"identifier"`
	Version    string `json:"version"`
	Arch       string `json:"arch"`
}

type RancherOfflineRequest struct {
	Product ProductTriplet `json:"product"`

	UUID       uuid.UUID `json:"uuid"`
	RancherUrl string    `json:"server_url"`
}

type RancherOfflineRequestEncoded []byte

func NewInfoExporter(
	infoProvider *InfoProvider,
	wContext *wrangler.Context,
) *InfoExporter {
	return &InfoExporter{
		infoProvider:   infoProvider,
		clusterCache:   wContext.Mgmt.Cluster().Cache(),
		namespaceCache: wContext.Core.Namespace().Cache(),
		isLocalReady:   false,
	}
}

func (e *InfoExporter) Provider() *InfoProvider {
	return e.infoProvider
}

func (e *InfoExporter) preparedForSCC() RancherSCCInfo {
	// Fetch current node count
	totalNodeCount := 0
	localNodeCount := 0
	vcpusCount := 0
	// TODO: i don't think rancher exposes this...because k8s doesnt
	socketsCount := 1

	localNodes, nodesErr := e.infoProvider.nodeCache.List("local", labels.Everything())
	if nodesErr != nil {
		localNodeCount += len(localNodes)
	}

	totalNodeCount += localNodeCount

	namespaces, err := e.namespaceCache.List(labels.Everything())
	if err != nil {
		panic(err)
	}

	for _, ns := range namespaces {
		if ns.Name == "local" {
			continue
		}
		nodes, err := e.infoProvider.nodeCache.List(ns.Name, labels.Everything())
		if err != nil {
			panic(err)
		}

		totalNodeCount += len(nodes)
		for _, node := range nodes {
			cpuCores := node.Status.InternalNodeStatus.Capacity.Cpu()
			if cpuCores != nil {
				vcpusCount += cpuCores.Size()
			}
		}
	}

	// Fetch current cluster count
	clusterCount := 0
	clusterList, err := e.clusterCache.List(labels.Everything())
	if err != nil {
		panic(err)
	}
	clusterCount = len(clusterList)

	// TODO: collect and organize downstream counts

	return RancherSCCInfo{
		UUID:       e.infoProvider.RancherUuid,
		RancherUrl: ServerUrl(),
		Version:    e.infoProvider.GetVersion(),
		Nodes:      totalNodeCount,
		Sockets:    socketsCount,
		Clusters:   clusterCount,
	}
}

func (e *InfoExporter) PreparedForSCC() (registration.SystemInformation, error) {
	sccPreparedInfo := e.preparedForSCC()
	sccJson, jsonErr := json.Marshal(sccPreparedInfo)
	if jsonErr != nil {
		return nil, jsonErr
	}

	systemInfoMap := make(registration.SystemInformation)
	err := json.Unmarshal(sccJson, &systemInfoMap)
	if err != nil {
		return nil, err
	}

	return systemInfoMap, nil
}

// PreparedForSCCOffline returns a RancherOfflineRequestEncoded just to delineate between other []byte types,
// and to show connection to the original data structure it came from
func (e *InfoExporter) PreparedForSCCOffline() (RancherOfflineRequestEncoded, error) {
	sccPreparedInfo := e.preparedForSCC()

	identifier, version, arch := e.infoProvider.GetProductIdentifier()

	offlinePrepared := RancherOfflineRequest{
		UUID:       sccPreparedInfo.UUID,
		RancherUrl: sccPreparedInfo.RancherUrl,
		Product: ProductTriplet{
			Identifier: identifier,
			Version:    version,
			Arch:       arch,
		},
	}

	offlineJson, jsonErr := json.Marshal(offlinePrepared)
	if jsonErr != nil {
		return nil, jsonErr
	}

	return util.JSONToBase64(offlineJson)
}
