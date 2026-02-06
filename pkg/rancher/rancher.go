package rancher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	responsewriter "github.com/rancher/apiserver/pkg/middleware"
	normanStoreProxy "github.com/rancher/norman/store/proxy"
	"github.com/rancher/rancher/pkg/api/norman/customization/kontainerdriver"
	steveapi "github.com/rancher/rancher/pkg/api/steve"
	"github.com/rancher/rancher/pkg/api/steve/aggregation"
	"github.com/rancher/rancher/pkg/api/steve/proxy"
	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/cleanup"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/local/pbkdf2"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/clusterrouter"
	auditlogcontroller "github.com/rancher/rancher/pkg/controllers/auditlog/auditpolicy"
	"github.com/rancher/rancher/pkg/controllers/dashboard"
	"github.com/rancher/rancher/pkg/controllers/dashboard/apiservice"
	"github.com/rancher/rancher/pkg/controllers/dashboard/plugin"
	"github.com/rancher/rancher/pkg/controllers/dashboardapi"
	managementauth "github.com/rancher/rancher/pkg/controllers/management/auth"
	"github.com/rancher/rancher/pkg/controllers/nodedriver"
	provisioningv2 "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	"github.com/rancher/rancher/pkg/crds"
	dashboardcrds "github.com/rancher/rancher/pkg/crds/dashboard"
	dashboarddata "github.com/rancher/rancher/pkg/data/dashboard"
	"github.com/rancher/rancher/pkg/ext"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/generated/controllers/auditlog.cattle.io"
	mgmntv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainerdrivermetadata"
	"github.com/rancher/rancher/pkg/multiclustermanager"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/scc"
	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/telemetry"
	telemetrycontrollers "github.com/rancher/rancher/pkg/telemetry/controllers"
	"github.com/rancher/rancher/pkg/telemetry/initcond"
	"github.com/rancher/rancher/pkg/tls"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/ui"
	"github.com/rancher/rancher/pkg/utils"
	"github.com/rancher/rancher/pkg/websocket"
	"github.com/rancher/rancher/pkg/wrangler"
	aggregation2 "github.com/rancher/steve/pkg/aggregation"
	steveauth "github.com/rancher/steve/pkg/auth"
	steveserver "github.com/rancher/steve/pkg/server"
	"github.com/rancher/steve/pkg/sqlcache/informer/factory"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/k8scheck"
	"github.com/rancher/wrangler/v3/pkg/unstructured"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gopkg.in/natefinch/lumberjack.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
	k8dynamic "k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

const (
	encryptionConfigUpdate        = "provisioner.cattle.io/encrypt-migrated"
	defaultSQLCacheMaxEventsCount = 1000
)

type Options struct {
	ACMEDomains                    cli.StringSlice
	AddLocal                       string
	Embedded                       bool
	BindHost                       string
	HTTPListenPort                 int
	HTTPSListenPort                int
	K8sMode                        string
	Debug                          bool
	Trace                          bool
	NoCACerts                      bool
	AuditLogPath                   string
	AuditLogMaxage                 int
	AuditLogMaxsize                int
	AuditLogMaxbackup              int
	AuditLogLevel                  int
	AuditLogEnabled                bool
	Features                       string
	ClusterRegistry                string
	AggregationRegistrationTimeout time.Duration
}

type Rancher struct {
	Auth             steveauth.Middleware
	Handler          http.Handler
	Wrangler         *wrangler.Context
	Steve            *steveserver.Server
	auditLog         *audit.Writer
	telemetryManager telemetry.TelemetryExporterManager
	authServer       *auth.Server
	opts             *Options

	aggregationRegistrationTimeout time.Duration
	kubeAggregationReadyChan       <-chan struct{}
}

