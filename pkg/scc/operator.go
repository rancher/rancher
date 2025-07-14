package scc

import (
	"context"
	"errors"
	"fmt"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/telemetry"

	"github.com/rancher/rancher/pkg/scc/util/log"

	"github.com/pborman/uuid"
	k8sv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	devMode            bool
	log                log.StructuredLogger
	sccResourceFactory *scc.Factory
	secrets            corev1.SecretController
	rancherTelemetry   telemetry.TelemetryGatherer
}

func setup(wContext *wrangler.Context, logger log.StructuredLogger, infoProvider *systeminfo.InfoProvider) (*sccOperator, error) {
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
	infoProvider = infoProvider.SetUuids(parsedRancherUUID, parsedkubeSystemNSUID)

	rancherVersion := systeminfo.GetVersion()
	rancherTelemetry := telemetry.NewTelemetryGatherer(rancherVersion, wContext.Mgmt.Cluster().Cache(), wContext.Mgmt.Node().Cache())

	return &sccOperator{
		devMode:            consts.IsDevMode(),
		log:                logger,
		sccResourceFactory: sccResources,
		secrets:            wContext.Core.Secret(),
		rancherTelemetry:   rancherTelemetry,
	}, nil
}

// Setup initializes the SCC Operator and configures it to start when Rancher is in appropriate state.
func Setup(
	ctx context.Context,
	wContext *wrangler.Context,
) error {
	operatorLogger := log.NewLog()
	operatorLogger.Debug("Preparing to setup SCC operator")

	infoProvider := systeminfo.NewInfoProvider(
		wContext.Mgmt.Node().Cache(),
	)

	starter := sccStarter{
		log:                     operatorLogger.WithField("component", "scc-starter"),
		systemInfoProvider:      infoProvider,
		systemRegistrationReady: make(chan struct{}),
	}

	go starter.waitForSystemReady(func() {
		operatorLogger.Debug("Setting up SCC Operator")
		initOperator, err := setup(wContext, operatorLogger, infoProvider)
		if err != nil {
			starter.log.Errorf("error setting up scc operator: %s", err.Error())
		}

		controllers.Register(
			ctx,
			consts.DefaultSCCNamespace,
			initOperator.sccResourceFactory.Scc().V1().Registration(),
			initOperator.secrets,
			initOperator.rancherTelemetry,
			infoProvider,
		)

		if err := start.All(ctx, 2, initOperator.sccResourceFactory); err != nil {
			initOperator.log.Errorf("error starting operator: %s", err.Error())
		}
	})

	if starter.systemRegistrationReady != nil {
		operatorLogger.Info("SCC operator initialized; controllers waiting to start until system is ready")
	}

	return nil
}
