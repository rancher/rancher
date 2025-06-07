package systeminfo

import (
	"encoding/json"
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/google/uuid"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/rancher/pkg/wrangler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type InfoExporter struct {
	InfoProvider *InfoProvider
	wContext     *wrangler.Context
}

type RancherSCCInfo struct {
	UUID       uuid.UUID `json:"uuid"`
	RancherUrl string    `json:"server_url"`
	Nodes      int       `json:"nodes"`
	Sockets    int       `json:"sockets"`
	Clusters   int       `json:"clusters"`
	Version    string    `json:"version"`
}

type RancherOfflineRequest struct {
	UUID       uuid.UUID `json:"uuid"`
	RancherUrl string    `json:"server_url"`
	Nodes      int       `json:"nodes"`
	Sockets    int       `json:"sockets"`
	VCPUs      int       `json:vcpus`
	Clusters   int       `json:"clusters"`
	Version    string    `json:"version"`
}

type RancherOfflineRequestEncoded []byte

func NewInfoExporter(infoProvider *InfoProvider, wContext *wrangler.Context) *InfoExporter {
	return &InfoExporter{
		InfoProvider: infoProvider,
		wContext:     wContext,
	}
}

func (e *InfoExporter) Provider() *InfoProvider {
	return e.InfoProvider
}

func (e *InfoExporter) preparedForSCC() RancherSCCInfo {
	// Fetch current node count
	nodeCount := 0
	socketsCount := 1 // TODO: i don't think rancher exposes this...because k8s doesnt
	vcpusCount := 0
	mgmtNodes, nodesErr := e.wContext.Mgmt.Node().List("local", metav1.ListOptions{})
	if nodesErr == nil {
		nodeCount = len(mgmtNodes.Items)
		for _, node := range mgmtNodes.Items {
			cpuCores := node.Status.InternalNodeStatus.Capacity.Cpu()
			if cpuCores != nil {
				vcpusCount += cpuCores.Size()
			}
		}
	}

	// Fetch current cluster count
	clusterCount := 0
	clusterList, clusterErr := e.wContext.Mgmt.Cluster().List(metav1.ListOptions{})
	if clusterErr == nil {
		clusterCount = len(clusterList.Items)
	}

	// TODO: collect and organize downstream counts

	return RancherSCCInfo{
		UUID:       e.InfoProvider.RancherUuid,
		RancherUrl: ServerHostname(),
		Version:    e.InfoProvider.GetVersion(),
		Nodes:      nodeCount,
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

	offlinePrepared := RancherOfflineRequest{
		UUID:       sccPreparedInfo.UUID,
		RancherUrl: sccPreparedInfo.RancherUrl,
		Nodes:      sccPreparedInfo.Nodes,
		// TODO: unsure what to do here for offline since we don't have metrics on both sockets and vCPU
		Sockets:  sccPreparedInfo.Sockets,
		VCPUs:    sccPreparedInfo.Sockets,
		Clusters: sccPreparedInfo.Clusters,
		Version:  sccPreparedInfo.Version,
	}

	offlineJson, jsonErr := json.Marshal(offlinePrepared)
	if jsonErr != nil {
		return nil, jsonErr
	}

	return util.JSONToBase64(offlineJson)
}