func New(ctx context.Context, clientConfg clientcmd.ClientConfig, opts *Options) (*Rancher, error) {
	var (
		authServer *auth.Server
	)

	if opts == nil {
		opts = &Options{}
	}

	restConfig, err := clientConfg.ClientConfig()
	if err != nil {
		return nil, err
	}

	restConfig, err = setupAndValidationRESTConfig(ctx, restConfig)
	if err != nil {
		return nil, err
	}

	// Run the encryption migration before any controllers run otherwise the fields will be dropped
	if err := migrateEncryptionConfig(ctx, restConfig); err != nil {
		return nil, err
	}

	wranglerContext, err := wrangler.NewPrimaryContext(ctx, clientConfg, restConfig)
	if err != nil {
		return nil, err
	}

	// Check for deprecated RKE1 resources in the cluster
	if err := validateRKE1Resources(wranglerContext); err != nil {
		return nil, fmt.Errorf("rke1 pre-upgrade validation failed: %w", err)
	}

	if err := dashboarddata.EarlyData(ctx, wranglerContext.K8s); err != nil {
		return nil, err
	}

	if opts.Embedded {
		if err := setupRancherService(ctx, restConfig, opts.HTTPSListenPort); err != nil {
			return nil, err
		}
		if err := bumpRancherWebhookIfNecessary(ctx, restConfig); err != nil {
			return nil, err
		}
	}

	wranglerContext.MultiClusterManager = newMCM(wranglerContext, opts)

	// Initialize Features as early as possible
	if err := dashboardcrds.CreateFeatureCRD(ctx, restConfig); err != nil {
		return nil, err
	}

	if err := features.MigrateFeatures(wranglerContext.Mgmt.Feature(), wranglerContext.CRD.CustomResourceDefinition(), wranglerContext.Mgmt.Cluster()); err != nil {
		return nil, fmt.Errorf("migrating features: %w", err)
	}
	features.InitializeFeatures(wranglerContext.Mgmt.Feature(), opts.Features)

	kontainerdriver.RegisterIndexers(wranglerContext)
	managementauth.RegisterWranglerIndexers(wranglerContext)

	if features.ProvisioningV2.Enabled() {
		// ensure indexers are registered for all replicas
		provisioningv2.RegisterIndexers(wranglerContext)
	}

	clientSet, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create new clientset: %w", err)
	}

	// ensure migrated CRDs
	if err := crds.EnsureRequired(ctx, clientSet.ApiextensionsV1().CustomResourceDefinitions()); err != nil {
		return nil, fmt.Errorf("failed to ensure CRDs: %w", err)
	}

	// install all non migrated CRDs
	if err := dashboardcrds.Create(ctx, restConfig); err != nil {
		return nil, fmt.Errorf("failed to create CRDs: %w", err)
	}

	if features.MCM.Enabled() && !features.Fleet.Enabled() {
		logrus.Info("fleet can't be turned off when MCM is enabled. Turning on fleet feature")
		if err := features.SetFeature(wranglerContext.Mgmt.Feature(), features.Fleet.Name(), true); err != nil {
			return nil, err
		}
	}

	if features.Auth.Enabled() {
		sc, err := config.NewScaledContext(*restConfig, nil)
		if err != nil {
			return nil, err
		}

		sc.Wrangler = wranglerContext

		sc.UserManager, err = common.NewUserManagerNoBindings(wranglerContext)
		if err != nil {
			return nil, err
		}

		sc.ClientGetter, err = normanStoreProxy.NewClientGetterFromConfig(*restConfig)
		if err != nil {
			return nil, err
		}

		tokenAuthenticator := requests.NewAuthenticator(ctx, clusterrouter.GetClusterID, sc)

		authServer, err = auth.NewServer(ctx, wranglerContext, sc, tokenAuthenticator)
		if err != nil {
			return nil, err
		}
	} else {
		authServer, err = auth.NewAlwaysAdmin()
		if err != nil {
			return nil, err
		}
	}

	steveControllers, err := steveserver.NewController(restConfig, &generic.FactoryOptions{SharedControllerFactory: wranglerContext.SharedControllerFactory})
	if err != nil {
		return nil, err
	}

	if ext.RDPEnabled() {
		if err := ext.RDPStart(ctx, restConfig, wranglerContext); err != nil {
			return nil, err
		}
	}

	extensionOpts := ext.DefaultOptions()

	extensionAPIServer, err := ext.NewExtensionAPIServer(ctx, wranglerContext, extensionOpts)
	if err != nil {
		return nil, fmt.Errorf("extension api server: %w", err)
	}

	skipWaitForExtensionAPIServer := ext.AggregationPreCheck(wranglerContext.API.APIService())

	var kubeAggregationReadyChan <-chan struct{}
	if !skipWaitForExtensionAPIServer && extensionAPIServer != nil {
		kubeAggregationReadyChan = extensionAPIServer.Registered()
	}

	gcInterval, gcKeepCount := getSQLCacheGCValues(wranglerContext)
	steve, err := steveserver.New(ctx, restConfig, &steveserver.Options{
		ServerVersion:   settings.ServerVersion.Get(),
		Controllers:     steveControllers,
		AccessSetLookup: wranglerContext.ASL,
		AuthMiddleware:  steveauth.ExistingContext,
		Next:            ui.New(wranglerContext.Mgmt.Preference().Cache(), wranglerContext.Mgmt.ClusterRegistrationToken().Cache()),
		ClusterRegistry: opts.ClusterRegistry,
		SQLCache:        features.UISQLCache.Enabled(),
		SQLCacheFactoryOptions: factory.CacheFactoryOptions{
			GCInterval:  gcInterval,
			GCKeepCount: gcKeepCount,
		},
		ExtensionAPIServer:            extensionAPIServer,
		SkipWaitForExtensionAPIServer: skipWaitForExtensionAPIServer,
	})
	if err != nil {
		return nil, err
	}

	clusterProxy, err := proxy.NewProxyMiddleware(wranglerContext.K8s.AuthorizationV1(),
		wranglerContext.TunnelServer.Dialer,
		wranglerContext.Mgmt.Cluster().Cache(),
		localClusterEnabled(opts),
		steve,
	)
	if err != nil {
		return nil, err
	}

	additionalAPIPreMCM := steveapi.AdditionalAPIsPreMCM(wranglerContext)
	additionalAPI, err := steveapi.AdditionalAPIs(ctx, wranglerContext, steve)
	if err != nil {
		return nil, err
	}

	auditLogMiddleware := func(next http.Handler) http.Handler {
		return next
	}
	var auditLogWriter *audit.Writer

	if opts.AuditLogEnabled {
		out := &lumberjack.Logger{
			Filename:   opts.AuditLogPath,
			MaxAge:     opts.AuditLogMaxage,
			MaxBackups: opts.AuditLogMaxbackup,
			MaxSize:    opts.AuditLogMaxsize,
		}
		defer out.Close()

		auditLogWriter, err = audit.NewWriter(out, audit.WriterOptions{
			DefaultPolicyLevel:     auditlogv1.Level(opts.AuditLogLevel),
			DisableDefaultPolicies: !opts.AuditLogEnabled,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create audit log writer: %w", err)
		}

		auditController := auditlog.New(wranglerContext.SharedControllerFactory)
		if err := auditlogcontroller.Register(ctx, auditLogWriter, auditController); err != nil {
			return nil, fmt.Errorf("failed to register audit log controller: %w", err)
		}

		auditLogMiddleware = audit.NewAuditLogMiddleware(auditLogWriter)
	}

	aggregationMiddleware := aggregation.NewMiddleware(ctx, wranglerContext.Mgmt.APIService(), wranglerContext.TunnelServer)

	wranglerContext.OnLeaderOrDie("rancher-new", func(ctx context.Context) error {
		serviceaccounttoken.StartServiceAccountSecretCleaner(
			ctx,
			wranglerContext.Core.Secret().Cache(),
			wranglerContext.Core.ServiceAccount().Cache(),
			wranglerContext.K8s.CoreV1())
		return nil
	})

	wranglerContext.OnLeader(func(context.Context) error {
		return cleanup.CleanupUnusedSecretTokens(
			wranglerContext.Core.Secret(),
			wranglerContext.Mgmt.AuthConfig(),
		)
	})

	var telemetryManager telemetry.TelemetryExporterManager
	// Only run when MCM is enabled but Agent is not - requires MCM for Cluster/Node access; fixes Harvester startup
	if utils.IsMCMServerOnly() {
		telG := telemetry.NewTelemetryGatherer(
			wranglerContext.Mgmt.Cluster().Cache(),
			wranglerContext.Mgmt.Node().Cache(),
		)
		telemetryManager = telemetry.NewTelemetryExporterManager(telG, time.Second*10)
	}

	return &Rancher{
		Auth: authServer.Authenticator.Chain(
			auditLogMiddleware),
		Handler: responsewriter.Chain{
			auth.SetXAPICattleAuthHeader,
			responsewriter.ContentTypeOptions,
			responsewriter.NoCache,
			websocket.NewWebsocketHandler,
			proxy.RewriteLocalCluster,
			clusterProxy,
			aggregationMiddleware,
			additionalAPIPreMCM,
			wranglerContext.MultiClusterManager.Middleware,
			authServer.Management,
			additionalAPI,
			requests.NewRequireAuthenticatedFilter("/v1/", "/v1/management.cattle.io.setting"),
		}.Handler(steve),
		Wrangler:                       wranglerContext,
		Steve:                          steve,
		auditLog:                       auditLogWriter,
		authServer:                     authServer,
		telemetryManager:               telemetryManager,
		opts:                           opts,
		aggregationRegistrationTimeout: opts.AggregationRegistrationTimeout,
		kubeAggregationReadyChan:       kubeAggregationReadyChan,
	}, nil
}

