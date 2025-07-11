package suseconnect

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"github.com/rancher/rancher/pkg/scc/util/log"

	"github.com/SUSE/connect-ng/pkg/connection"
	"github.com/SUSE/connect-ng/pkg/registration"
)

func sccContextLogger() log.StructuredLogger {
	logBuilder := &log.Builder{
		SubComponent: "suse-connect",
	}
	return logBuilder.ToLogger()
}

type SccWrapper struct {
	credentials connection.Credentials
	conn        *connection.ApiConnection
	registered  *bool // only used by online mode
	systemInfo  *systeminfo.InfoExporter
}

func DefaultConnectionOptions() connection.Options {
	// So this doesn't necessarily mean these have to match Rancher on the cluster.
	// Rather the details about the HTTP client talking to SCC
	return connection.DefaultOptions("rancher-scc-integration", "0.0.1", "en_US")
}

func OnlineRancherConnection(credentials connection.Credentials, systemInfo *systeminfo.InfoExporter, url string) SccWrapper {
	if credentials == nil {
		panic("credentials must be set")
	}

	registered := false
	if credentials.HasAuthentication() {
		registered = true
	}

	options := DefaultConnectionOptions()

	if url != "" {
		options.URL = url
	}

	return SccWrapper{
		credentials: credentials,
		conn:        connection.New(options, credentials),
		registered:  &registered,
		systemInfo:  systemInfo,
	}
}

func OfflineRancherRegistration(systemInfo *systeminfo.InfoExporter) SccWrapper {
	return SccWrapper{
		systemInfo: systemInfo,
	}
}

type RegistrationSystemId int

func (id RegistrationSystemId) Int() int {
	return int(id)
}

func (id RegistrationSystemId) Ptr() *int {
	i := int(id)
	return &i
}

// Define constant values for empty and error
const (
	EmptyRegistrationSystemId     RegistrationSystemId = 0  // Used if an error happened before registration
	ErrorRegistrationSystemId     RegistrationSystemId = -1 // Used when error is related to registration
	KeepAliveRegistrationSystemId RegistrationSystemId = -2 // Indicates the Registration was handled via keepalive instead
	OfflineRegistrationSystemId   RegistrationSystemId = -3
)

func (sw *SccWrapper) SystemRegistration(regCode string) (RegistrationSystemId, error) {
	// 1 collect system info
	systemInfo := sw.systemInfo
	preparedSystemInfo, err := systemInfo.PreparedForSCC()
	if err != nil {
		return EmptyRegistrationSystemId, err
	}

	id, regErr := registration.Register(sw.conn, regCode, systeminfo.ServerHostname(), preparedSystemInfo, registration.NoExtraData)
	if regErr != nil {
		return ErrorRegistrationSystemId, errors.Wrap(regErr, "Cannot register system to SCC")
	}

	return RegistrationSystemId(id), nil
}

func (sw *SccWrapper) PrepareOfflineRegistrationRequest() (*registration.OfflineRequest, error) {
	identifier, version, arch := sw.systemInfo.GetProductIdentifier()
	rancherUuid := sw.systemInfo.RancherUuid()

	// 1 collect system info
	preparedSystemInfo, err := sw.systemInfo.PreparedForSCC()
	if err != nil {
		return nil, err
	}

	return registration.BuildOfflineRequest(identifier, version, arch, rancherUuid.String(), preparedSystemInfo), nil
}

func (sw *SccWrapper) KeepAlive() error {
	// 1 collect system info
	systemInfo := sw.systemInfo
	preparedSystemInfo, err := systemInfo.PreparedForSCC()
	if err != nil {
		return err
	}
	// 2 call Status
	status, statusErr := registration.Status(sw.conn, systeminfo.ServerHostname(), preparedSystemInfo, registration.NoExtraData)
	if status != registration.Registered {
		return fmt.Errorf("trying to send keepalive on a system that is not yet registered. register this system first: %v", statusErr)
	}
	// 3 verify response says we're registered still
	return statusErr
}

func (sw *SccWrapper) RegisterOrKeepAlive(regCode string) (RegistrationSystemId, error) {
	if *sw.registered {
		return KeepAliveRegistrationSystemId, sw.KeepAlive()
	}

	return sw.SystemRegistration(regCode)
}

func (sw *SccWrapper) Activate(regCode string) (*registration.Metadata, *registration.Product, error) {
	identifier, version, arch := sw.systemInfo.GetProductIdentifier()
	metaData, product, err := registration.Activate(sw.conn, identifier, version, arch, regCode)
	if err != nil {
		return nil, nil, err
	}

	return metaData, product, err
}

func (sw *SccWrapper) ActivationStatus() ([]*registration.Activation, error) {
	activations, err := registration.FetchActivations(sw.conn)
	if err != nil {
		return nil, err
	}
	return activations, nil
}

func (sw *SccWrapper) ProductInfo() (*registration.Product, error) {
	identifier, version, arch := sw.systemInfo.GetProductIdentifier()
	return registration.FetchProductInfo(sw.conn, identifier, version, arch)
}

func (sw *SccWrapper) Deregister() error {
	return registration.Deregister(sw.conn)
}
