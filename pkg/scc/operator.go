package scc

import (
	"context"
	"errors"
	"fmt"
	"github.com/rancher/rancher/pkg/generated/controllers/scc.cattle.io"
	"github.com/rancher/rancher/pkg/scc/controllers"
	"github.com/rancher/wrangler/v3/pkg/start"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"

	"github.com/rancher/rancher/pkg/wrangler"
)

type sccOperator struct {
	configMaps              v1core.ConfigMapController
	sccResourceFactory      *scc.Factory
	secrets                 v1core.SecretController
	systemInformation       *util.RancherSystemInfo
	systemRegistrationReady chan struct{}
}

func setup(wContext *wrangler.Context) (*sccOperator, error) {
	namespaces := wContext.Core.Namespace()
	kubeSystemNS, err := namespaces.Get("kube-system", metav1.GetOptions{})
	if err != nil {
		// fatal log here, because we need the kube-system ns UID while creating any backup file
		logrus.Fatalf("Error getting namespace kube-system %v", err)
	}

	rancherUuid := settings.InstallUUID.Get()
	if rancherUuid == "" {
		err := errors.New("no rancher uuid found")
		logrus.Fatalf("Error getting rancher uuid: %v", err)
		return nil, err
	}

	sccResources, err := scc.NewFactoryFromConfig(wContext.RESTConfig)
	if err != nil {
		logrus.Fatalf("Error getting scc resources: %v", err)
		return nil, err
	}

	// TODO(o&b): also get Node, Sockets, v-cpus, Clusters and watch those
	return &sccOperator{
		configMaps:         wContext.Core.ConfigMap(),
		sccResourceFactory: sccResources,
		secrets:            wContext.Core.Secret(),
		systemInformation: util.NewRancherSystemInfo(
			uuid.MustParse(rancherUuid),
			uuid.MustParse(string(kubeSystemNS.UID)),
			wContext,
		),
		systemRegistrationReady: make(chan struct{}),
	}, nil
}

func (so *sccOperator) waitForSystemReady(onSystemReady func()) {
	// Currently we only wait for ServerUrl not being empty, this is a good start as with out the URL we cannot start.
	// However, we should also consider other state that we "need" to register with SCC like metrics about nodes/clusters.
	if so.systemInformation.ServerUrl() != "" {
		close(so.systemRegistrationReady)
		return
	}
	logrus.Info("[scc-operator] Waiting for server-url to be ready")
	wait.Until(func() {
		if so.systemInformation.ServerUrl() != "" {
			logrus.Info("[scc-operator] can now start controllers; server URL is now ready.")
			close(so.systemRegistrationReady)
		} else {
			logrus.Info("[scc-operator] cannot start controllers yet; server URL is not ready.")
		}
	}, 15*time.Second, so.systemRegistrationReady)
	onSystemReady()
}

// maybeFirstInit will check if the initial `Registration` seeding values exist
// and if they need to be processed into a new `Registration` (used during first boot ever)
func (so *sccOperator) maybeFirstInit() (*v1.Registration, error) {
	logrus.Debug("[scc-operator] Running maybeFirstInit")
	if strings.EqualFold(settings.SCCFirstStart.Get(), "false") {
		logrus.Warn("Skipping the SCC controller first start; first start already completed previously.")
		return nil, nil
	}

	// Check if the `cattle-system:initial-scc-registration` ConfigMap exists
	// If it does not exist, then we skip initialization via ConfigMap and proceed to set SCCFirstStart to false
	configMap, err := so.configMaps.Get("cattle-system", "initial-scc-registration", metav1.GetOptions{})
	var newRegistration *v1.Registration
	if err != nil {
		if !apierrors.IsNotFound(err) {
			logrus.Errorf("Error getting cattle-system initial-scc-registration: %v", err)
			return nil, err
		}
		logrus.Warn("Cannot find initial-scc-registration configmap; it will be skipped")
	} else {
		secretRef, mode, err := util.ValidateInitializingConfigMap(configMap)
		if err != nil {
			logrus.Warn("Cannot validate initial-scc-registration configmap; it will be skipped")
		} else {
			newRegistration = &v1.Registration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "rancher-",
				},
				Spec: v1.RegistrationSpec{
					RegistrationRequest: &v1.RegistrationRequest{},
				},
			}
			newRegistration.Spec.Mode = *mode
			if *mode == v1.Online {
				newRegistration.Spec.RegistrationRequest.RegistrationCodeSecretRef = secretRef
			}

			_, err = so.sccResourceFactory.Scc().V1().Registration().Create(newRegistration)
			if err != nil {
				logrus.Errorf("Cannot create registration request; %s", err)
				return nil, err
			}
			_ = so.configMaps.Delete(configMap.Namespace, configMap.Name, &metav1.DeleteOptions{})
		}
	}

	// At very end, we will set it to false so this doesn't run again
	if !strings.EqualFold(settings.SCCFirstStart.Get(), "false") {
		if err := settings.SCCFirstStart.Set("false"); err != nil {
			return newRegistration, err
		}
	}

	return newRegistration, nil
}

func Setup(
	ctx context.Context,
	wContext *wrangler.Context,
) error {
	logrus.Debug("Starting SCC Operator")
	initOperator, err := setup(wContext)
	if err != nil {
		return fmt.Errorf("error setting up scc operator: %s", err.Error())
	}

	// Start goroutine to wait for systemRegistrationReady to complete; currently based on server-url only
	go func() {
		logrus.Debug("[scc-operator] Waiting to run first init until system is ready for registration")
		<-initOperator.systemRegistrationReady

		_, err = initOperator.maybeFirstInit()
		if err != nil {
			logrus.Errorf("error creating first-start `Registration`: %s", err.Error())
		}

		return
	}()

	go initOperator.waitForSystemReady(func() {
		controllers.Register(
			ctx,
			initOperator.sccResourceFactory.Scc().V1().Registration(),
			initOperator.secrets,
			initOperator.systemInformation,
		)

		if err := start.All(ctx, 2, initOperator.sccResourceFactory); err != nil {
			logrus.Errorf("error starting scc operator: %s", err.Error())
		}
	})

	if initOperator.systemRegistrationReady != nil {
		logrus.Info("[scc-operator] Initial setup initiated. When Server URL is configured full setup will complete.")
	}

	return nil
}