// settings aren't initialized yet so we need to use the regular client.
func getSQLCacheGCValues(wranglerContext *wrangler.Context) (time.Duration, int) {
	interval, _ := time.ParseDuration(settings.SQLCacheGCInterval.Default)
	gcIntervalSetting, err := wranglerContext.Mgmt.Setting().Get(settings.SQLCacheGCInterval.Name, metav1.GetOptions{})
	if err != nil {
		logrus.Warnf("Unable to fetch %s setting (will use default): %v", settings.SQLCacheGCInterval.Name, err)
	} else if gcIntervalSetting.Value != "" {
		dur, err := time.ParseDuration(gcIntervalSetting.Value)
		if err != nil {
			logrus.Warnf("Invalid GC interval %q: %v", gcIntervalSetting.Value, err)
		} else {
			interval = dur
		}
	}

	keepCount, _ := strconv.Atoi(settings.SQLCacheGCKeepCount.Default)
	gcKeepCountSetting, err := wranglerContext.Mgmt.Setting().Get(settings.SQLCacheGCKeepCount.Name, metav1.GetOptions{})
	if err != nil {
		logrus.Warnf("Unable to fetch %s setting (will use default): %v", settings.SQLCacheGCKeepCount.Name, err)
	} else if gcKeepCountSetting.Value != "" {
		count, err := strconv.Atoi(gcKeepCountSetting.Value)
		if err != nil {
			logrus.Warnf("Invalid GC keep count %q: %v", gcKeepCountSetting.Value, err)
		} else {
			keepCount = count
		}
	}
	return interval, keepCount
}

