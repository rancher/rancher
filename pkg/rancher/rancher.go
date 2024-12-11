package rancher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	responsewriter "github.com/rancher/apiserver/pkg/middleware"
	"github.com/rancher/rancher/pkg/api/norman/customization/kontainerdriver"
	steveapi "github.com/rancher/rancher/pkg/api/steve"
	"github.com/rancher/rancher/pkg/api/steve/aggregation"
	"github.com/rancher/rancher/pkg/api/steve/proxy"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/requests"
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
	"github.com/rancher/rancher/pkg/features"
	mgmntv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainerdrivermetadata"
	"github.com/rancher/rancher/pkg/multiclustermanager"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/tls"
	"github.com/rancher/rancher/pkg/ui"
	"github.com/rancher/rancher/pkg/websocket"
	"github.com/rancher/rancher/pkg/wrangler"
	aggregation2 "github.com/rancher/steve/pkg/aggregation"
	steveauth "github.com/rancher/steve/pkg/auth"
	steveserver "github.com/rancher/steve/pkg/server"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/k8scheck"
	"github.com/rancher/wrangler/v3/pkg/unstructured"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
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

const encryptionConfigUpdate = "provisioner.cattle.io/encrypt-migrated"

type Options struct {
	ACMEDomains       cli.StringSlice
	AddLocal          string
	Embedded          bool
	BindHost          string
	HTTPListenPort    int
	HTTPSListenPort   int
	K8sMode           string
	Debug             bool
	Trace             bool
	NoCACerts         bool
	AuditLogPath      string
	AuditLogMaxage    int
	AuditLogMaxsize   int
	AuditLogMaxbackup int
	AuditLevel        int
	Features          string
	ClusterRegistry   string
}

