package systeminfo

import (
	"encoding/json"
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/pborman/uuid"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/rancher/pkg/telemetry"
	"k8s.io/client-go/util/retry"
)

type InfoExporter struct {
	infoProvider *InfoProvider
	tel          telemetry.TelemetryGatherer
	isLocalReady bool
}

type RancherSCCInfo struct {
	UUID             uuid.UUID `json:"uuid"`
	RancherUrl       string    `json:"server_url"`
	Nodes            int       `json:"nodes"`
	Sockets          int       `json:"sockets"`
	Clusters         int       `json:"clusters"`
	Version          string    `json:"version"`
	CpuCores         int       `json:"vcpus"`
	MemoryBytesTotal int       `json:"mem_total"`
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
	rancherTelemetry telemetry.TelemetryGatherer,
) *InfoExporter {
	return &InfoExporter{
		infoProvider: infoProvider,
		tel:          rancherTelemetry,
		isLocalReady: false,
	}
}

func (e *InfoExporter) Provider() *InfoProvider {
	return e.infoProvider
}

// GetProductIdentifier returns a triple of product ID, version and CPU architecture
func (e *InfoExporter) GetProductIdentifier() (string, string, string) {
	return e.infoProvider.GetProductIdentifier()
}

func (e *InfoExporter) RancherUuid() uuid.UUID {
	return e.infoProvider.rancherUuid
}

func (e *InfoExporter) ClusterUuid() uuid.UUID {
	return e.infoProvider.clusterUuid
}

func (e *InfoExporter) preparedForSCC() RancherSCCInfo {
	var exporter telemetry.RancherManagerTelemetry
	// TODO(dan): this logic might need some tweaking
	if err := retry.OnError(retry.DefaultRetry, func(_ error) bool {
		return true
	}, func() error {
		exp, err := e.tel.GetClusterTelemetry()
		if err != nil {
			return err
		}
		exporter = exp
		return nil
	}); err != nil {
		// TODO(dan) : should probably surface an error here and handle it
		return RancherSCCInfo{}
	}

	nodeCount := 0
	totalCpuCores := int(0)
	// note: this will only correctly report up to ~9 exabytes of memory,
	// which should be fine
	totalMemBytes := int(0)
	clusterCount := exporter.ManagedClusterCount()

	// local cluster metrics
	localClT := exporter.LocalClusterTelemetry()
	localCores, _ := localClT.CpuCores()
	localMem, _ := localClT.MemoryCapacityBytes()
	totalCpuCores += localCores
	totalMemBytes += localMem
	for _, _ = range localClT.PerNodeTelemetry() {
		nodeCount++
	}

	// managed cluster metrics
	for _, clT := range exporter.PerManagedClusterTelemetry() {
		cores, _ := clT.CpuCores()
		totalCpuCores += cores
		memBytes, _ := clT.MemoryCapacityBytes()
		totalMemBytes += memBytes
		for _, _ = range clT.PerNodeTelemetry() {
			nodeCount++
		}
	}

	return RancherSCCInfo{
		UUID:             e.infoProvider.rancherUuid,
		RancherUrl:       ServerUrl(),
		Version:          exporter.RancherVersion(),
		Nodes:            nodeCount,
		Sockets:          0,
		Clusters:         clusterCount,
		CpuCores:         totalCpuCores,
		MemoryBytesTotal: util.BytesToMiBRounded(totalMemBytes),
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

// PrepareOfflineRegistrationRequest returns a RancherOfflineRequestEncoded just to delineate between other []byte types,
// and to show connection to the original data structure it came from
func (e *InfoExporter) PrepareOfflineRegistrationRequest() (RancherOfflineRequestEncoded, error) {
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