func (r *Rancher) Start(ctx context.Context) error {
	// ensure namespace for storing local users password is created
	if _, err := r.Wrangler.Core.Namespace().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: pbkdf2.LocalUserPasswordsNamespace},
	}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	if err := dashboardapi.Register(ctx, r.Wrangler); err != nil {
		return err
	}

	userManager, err := common.NewUserManagerNoBindings(r.Wrangler)
	if err != nil {
		return fmt.Errorf("error creating user manager: %w", err)
	}

	if err := steveapi.Setup(ctx, r.Steve, r.Wrangler, userManager); err != nil {
		return err
	}

	if features.MCM.Enabled() && features.UIExtension.Enabled() {
		plugin.Register(ctx, r.Wrangler)
	}

	if features.MCM.Enabled() {
		// Registers handlers for all rancher replicas running in the local cluster, but not downstream agents
		nodedriver.Register(ctx, r.Wrangler)
		kontainerdrivermetadata.Register(ctx, r.Wrangler)
		if err := r.Wrangler.MultiClusterManager.Start(ctx); err != nil {
			return err
		}
	}

	r.Wrangler.OnLeaderOrDie("rancher-start::dashboarddata", func(ctx context.Context) error {
		if err := dashboarddata.Add(ctx, r.Wrangler, localClusterEnabled(r.opts), r.opts.AddLocal == "false", r.opts.Embedded); err != nil {
			return errors.New("dashboarddata.Add() failed: " + err.Error())
		}

		if err := r.Wrangler.StartWithTransaction(ctx, func(ctx context.Context) error {
			return dashboard.Register(ctx, r.Wrangler, r.opts.Embedded, r.opts.ClusterRegistry)
		}); err != nil {
			return errors.New("dashboard.Register() failed: " + err.Error())
		}

		return runMigrations(r.Wrangler)
	})

	r.Wrangler.OnLeaderOrDie("rancher-start::DefferedCAPIRegistration", func(ctx context.Context) error {
		errChan := r.Wrangler.DeferredCAPIRegistration.DeferFuncWithError(runRKE2Migrations)
		err, ok := <-errChan
		if !ok {
			return nil
		}
		return err
	})

	if utils.IsMCMServerOnly() && features.RancherSCCRegistrationExtension.Enabled() {
		r.Wrangler.OnLeaderOrDie("rancher-start::RancherSCCRegistration", func(ctx context.Context) error {
			// TODO: pull this out of here if/when other features depend on the SecretRequest controllers
			if err := r.Wrangler.StartWithTransaction(ctx, func(ctx context.Context) error {
				return telemetrycontrollers.RegisterControllers(ctx, r.Wrangler, r.telemetryManager)
			}); err != nil {
				return errors.New("telemetrycontrollers.RegisterControllers() failed: " + err.Error())
			}
			logrus.Debug("[rancher::Start] starting RancherSCCRegistrationExtension")

			return scc.StartDeployer(ctx, r.Wrangler)
		})
	}

	if err := r.authServer.Start(ctx, false); err != nil {
		return err
	}

	r.Wrangler.OnLeaderOrDie("rancher-start::authServer", r.authServer.OnLeader)

	r.auditLog.Start(ctx)

	if utils.IsMCMServerOnly() {
		r.startTelemetryManager(context.TODO())
	}
	return r.Wrangler.Start(ctx)
}