type Rancher struct {
	Auth     steveauth.Middleware
	Handler  http.Handler
	Wrangler *wrangler.Context
	Steve    *steveserver.Server

	auditLog   *audit.LogWriter
	authServer *auth.Server
	opts       *Options
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

	wranglerContext, err := wrangler.NewContext(ctx, clientConfg, restConfig)
	if err != nil {
		return nil, err
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
		authServer, err = auth.NewServer(ctx, restConfig)
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

	steve, err := steveserver.New(ctx, restConfig, &steveserver.Options{
		ServerVersion:   settings.ServerVersion.Get(),
		Controllers:     steveControllers,
		AccessSetLookup: wranglerContext.ASL,
		AuthMiddleware:  steveauth.ExistingContext,
		Next:            ui.New(wranglerContext.Mgmt.Preference().Cache(), wranglerContext.Mgmt.ClusterRegistrationToken().Cache()),
		ClusterRegistry: opts.ClusterRegistry,
		SQLCache:        features.UISQLCache.Enabled(),
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

	auditLogWriter := audit.NewLogWriter(opts.AuditLogPath, audit.Level(opts.AuditLevel), opts.AuditLogMaxage, opts.AuditLogMaxbackup, opts.AuditLogMaxsize)
	auditFilter, err := audit.NewAuditLogMiddleware(auditLogWriter)
	if err != nil {
		return nil, err
	}
	aggregationMiddleware := aggregation.NewMiddleware(ctx, wranglerContext.Mgmt.APIService(), wranglerContext.TunnelServer)

	wranglerContext.OnLeader(func(ctx context.Context) error {
		serviceaccounttoken.StartServiceAccountSecretCleaner(
			ctx,
			wranglerContext.Core.Secret().Cache(),
			wranglerContext.Core.ServiceAccount().Cache(),
			wranglerContext.K8s.CoreV1())
		return nil
	})

	return &Rancher{
		Auth: authServer.Authenticator.Chain(
			auditFilter),
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
		Wrangler:   wranglerContext,
		Steve:      steve,
		auditLog:   auditLogWriter,
		authServer: authServer,
		opts:       opts,
	}, nil
}

func (r *Rancher) Start(ctx context.Context) error {
	if err := dashboardapi.Register(ctx, r.Wrangler); err != nil {
		return err
	}

	if err := steveapi.Setup(ctx, r.Steve, r.Wrangler); err != nil {
		return err
	}
	if features.UIExtension.Enabled() {
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

	r.Wrangler.OnLeader(func(ctx context.Context) error {
		if err := dashboarddata.Add(ctx, r.Wrangler, localClusterEnabled(r.opts), r.opts.AddLocal == "false", r.opts.Embedded); err != nil {
			return err
		}
		if err := r.Wrangler.StartWithTransaction(ctx, func(ctx context.Context) error {
			return dashboard.Register(ctx, r.Wrangler, r.opts.Embedded, r.opts.ClusterRegistry)
		}); err != nil {
			return err
		}

		return runMigrations(r.Wrangler)
	})

	if err := r.authServer.Start(ctx, false); err != nil {
		return err
	}

	r.Wrangler.OnLeader(r.authServer.OnLeader)
	r.auditLog.Start(ctx)

	return r.Wrangler.Start(ctx)
}

func (r *Rancher) ListenAndServe(ctx context.Context) error {
	if err := r.Start(ctx); err != nil {
		return err
	}

	r.Wrangler.MultiClusterManager.Wait(ctx)

	r.startAggregation(ctx)
	go r.Steve.StartAggregation(ctx)
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

// setupRancherService will ensure that a Rancher service with a custom endpoint exists that will be used
// to access Rancher
func setupRancherService(ctx context.Context, restConfig *rest.Config, httpsListenPort int) error {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("error setting up kubernetes clientset while setting up rancher service: %w", err)
	}

	service := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      apiservice.RancherServiceName,
			Namespace: namespace.System,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       443,
					TargetPort: intstr.FromInt(httpsListenPort + 1),
				},
			},
		},
	}

	refreshService := false

	s, err := clientset.CoreV1().Services(namespace.System).Get(ctx, apiservice.RancherServiceName, metav1.GetOptions{})
	if err != nil {
		if k8serror.IsNotFound(err) {
			refreshService = true
		} else {
			return fmt.Errorf("error looking for rancher service: %w", err)
		}
	} else {
		if s.Spec.String() != service.Spec.String() {
			refreshService = true
		}
	}

	if refreshService {
		logrus.Debugf("setupRancherService refreshing service")
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if s, err := clientset.CoreV1().Services(namespace.System).Get(ctx, apiservice.RancherServiceName, metav1.GetOptions{}); err != nil {
				if k8serror.IsNotFound(err) {
					if _, err := clientset.CoreV1().Services(namespace.System).Create(ctx, &service, metav1.CreateOptions{}); err != nil {
						return err
					}
				} else {
					return err
				}
			} else {
				s.Spec.Ports = service.Spec.Ports
				if _, err := clientset.CoreV1().Services(namespace.System).Update(ctx, s, metav1.UpdateOptions{}); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("setupRancherService error refreshing service: %w", err)
		}
	}

	ip, err := net.ChooseHostInterface()
	if err != nil {
		return fmt.Errorf("setupRancherService error getting host IP while setting up rancher service: %w", err)
	}

	endpoint := v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      apiservice.RancherServiceName,
			Namespace: namespace.System,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{
						IP: ip.String(),
					},
				},
				Ports: []v1.EndpointPort{
					{
						Port: int32(httpsListenPort + 1),
					},
				},
			},
		},
	}

	refreshEndpoint := false
	e, err := clientset.CoreV1().Endpoints(namespace.System).Get(ctx, apiservice.RancherServiceName, metav1.GetOptions{})
	if err != nil {
		if k8serror.IsNotFound(err) {
			refreshEndpoint = true
		} else {
			return fmt.Errorf("error looking for rancher endpoint while setting up rancher service: %w", err)
		}
	} else {
		if e.Subsets[0].String() != endpoint.Subsets[0].String() && len(e.Subsets) != 1 {
			logrus.Debugf("setupRancherService subsets did not match, refreshing endpoint (%s vs %s)", e.Subsets[0].String(), endpoint.String())
			refreshEndpoint = true
		}
	}

	if refreshEndpoint {
		logrus.Debugf("setupRancherService refreshing endpoint")
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if e, err := clientset.CoreV1().Endpoints(namespace.System).Get(ctx, apiservice.RancherServiceName, metav1.GetOptions{}); err != nil {
				if k8serror.IsNotFound(err) {
					if _, err := clientset.CoreV1().Endpoints(namespace.System).Create(ctx, &endpoint, metav1.CreateOptions{}); err != nil {
						return err
					}
				} else {
					return err
				}
			} else {
				e.Subsets = endpoint.Subsets
				if _, err := clientset.CoreV1().Endpoints(namespace.System).Update(ctx, e, metav1.UpdateOptions{}); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("setupRancherService error refreshing endpoint: %w", err)
		}
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
		if k8serror.IsNotFound(err) {
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
		if !k8serror.IsNotFound(err) {
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
			if k8serror.IsConflict(err) || k8serror.IsServiceUnavailable(err) || k8serror.IsInternalError(err) {
				return false, nil
			}
			return false, err
		})
		allErrors = errors.Join(err, allErrors)
	}
	return allErrors
}
