package systeminfo

import (
	"encoding/json"
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/google/uuid"
	"github.com/rancher/rancher/pkg/wrangler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type InfoExporter struct {
	InfoProvider *InfoProvider
	wContext     *wrangler.Context
}

func NewInfoExporter(infoProvider *InfoProvider, wContext *wrangler.Context) *InfoExporter {
	return &InfoExporter{
		InfoProvider: infoProvider,
		wContext:     wContext,
	}
}

func (e *InfoExporter) Provider() *InfoProvider {
	return e.InfoProvider
}

func (e *InfoExporter) preparedForSCC() ([]byte, error) {
	type RancherSCCInfo struct {
		UUID     uuid.UUID `json:"uuid"`
		Url      string    `json:"server_url"`
		Nodes    int       `json:"nodes"`
		Sockets  int       `json:"sockets"`
		Vcpus    int       `json:"vcpus"`
		Clusters int       `json:"clusters"`
		Version  string    `json:"version"`
	}

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

	sccInfo := &RancherSCCInfo{
		UUID:     e.InfoProvider.RancherUuid,
		Url:      ServerHostname(),
		Version:  e.InfoProvider.GetVersion(),
		Nodes:    nodeCount,
		Sockets:  socketsCount,
		Vcpus:    vcpusCount,
		Clusters: clusterCount,
	}

	return json.Marshal(sccInfo)
}

func (e *InfoExporter) PreparedForSCC() (registration.SystemInformation, error) {
	systemInfoMap := make(registration.SystemInformation)
	jsonInfo, err := e.preparedForSCC()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonInfo, &systemInfoMap)
	if err != nil {
		return nil, err
	}

	return systemInfoMap, nil
}

func (e *InfoExporter) PreparedForSCCOffline() ([]byte, error) {
	return e.preparedForSCC()
}
