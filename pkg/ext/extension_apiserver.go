package ext

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/rancher/pkg/controllers/managementuser/clusterauthtoken"
	extstores "github.com/rancher/rancher/pkg/ext/stores"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/generated/controllers/ext.cattle.io"
	"github.com/rancher/rancher/pkg/wrangler"
	steveext "github.com/rancher/steve/pkg/ext"
	steveserver "github.com/rancher/steve/pkg/server"
	wranglerapiregistrationv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	wranglercorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	apiregv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

const (
	imperativeApiExtensionEnvVar = "IMPERATIVE_API_APP_SELECTOR"
)

type Options struct {
	// AppSelector is the expected value for the "app" label on the rancher service.
	AppSelector string
}

func DefaultOptions() Options {
	return Options{
		AppSelector: os.Getenv(imperativeApiExtensionEnvVar),
	}
}

const (
	// Port is the port that the separate extension API server listen to as
	// defined in the Imperative API RFC.
	//
	// The main kube-apiserver will connect to that port (through a tunnel).
	Port              = 6666
	APIServiceName    = "v1.ext.cattle.io"
	TargetServiceName = "imperative-api-extension"
	Namespace         = "cattle-system"
)

func CreateOrUpdateAPIService(apiservice wranglerapiregistrationv1.APIServiceController, caBundle []byte) error {
	port := int32(Port)
	desired := &apiregv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: APIServiceName,
		},
		Spec: apiregv1.APIServiceSpec{
			Group:                "ext.cattle.io",
			GroupPriorityMinimum: 100,
			CABundle:             caBundle,
			Service: &apiregv1.ServiceReference{
				Namespace: Namespace,
				Name:      TargetServiceName,
				Port:      &port,
			},
			Version:         "v1",
			VersionPriority: 100,
		},
	}

	current, err := apiservice.Get(APIServiceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, err := apiservice.Create(desired); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		current.Spec = desired.Spec

		if _, err := apiservice.Update(current); err != nil {
			return err
		}
	}

	return nil
}

func CreateOrUpdateService(service wranglercorev1.ServiceController, appSelector string) error {
	if RDPEnabled() {
		appSelector = "api-extension"
	}

	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TargetServiceName,
			Namespace: Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: Port,
				},
			},
			Selector: map[string]string{
				"app": appSelector,
			},
		},
	}

	current, err := service.Get(Namespace, TargetServiceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, err := service.Create(desired); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		current.Spec = desired.Spec

		if _, err := service.Update(current); err != nil {
			return err
		}
	}

	return nil
}

