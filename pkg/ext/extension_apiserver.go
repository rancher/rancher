package ext

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/storage/kubernetes"
	extstores "github.com/rancher/rancher/pkg/ext/stores"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/wrangler"
	steveext "github.com/rancher/steve/pkg/ext"
	steveserver "github.com/rancher/steve/pkg/server"
	wranglerapiregistrationv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	wranglercorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

func CreateOrUpdateService(service wranglercorev1.ServiceController) error {
	appSelector := "rancher"
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

func CleanupExtensionAPIServer(wranglerContext *wrangler.Context) error {
	if err := wranglerContext.Core.Service().Delete(Namespace, TargetServiceName, &metav1.DeleteOptions{}); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	if err := wranglerContext.API.APIService().Delete(APIServiceName, &metav1.DeleteOptions{}); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete APIService: %w", err)
	}

	return nil
}

func NewExtensionAPIServer(ctx context.Context, wranglerContext *wrangler.Context) (steveserver.ExtensionAPIServer, error) {
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

	config := dynamiclistener.Config{
		CN: fmt.Sprintf("%s.%s.svc", TargetServiceName, Namespace),
		RegenerateCerts: func() bool {
			_, err := wranglerContext.Core.Secret().Get(Namespace, CertName, metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		},
	}

	var additionalSniProviders []dynamiccertificates.SNICertKeyContentProvider
	var ln net.Listener

	if features.ImperativeApiExtension.Enabled() {
		logrus.Info("creating imperative extension apiserver resources")

		// Only need to listen on localhost because that port will be reached
		// from a remotedialer tunnel on localhost
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", Port))
		if err != nil {
			return nil, fmt.Errorf("failed to create tcp listener: %w", err)
		}

		sniProvider := NewStore("imperative-api-sni-provider", []string{fmt.Sprintf("%s.%s.svc", TargetServiceName, Namespace)})
		sniProvider.AddListener(ApiServiceCertListener(sniProvider, wranglerContext.API.APIService()))

		additionalSniProviders = append(additionalSniProviders, sniProvider)

		store := kubernetes.New(ctx, coreGetterFactory(wranglerContext), Namespace, CertName, sniProvider)

		ln, _, err = dynamiclistener.NewListenerWithChain(ln, store, nil, nil, config)
		if err != nil {
			return nil, err
		}

		if err := CreateOrUpdateService(wranglerContext.Core.Service()); err != nil {
			return nil, fmt.Errorf("failed to create or update APIService: %w", err)
		}

		defaultAuthenticator, err := steveext.NewDefaultAuthenticator(wranglerContext.K8s)
		if err != nil {
			return nil, fmt.Errorf("failed to create extension server authenticator: %w", err)
		}

		authenticators = append(authenticators, defaultAuthenticator)
	} else {
		logrus.Info("deleting imperative extension apiserver resources")

		ln = NewBlockingListener()

		if err := CleanupExtensionAPIServer(wranglerContext); err != nil {
			return nil, fmt.Errorf("failed to clean up extension api resources: %w", err)
		}
	}

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
			allowedPathsPrefix := []string{"/api", "/apis", "/openapi/v2", "/openapi/v3"}
			for _, path := range allowedPathsPrefix {
				if strings.HasPrefix(a.GetPath(), path) {
					maybeAllowed = true
					break
				}
			}

			if !maybeAllowed {
				return authorizer.DecisionDeny, "only /api, /apis, /openapi/v2 and /openapi/v3 supported", nil
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

	return extensionAPIServer, nil
}