func (r *Rancher) startTelemetryManager(ctx context.Context) {
	if r.telemetryManager == nil {
		logrus.Info("telemetry manager not enabled")
		logrus.Debug("not starting telemetry manager because it is disabled")
		return
	}

	initChan := make(chan struct{})
	initInfo := &initcond.InitInfo{}
	go func() {
		initcond.WaitForInfo(r.Wrangler, initInfo, initChan)
	}()
	go func() {
		logrus.Info("waiting for telemetry manager to start...")
		<-initChan
		log := logrus.WithFields(
			logrus.Fields{
				"server-url":      initInfo.ServerURL,
				"cluster-uuid":    initInfo.ClusterUUID,
				"install-uuid":    initInfo.InstallUUID,
				"rancher-version": initInfo.RancherVersion,
				"git-hash":        initInfo.GitHash,
			},
		)
		if err := r.telemetryManager.Start(ctx, *initInfo); err != nil {
			log.Errorf("failed to start telemetry manager : %s", err)
		}
		log.Info("telemetry manager started")
	}()
}

func (r *Rancher) ListenAndServe(ctx context.Context) error {
	if err := r.Start(ctx); err != nil {
		return err
	}

	r.Wrangler.MultiClusterManager.Wait(ctx)

	r.startAggregation(ctx)
	go r.Steve.StartAggregation(ctx)

	if !features.MCMAgent.Enabled() && r.kubeAggregationReadyChan != nil {
		go r.checkAPIAggregationOrDie()
	}

	if err := tls.ListenAndServe(ctx, r.Wrangler.RESTConfig,
		r.Auth(r.Handler),
		r.opts.BindHost,
		r.opts.HTTPSListenPort,
		r.opts.HTTPListenPort,
		r.opts.ACMEDomains,
		r.opts.NoCACerts); err != nil {
		return err
	}

	<-ctx.Done()
	return ctx.Err()
}

// checkAPIAggregationOrDie will wait for the kubeapi server to contact the configured APIService endpoints.
// If the condition is not met within the aggregationRegistrationTimeout the Rancher process will exit with an error.
func (r *Rancher) checkAPIAggregationOrDie() {
	logrus.Infof("Waiting for %s imperative API to be ready", r.aggregationRegistrationTimeout)

	ctxTimeout, cancel := context.WithTimeout(context.Background(), r.aggregationRegistrationTimeout)
	defer cancel()

	apiserviceClient := r.Wrangler.API.APIService()
	for {
		// Case 1: Successful - ExtensionServer was contacted by the Kube API, causing the channel to be closed.
		// Case 2: Successful - ExtensionServer in a different replica was contacted by the Kube API, and that updated the APIService object annotation.
		// Case 3: Fatal - the Kube API didn't contact the Extension Server within the configured timeout, interpreted as API Aggregation not being supported in the cluster.
		select {
		case <-r.kubeAggregationReadyChan:
			logrus.Info("kube-apiserver connected to imperative api")

			if err := ext.SetAggregationCheck(apiserviceClient, true); err != nil {
				logrus.Warnf("failed to set aggregation pre-check: %s", err)
			}

			return
		case <-time.After(5 * time.Second):
			if ext.AggregationPreCheck(apiserviceClient) {
				logrus.Info("kube-apiserver connected to imperative api")
				return
			}
		case <-ctxTimeout.Done():
			if err := ext.SetAggregationCheck(apiserviceClient, false); err != nil {
				logrus.Warnf("failed to unset aggregation pre-check: %s", err)
			}

			logrus.Fatal("kube-apiserver did not contact the rancher imperative api in time, please see https://ranchermanager.docs.rancher.com/api/extension-apiserver for more information")
		}
	}
}