func NewExtensionAPIServer(ctx context.Context, wranglerContext *wrangler.Context, opts Options) (steveserver.ExtensionAPIServer, error) {
	// Only the local cluster runs an extension API server
	if features.MCMAgent.Enabled() {
		return nil, nil
	}

	authenticators := []authenticator.Request{
		authenticator.RequestFunc(func(req *http.Request) (*authenticator.Response, bool, error) {
			reqUser, ok := request.UserFrom(req.Context())
			if !ok {
				return nil, false, nil
			}
			return &authenticator.Response{
				User: reqUser,
			}, true, nil
		}),
	}

	var additionalSniProviders []dynamiccertificates.SNICertKeyContentProvider
	var ln net.Listener

	logrus.Info("creating imperative extension apiserver resources")

	sniProvider, err := NewSNIProviderForCname(
		"imperative-api-sni-provider",
		[]string{fmt.Sprintf("%s.%s.svc", TargetServiceName, Namespace)},
		wranglerContext.Core.Secret(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cert for target service: %w", err)
	}

	sniProvider.AddListener(ApiServiceCertListener(sniProvider, wranglerContext.API.APIService()))

	go func() {
		// sniProvider.Run uses a Watch that could be aborted due to external reasons, make sure we retry unless the context was already canceled
		for {
			if err := sniProvider.Run(ctx.Done()); err != nil {
				logrus.Errorf("sni provider failed: %s", err)
				if ctx.Err() != nil {
					return
				}
			}
			time.Sleep(10 * time.Second)
		}
	}()

	// Only need to listen on localhost because that port will be reached
	// from a remotedialer tunnel on localhost
	ln, err = net.Listen("tcp", fmt.Sprintf(":%d", Port))
	if err != nil {
		return nil, fmt.Errorf("failed to create tcp listener: %w", err)
	}

	additionalSniProviders = append(additionalSniProviders, sniProvider)

	if err := CreateOrUpdateService(wranglerContext.Core.Service(), opts.AppSelector); err != nil {
		return nil, fmt.Errorf("failed to create or update APIService: %w", err)
	}

	defaultAuthenticator, err := steveext.NewDefaultAuthenticator(wranglerContext.K8s)
	if err != nil {
		return nil, fmt.Errorf("failed to create extension server authenticator: %w", err)
	}

	authenticators = append(authenticators, defaultAuthenticator)

	scheme := wrangler.Scheme

	authenticator := steveext.NewUnionAuthenticator(authenticators...)

	aslAuthorizer := steveext.NewAccessSetAuthorizer(wranglerContext.ASL)
	codecs := serializer.NewCodecFactory(scheme)
	extOpts := steveext.ExtensionAPIServerOptions{
		Listener:              ln,
		GetOpenAPIDefinitions: getOpenAPIDefinitions,
		OpenAPIDefinitionNameReplacements: map[string]string{
			// The OpenAPI spec generated from the types in pkg/apis/ext.cattle.io/v1
			// ends up with the form "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1.<Type>".
			//
			// The k8s library then uses reflection to find those types (looking inside the *runtime.Scheme)
			// and automagically generate a name such as "com.github.rancher.rancher.pkg.apis.ext.cattle.io.v1.<Type>".
			// This is user facing and we want the names in the OpenAPI documents to be of the form "io.cattle.ext.v1.<Type>"
			// and that's what this replacement map is doing.
			"com.github.rancher.rancher.pkg.apis.ext.cattle.io.v1": "io.cattle.ext.v1",
			// TODO(frameworks): verify this needs to be added for openAPI stuff
			"com.github.rancher.rancher.pkg.apis.scc.cattle.io.v1": "io.cattle.scc.v1", // assume this is needed for my new CRDs too?
		},
		Authenticator: authenticator,
		Authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
			if a.IsResourceRequest() {
				return aslAuthorizer.Authorize(ctx, a)
			}

			// An API server has a lot more routes exposed but for now
			// we just want to expose these. Note /api is needed for client-go's
			// discovery even though not strictly necessary
			maybeAllowed := false
			allowedPathsPrefix := []string{"/api", "/apis", "/openapi/v2", "/openapi/v3", "/version"}
			for _, path := range allowedPathsPrefix {
				if strings.HasPrefix(a.GetPath(), path) {
					maybeAllowed = true
					break
				}
			}

			if !maybeAllowed {
				return authorizer.DecisionDeny, "only /api, /apis, /openapi/v2, /openapi/v3, and /version supported", nil
			}

			return aslAuthorizer.Authorize(ctx, a)
		}),
		SNICerts: additionalSniProviders,
	}

	extensionAPIServer, err := steveext.NewExtensionAPIServer(scheme, codecs, extOpts)
	if err != nil {
		return nil, fmt.Errorf("new extension API server: %w", err)
	}

	if err = extstores.InstallStores(extensionAPIServer, wranglerContext, scheme); err != nil {
		return nil, fmt.Errorf("failed to install stores: %w", err)
	}

	// Note to self: the extension api server is handed to the caller to be
	// run, which is done by handing it to a steveserver constructor. Not
	// having direct access to the `Run` a goro with a delay is used to wait
	// for the start and then perform the necessary factory action for
	// ext'ension controllers.

	go func() {
		time.Sleep(3 * time.Second)

		// Get the rest.Config from loopback client which has the following attributes:
		// - username: system:apiserver
		// - groups:   [system:authenticated system:masters]
		// - extras:   []
		restConfig := extensionAPIServer.LoopbackClientConfig()

		// set up factory and controllers for ext api
		controllerFactory, err := controller.NewSharedControllerFactoryFromConfigWithOptions(restConfig, scheme, nil)
		if err != nil {
			panic(err)
		}

		core, err := ext.NewFactoryFromConfigWithOptions(restConfig, &generic.FactoryOptions{
			SharedControllerFactory: controllerFactory,
		})
		if err != nil {
			panic(err)
		}

		// ext controller setup ...
		err = clusterauthtoken.RegisterExtIndexers(core.Ext().V1())
		if err != nil {
			panic(err)
		}

		err = controllerFactory.SharedCacheFactory().Start(ctx)
		if err != nil {
			panic(err)
		}

		controllerFactory.SharedCacheFactory().WaitForCacheSync(ctx)

		err = controllerFactory.Start(ctx, 10)
		if err != nil {
			panic(err)
		}

		// See clusterauthtoken's `registerDeferred` for user
		wrangler.InitExtAPI(wranglerContext, core.Ext().V1())
	}()

	return extensionAPIServer, nil
}

const apiAggregationPreCheckedAnnotation = "ext.cattle.io/aggregation-available-checked"

// AggregationPreCheck allows verifying if a previous execution of Rancher already checked API Agreggation works in the upstream cluster
func AggregationPreCheck(client wranglerapiregistrationv1.APIServiceClient) bool {
	apiservice, err := client.Get(APIServiceName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return apiservice.Annotations[apiAggregationPreCheckedAnnotation] == "true"
}

// SetAggregationCheck adds an annotation in the extension APIService object, so it can later be retrieved by AggregationPreCheck
func SetAggregationCheck(client wranglerapiregistrationv1.APIServiceClient, value bool) {
	apiservice, err := client.Get(APIServiceName, metav1.GetOptions{})
	if err != nil {
		logrus.Warnf("failed to set aggregation check for APIService: %v", err)
		return
	}

	previous := apiservice.Annotations[apiAggregationPreCheckedAnnotation] == "true"
	if previous == value {
		// already set
		return
	}

	if value {
		if apiservice.Annotations == nil {
			apiservice.Annotations = make(map[string]string)
		}
		apiservice.Annotations[apiAggregationPreCheckedAnnotation] = "true"
	} else {
		delete(apiservice.Annotations, apiAggregationPreCheckedAnnotation)
	}

	if _, err := client.Update(apiservice); err != nil {
		logrus.Warnf("failed to set aggregation check for APIService: %v", err)
	}
}
