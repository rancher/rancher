package ext

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	extstores "github.com/rancher/rancher/pkg/ext/stores"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/wrangler"
	steveext "github.com/rancher/steve/pkg/ext"
	steveserver "github.com/rancher/steve/pkg/server"
	corev1 "k8s.io/api/core/v1"
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
	CACertSecretName  = "imperative-api-extension-ca"
	TargetServiceName = "imperative-api-extension"
	Namespace         = "cattle-system"
)

func APIService(caBundle []byte) (*apiregv1.APIService, error) {
	port := int32(Port)

	return &apiregv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: APIServiceName,
		},
		Spec: apiregv1.APIServiceSpec{
			Group:                "ext.cattle.io",
			GroupPriorityMinimum: 100,
			CABundle:             caBundle,
			Service: &apiregv1.ServiceReference{
				Namespace: "cattle-system",
				Name:      "imperative-api-extension",
				Port:      &port,
			},
			Version:         "v1",
			VersionPriority: 100,
		},
	}, nil
}

func Service() corev1.Service {
	return corev1.Service{
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
				"app": "rancher",
			},
		},
	}
}

func NewExtensionAPIServer(ctx context.Context, wranglerContext *wrangler.Context) (steveserver.ExtensionAPIServer, error) {
	// Only the local cluster runs an extension API server
	if features.MCMAgent.Enabled() {
		return nil, nil
	}

	scheme := wrangler.Scheme
	// Only need to listen on localhost because that port will be reached
	// from a remotedialer tunnel on localhost
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", Port))
	if err != nil {
		return nil, fmt.Errorf("failed to create tcp listener: %w", err)
	}

	defaultAuthenticator, err := steveext.NewDefaultAuthenticator(wranglerContext.K8s)
	if err != nil {
		return nil, fmt.Errorf("failed to create extension server authenticator: %w", err)
	}

	authenticator := steveext.NewUnionAuthenticator(
		authenticator.RequestFunc(func(req *http.Request) (*authenticator.Response, bool, error) {
			reqUser, ok := request.UserFrom(req.Context())
			if !ok {
				return nil, false, nil
			}
			return &authenticator.Response{
				User: reqUser,
			}, true, nil
		}),
		defaultAuthenticator,
	)

	sniProvider, err := certForCommonName(fmt.Sprintf("%s.%s.svc", TargetServiceName, Namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to generate cert for target service: %w", err)
	}

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

			// Until https://github.com/rancher/rancher/issues/47483 is fixed
			// we limit nonResourceURLs to what is effectively an admin user
			attrs := authorizer.AttributesRecord{
				User:            a.GetUser(),
				Verb:            "*",
				APIGroup:        "*",
				Resource:        "*",
				ResourceRequest: true,
			}
			return aslAuthorizer.Authorize(ctx, attrs)
		}),
		SNICerts: []dynamiccertificates.SNICertKeyContentProvider{sniProvider},
	}

	extensionAPIServer, err := steveext.NewExtensionAPIServer(scheme, codecs, extOpts)
	if err != nil {
		return nil, fmt.Errorf("new extension API server: %w", err)
	}

	if err = extstores.InstallStores(extensionAPIServer, scheme); err != nil {
		return nil, fmt.Errorf("failed to install install stores: %w", err)
	}

	service := Service()
	if _, err := wranglerContext.Core.Service().Create(&service); client.IgnoreAlreadyExists(err) != nil {
		return nil, fmt.Errorf("failed to create a new proxy service: %w", err)
	}

	caBundle, _ := sniProvider.CurrentCertKeyContent()
	apiService, err := APIService(caBundle)
	if err != nil {
		return nil, fmt.Errorf("failed to construct a new APIService: %w", err)
	}

	if _, err := wranglerContext.API.APIService().Create(apiService); client.IgnoreAlreadyExists(err) != nil {
		return nil, fmt.Errorf("failed to create a new APIService: %w", err)
	}

	return extensionAPIServer, nil
}

func CleanupExtensionAPIServer(wranglerContext *wrangler.Context) error {
	if err := wranglerContext.Core.Service().Delete(Namespace, APIServiceName, &metav1.DeleteOptions{}); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to create a new proxy service: %w", err)
	}

	if err := wranglerContext.API.APIService().Delete(Service().Name, &metav1.DeleteOptions{}); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete APIService: %w", err)
	}

	return nil
}
