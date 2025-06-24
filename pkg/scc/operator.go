package scc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/telemetry"

	"github.com/rancher/rancher/pkg/scc/util/log"

	"github.com/pborman/uuid"
	k8sv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io"
	"github.com/rancher/rancher/pkg/scc/controllers"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/start"
)

type sccOperator struct {
	devMode                 bool
	log                     log.StructuredLogger
	sccResourceFactory      *scc.Factory
	secrets                 corev1.SecretController
	systemInfoProvider      *systeminfo.InfoProvider
	systemInfoExporter      *systeminfo.InfoExporter
	systemRegistrationReady chan struct{}
}

func setup(wContext *wrangler.Context, logger log.StructuredLogger) (*sccOperator, error) {
	namespaces := wContext.Core.Namespace()
	var kubeSystemNS *k8sv1.Namespace

	kubeNsErr := retry.OnError(
		retry.DefaultRetry,
		func(err error) bool {
			return apierrors.IsNotFound(err)
		},
		func() error {
			maybeNs, err := namespaces.Get("kube-system", metav1.GetOptions{})
			if err != nil {
				return err
			}

			kubeSystemNS = maybeNs
			return nil
		},
	)

	if kubeNsErr != nil {
		return nil, fmt.Errorf("failed to get kube-system namespace: %v", kubeNsErr)
	}

	rancherUuid := settings.InstallUUID.Get()
	if rancherUuid == "" {
		err := errors.New("no rancher uuid found")
		logger.Fatalf("Error getting rancher uuid: %v", err)
		return nil, err
	}

	sccResources, err := scc.NewFactoryFromConfig(wContext.RESTConfig)
	if err != nil {
		logger.Fatalf("Error getting scc resources: %v", err)
		return nil, err
	}
	// Validate that the UUID is in correct format
	parsedRancherUUID := uuid.Parse(rancherUuid)
	parsedkubeSystemNSUID := uuid.Parse(string(kubeSystemNS.UID))

	if parsedRancherUUID == nil || parsedkubeSystemNSUID == nil {
		return nil, fmt.Errorf("invalid UUID format: rancherUuid=%s, kubeSystemNS.UID=%s", rancherUuid, string(kubeSystemNS.UID))
	}
	infoProvider := systeminfo.NewInfoProvider(
		parsedRancherUUID,
		parsedkubeSystemNSUID,
		wContext.Mgmt.Node().Cache(),
	)

	rancherVersion := systeminfo.GetVersion()
	rancherTelemetry := telemetry.NewTelemetryGatherer(rancherVersion, wContext.Mgmt.Cluster().Cache(), wContext.Mgmt.Node().Cache())

	return &sccOperator{
		devMode:                 consts.IsDevMode(),
		log:                     logger,
		sccResourceFactory:      sccResources,
		secrets:                 wContext.Core.Secret(),
		systemInfoProvider:      infoProvider,
		systemInfoExporter:      systeminfo.NewInfoExporter(infoProvider, rancherTelemetry),
		systemRegistrationReady: make(chan struct{}),
	}, nil
}

func (so *sccOperator) waitForSystemReady(onSystemReady func()) {
	// Currently we only wait for ServerUrl not being empty, this is a good start as without the URL we cannot start.
	// However, we should also consider other state that we "need" to register with SCC like metrics about nodes/clusters.
	// TODO: expand wait to include verifying at least local cluster is ready too - this prevents issues with offline clusters
	defer onSystemReady()
	if systeminfo.IsServerUrlReady() &&
		(so.systemInfoProvider != nil && so.systemInfoProvider.IsLocalReady()) {
		close(so.systemRegistrationReady)
		return
	}
	so.log.Info("Waiting for server-url and/or local cluster to be ready")
	wait.Until(func() {
		// Todo: also wait for local cluster ready
		if systeminfo.IsServerUrlReady() {
			so.log.Info("can now start controllers; server URL and local cluster are now ready.")
			close(so.systemRegistrationReady)
		} else {
			so.log.Info("cannot start controllers yet; server URL and/or local cluster are not ready.")
		}
	}, 15*time.Second, so.systemRegistrationReady)
}

// Setup initializes the SCC Operator and configures it to start when Rancher is in appropriate state.
func Setup(
	ctx context.Context,
	wContext *wrangler.Context,
) error {
	operatorLogger := log.NewLog()

	operatorLogger.Debug("Setting up SCC Operator")
	initOperator, err := setup(wContext, operatorLogger)
	if err != nil {
		return fmt.Errorf("error setting up scc operator: %s", err.Error())
	}

	// Because the controller `Register` call will start activation refresh timers,
	// we need to wait for the system to be ready before starting the controller.
	// Doing it this way is more simple than tying the controller starting those timers to `systemRegistrationReady`
	go initOperator.waitForSystemReady(func() {
		controllers.Register(
			ctx,
			consts.DefaultSCCNamespace,
			initOperator.sccResourceFactory.Scc().V1().Registration(),
			initOperator.secrets,
			initOperator.systemInfoExporter,
		)

		if err := start.All(ctx, 2, initOperator.sccResourceFactory); err != nil {
			initOperator.log.Errorf("error starting operator: %s", err.Error())
		}
	})

	if initOperator.systemRegistrationReady != nil {
		initOperator.log.Info("SCC operator initialized; controllers waiting to start until system is ready")
	}

	return nil
}