func (r *Rancher) startAggregation(ctx context.Context) {
	aggregation2.Watch(ctx, r.Wrangler.Core.Secret(), namespace.System, "stv-aggregation", r.Handler)
}

func newMCM(wrangler *wrangler.Context, opts *Options) wrangler.MultiClusterManager {
	return multiclustermanager.NewDeferredServer(wrangler, &multiclustermanager.Options{
		RemoveLocalCluster:  opts.AddLocal == "false",
		LocalClusterEnabled: localClusterEnabled(opts),
		Embedded:            opts.Embedded,
		HTTPSListenPort:     opts.HTTPSListenPort,
		Debug:               opts.Debug,
		Trace:               opts.Trace,
	})
}

func setupAndValidationRESTConfig(ctx context.Context, restConfig *rest.Config) (*rest.Config, error) {
	restConfig = steveserver.RestConfigDefaults(restConfig)
	return restConfig, k8scheck.Wait(ctx, *restConfig)
}

func localClusterEnabled(opts *Options) bool {
	if opts.AddLocal == "true" || opts.AddLocal == "auto" {
		return true
	}
	return false
}

// createOrUpdateService ensures the Service exists and matches the desired spec.
func createOrUpdateService(ctx context.Context, k8sClient *kubernetes.Clientset, svc v1.Service) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existing, err := k8sClient.CoreV1().Services(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err := k8sClient.CoreV1().Services(svc.Namespace).Create(ctx, &svc, metav1.CreateOptions{})
				return err
			}
			return err
		}
		if !equality.Semantic.DeepEqual(existing.Spec.Ports, svc.Spec.Ports) {
			logrus.Debugf("service %s/%s ports did not match, refreshing service, (existing: %v vs desired: %v)",
				svc.Namespace, svc.Name, existing.Spec.Ports, svc.Spec.Ports)
			existing.Spec.Ports = svc.Spec.Ports
			_, err := k8sClient.CoreV1().Services(svc.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
			return err
		}
		return nil
	})
}

// createOrUpdateEndpoint ensures the Endpoints object exists and matches the desired spec.
func createOrUpdateEndpoint(ctx context.Context, k8sClient *kubernetes.Clientset, ep v1.Endpoints) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existing, err := k8sClient.CoreV1().Endpoints(ep.Namespace).Get(ctx, ep.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err := k8sClient.CoreV1().Endpoints(ep.Namespace).Create(ctx, &ep, metav1.CreateOptions{})
				return err
			}
			return err
		}

		if !equality.Semantic.DeepEqual(existing.Subsets, ep.Subsets) {
			logrus.Debugf("endpoint %s/%s subsets did not match, refreshing endpoint (existing: %v vs desired: %v)",
				ep.Namespace, ep.Name, existing.Subsets, ep.Subsets)
			existing.Subsets = ep.Subsets
			_, err := k8sClient.CoreV1().Endpoints(ep.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
			return err
		}
		return nil
	})
}

