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
	return log.NewLog().WithField("subcomponent", "suse-connect")
}

type SccWrapper struct {
	credentials connection.Credentials
	conn        *connection.ApiConnection
	registered  bool
	systemInfo  *systeminfo.InfoExporter
}

func DefaultConnectionOptions() connection.Options {
	// TODO(scc): I believe this is creating the options for the API "app"
	// So this doesn't necessarily mean these have to match Rancher on the cluster.
	// Rather the details about the HTTP client talking to SCC
	return connection.DefaultOptions("rancher-scc-integration", "0.0.1", "en_US")
}

func DefaultRancherConnection(credentials connection.Credentials, systemInfo *systeminfo.InfoExporter) SccWrapper {
	if credentials == nil {
		panic("credentials must be set")
	}
	options := DefaultConnectionOptions()
	// TODO(scc): options in CLI tool set the base URL, shouldn't we do that too? (for SMT/RMT support)

	registered := false
	if credentials.HasAuthentication() {
		registered = true
	}

	return SccWrapper{
		credentials: credentials,
		conn:        connection.New(options, credentials),
		registered:  registered,
		systemInfo:  systemInfo,
	}
}

type RegistrationSystemId int

// Define constant values for empty and error
const (
	EmptyRegistrationSystemId     RegistrationSystemId = 0  // Used if an error happened before registration
	ErrorRegistrationSystemId     RegistrationSystemId = -1 // Used when error is related to registration
	KeepAliveRegistrationSystemId RegistrationSystemId = -2 // Indicates the Registration was handled via keepalive instead
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
	if sw.registered {
		return KeepAliveRegistrationSystemId, sw.KeepAlive()
	}

	return sw.SystemRegistration(regCode)
}

func (sw *SccWrapper) Activate(identifier string, version string, arch string, regCode string) (*registration.Metadata, *registration.Product, error) {
	metaData, product, err := registration.Activate(sw.conn, identifier, version, arch, regCode)
	if err != nil {
		return nil, nil, err
	}

	return metaData, product, err
}

func (sw *SccWrapper) Deregister() error {
	return registration.Deregister(sw.conn)
}
