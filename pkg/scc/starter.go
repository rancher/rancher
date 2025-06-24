package scc

import (
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"github.com/rancher/rancher/pkg/scc/util/log"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"
)

type sccStarter struct {
	log                     log.StructuredLogger
	systemInfoProvider      *systeminfo.InfoProvider
	systemRegistrationReady chan struct{}
}

func (s *sccStarter) waitForSystemReady(onSystemReady func()) {
	// Currently we only wait for ServerUrl not being empty, this is a good start as without the URL we cannot start.
	// However, we should also consider other state that we "need" to register with SCC like metrics about nodes/clusters.
	// TODO: expand wait to include verifying at least local cluster is ready too - this prevents issues with offline clusters
	defer onSystemReady()
	if systeminfo.IsServerUrlReady() &&
		(s.systemInfoProvider != nil && s.systemInfoProvider.IsLocalReady()) {
		close(s.systemRegistrationReady)
		return
	}
	s.log.Info("Waiting for server-url and/or local cluster to be ready")
	wait.Until(func() {
		// Todo: also wait for local cluster ready
		if systeminfo.IsServerUrlReady() &&
			(s.systemInfoProvider != nil && s.systemInfoProvider.IsLocalReady()) {
			s.log.Info("can now start controllers; server URL and local cluster are now ready.")
			close(s.systemRegistrationReady)
		} else {
			s.log.Info("cannot start controllers yet; server URL and/or local cluster are not ready.")
		}
	}, 15*time.Second, s.systemRegistrationReady)
}