// setupRancherService ensures that the required services and endpoints exist on the local cluster.
// This should only be called when Rancher is run in a Docker container and not in a pod.
func setupRancherService(ctx context.Context, restConfig *rest.Config, httpsListenPort int) error {
	k8sClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("error setting up kubernetes clientset while setting up rancher service: %w", err)
	}

	rancherService := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      apiservice.RancherServiceName,
			Namespace: namespace.System,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{
				Protocol:   v1.ProtocolTCP,
				Port:       443,
				TargetPort: intstr.FromInt32(int32(httpsListenPort)),
			}},
		},
	}
	internalService := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      apiservice.RancherInternalServiceName,
			Namespace: namespace.System,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{
				Protocol:   v1.ProtocolTCP,
				Port:       443,
				TargetPort: intstr.FromInt32(int32(httpsListenPort + 1)),
			}},
		},
	}

	ip, err := net.ChooseHostInterface()
	if err != nil {
		return fmt.Errorf("setupRancherService error getting host IP: %w", err)
	}

	rancherEndpoint := v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      apiservice.RancherServiceName,
			Namespace: namespace.System,
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: ip.String()}},
			Ports: []v1.EndpointPort{{
				Port:     int32(httpsListenPort),
				Protocol: v1.ProtocolTCP,
			}},
		}},
	}
	internalEndpoint := v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      apiservice.RancherInternalServiceName,
			Namespace: namespace.System,
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: ip.String()}},
			Ports: []v1.EndpointPort{{
				Port:     int32(httpsListenPort + 1),
				Protocol: v1.ProtocolTCP,
			}},
		}},
	}

	// Setup rancher service and endpoint
	if err := createOrUpdateService(ctx, k8sClient, rancherService); err != nil {
		return fmt.Errorf("setupRancherService error refreshing rancher service: %w", err)
	}
	if err := createOrUpdateEndpoint(ctx, k8sClient, rancherEndpoint); err != nil {
		return fmt.Errorf("setupRancherService error refreshing rancher endpoint: %w", err)
	}
	// Setup rancher-internal service and endpoint
	if err := createOrUpdateService(ctx, k8sClient, internalService); err != nil {
		return fmt.Errorf("setupRancherService error refreshing internal service: %w", err)
	}
	if err := createOrUpdateEndpoint(ctx, k8sClient, internalEndpoint); err != nil {
		return fmt.Errorf("setupRancherService error refreshing internal endpoint: %w", err)
	}

	return nil
}

// bumpRancherServiceVersion bumps the version of rancher-webhook if it is detected that the version is less than
// v0.2.2-alpha1. This is because the version of rancher-webhook less than v0.2.2-alpha1 does not support Kubernetes v1.22+
// This should only be called when Rancher is run in a Docker container because the Kubernetes version and Rancher version
// are bumped at the same time. In a Kubernetes cluster, usually the Rancher version is bumped when the cluster is upgraded.
func bumpRancherWebhookIfNecessary(ctx context.Context, restConfig *rest.Config) error {
	v := os.Getenv("CATTLE_RANCHER_WEBHOOK_VERSION")
	webhookVersionParts := strings.Split(v, "+up")
	if len(webhookVersionParts) != 2 {
		return nil
	} else if !strings.HasPrefix(webhookVersionParts[1], "v") {
		webhookVersionParts[1] = "v" + webhookVersionParts[1]
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("error setting up kubernetes clientset: %w", err)
	}

	rancherWebhookDeployment, err := clientset.AppsV1().Deployments(namespace.System).Get(ctx, "rancher-webhook", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	for i, c := range rancherWebhookDeployment.Spec.Template.Spec.Containers {
		imageVersionParts := strings.Split(c.Image, ":")
		if c.Name != "rancher-webhook" || len(imageVersionParts) != 2 {
			continue
		}

		semVer, err := semver.NewVersion(strings.TrimPrefix(imageVersionParts[1], "v"))
		if err != nil {
			continue
		}
		if semVer.LessThan(semver.MustParse("0.2.2-alpha1")) {
			rancherWebhookDeployment = rancherWebhookDeployment.DeepCopy()
			c.Image = fmt.Sprintf("%s:%s", imageVersionParts[0], webhookVersionParts[1])
			rancherWebhookDeployment.Spec.Template.Spec.Containers[i] = c

			_, err = clientset.AppsV1().Deployments(namespace.System).Update(ctx, rancherWebhookDeployment, metav1.UpdateOptions{})
			return err
		}
	}

	return nil
}

// migrateEncryptionConfig uses the dynamic client to get all clusters and then marshals them through the
// standard go JSON package using the updated backing structs in RKE that include JSON tags. The k8s JSON
// tools are strict with casing so the fields would be dropped before getting saved back in the proper casing
// if any controller touches the cluster first. See https://github.com/rancher/rancher/issues/31385
func migrateEncryptionConfig(ctx context.Context, restConfig *rest.Config) error {
	dynamicClient, err := k8dynamic.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	clusterDynamicClient := dynamicClient.Resource(mgmntv3.ClusterGroupVersionResource)

	clusters, err := clusterDynamicClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// IsNotFound error means the CRD type doesn't exist in the cluster, indicating this is the first Rancher startup
		return nil
	}

	var allErrors error

	for _, c := range clusters.Items {
		err := wait.PollImmediateInfinite(100*time.Millisecond, func() (bool, error) {
			rawDynamicCluster, err := clusterDynamicClient.Get(ctx, c.GetName(), metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			annotations := rawDynamicCluster.GetAnnotations()
			if annotations != nil && annotations[encryptionConfigUpdate] == "true" {
				return true, nil
			}

			clusterBytes, err := rawDynamicCluster.MarshalJSON()
			if err != nil {
				return false, fmt.Errorf("error trying to Marshal dynamic cluster: %w", err)
			}

			var cluster *v3.Cluster

			if err := json.Unmarshal(clusterBytes, &cluster); err != nil {
				return false, fmt.Errorf("error trying to Unmarshal dynamicCluster into v3 cluster: %w", err)
			}

			if cluster.Annotations == nil {
				cluster.Annotations = make(map[string]string)
			}
			cluster.Annotations[encryptionConfigUpdate] = "true"

			u, err := unstructured.ToUnstructured(cluster)
			if err != nil {
				return false, err
			}

			_, err = clusterDynamicClient.Update(ctx, u, metav1.UpdateOptions{})
			if err == nil {
				return true, nil
			}
			if apierrors.IsConflict(err) || apierrors.IsServiceUnavailable(err) || apierrors.IsInternalError(err) {
				return false, nil
			}
			return false, err
		})
		allErrors = errors.Join(err, allErrors)
	}
	return allErrors
}

// checks for deprecated RKE1 resources in the cluster to ensure that the cluster is not using any deprecated resources.
func validateRKE1Resources(wranglerContext *wrangler.Context) error {
	resources, err := checkForRKE1Resources(wranglerContext)
	if err != nil {
		return fmt.Errorf("checking for RKE1 resources: %w", err)
	}
	if len(resources) == 0 {
		return nil
	}

	return fmt.Errorf("Rancher v2.12+ does not support RKE1. Detected RKE1-related resources (listed below).\nPlease migrate these clusters to RKE2 or K3s, or delete the related resources. More info: https://www.suse.com/c/rke-end-of-life-by-july-2025-replatform-to-rke2-or-k3s/\n - %s", strings.Join(resources, "\n - "))
}

// checkForRKE1Resources scans for deprecated RKE1 (Rancher Kubernetes Engine v1) resources in the Rancher management context.
func checkForRKE1Resources(wranglerContext *wrangler.Context) ([]string, error) {
	var found []string

	logrus.Infof("Scanning NodeTemplates in namespace: %s, group: nodetemplates.management.cattle.io", namespace.NodeTemplateGlobalNamespace)
	logrus.Infof("Scanning ClusterTemplates in namespace: %s, group: clustertemplates.management.cattle.io", namespace.GlobalNamespace)

	// Check for RKE1 clusters
	clusters, err := wranglerContext.Mgmt.Cluster().List(metav1.ListOptions{})
	if apierrors.IsNotFound(err) {
		clusters = &v3.ClusterList{}
	} else if err != nil {
		return nil, fmt.Errorf("error checking RKE1 clusters: %w", err)
	}

	for _, cluster := range clusters.Items {
		if cluster.Spec.RancherKubernetesEngineConfig != nil {
			found = append(found, fmt.Sprintf("Cluster: name=%s, displayName=%s", cluster.Name, cluster.Spec.DisplayName))
		}
	}

	// NodeTemplates in the global node template namespace
	nodeTemplates, err := wranglerContext.Mgmt.NodeTemplate().List(namespace.NodeTemplateGlobalNamespace, metav1.ListOptions{})

	if apierrors.IsNotFound(err) {
		nodeTemplates = &v3.NodeTemplateList{}
	} else if err != nil {
		return nil, fmt.Errorf("error checking nodeTemplates: %w", err)
	}

	for _, obj := range nodeTemplates.Items {
		found = append(found, fmt.Sprintf("NodeTemplate: name=%s, displayName=%s", obj.Name, obj.Spec.DisplayName))
	}

	// ClusterTemplates in the global namespace
	clusterTemplates, err := wranglerContext.Mgmt.ClusterTemplate().List(namespace.GlobalNamespace, metav1.ListOptions{})

	if apierrors.IsNotFound(err) {
		clusterTemplates = &v3.ClusterTemplateList{}
	} else if err != nil {
		return nil, fmt.Errorf("error checking clusterTemplates: %w", err)
	}

	for _, obj := range clusterTemplates.Items {
		found = append(found, fmt.Sprintf("ClusterTemplate: name=%s, displayName=%s", obj.Name, obj.Spec.DisplayName))
	}

	return found, nil
}
