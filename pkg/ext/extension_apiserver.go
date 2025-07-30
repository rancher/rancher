package ext

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	extstores "github.com/rancher/rancher/pkg/ext/stores"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/wrangler"
	steveext "github.com/rancher/steve/pkg/ext"
	steveserver "github.com/rancher/steve/pkg/server"
	wranglerapiregistrationv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/client-go/util/retry"
	apiregv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

const (
	imperativeApiExtensionEnvVar = "IMPERATIVE_API_APP_SELECTOR"

	annotationApiAggregationPreChecked = "ext.cattle.io/aggregation-available-checked"
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
	TargetServiceName = "api-extension"
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

	original, err := apiservice.Get(APIServiceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, err := apiservice.Create(desired); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {

		updateErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			current, err := apiservice.Get(APIServiceName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			modified := current.DeepCopy()
			modified.Spec = desired.Spec
			patch, err := makePatchAndUpdate(original, modified, apiservice)
			if err != nil {
				logrus.Errorf(":: error updating APIService %s -> request: %s", APIServiceName, patch)
				return err
			}
			return nil
		})
		if updateErr != nil {
			return fmt.Errorf(":: CreateOrUpdateAPIService :: failed to update APIService %s after retries: %w", APIServiceName, updateErr)
		}

	}

	return nil
}

func makePatchAndUpdate(original, modified *apiregv1.APIService, apiservice wranglerapiregistrationv1.APIServiceController) ([]byte, error) {
	originalJSON, err := json.Marshal(original)
	if err != nil {
		return nil, err
	}
	modifiedJSON, err := json.Marshal(modified)
	if err != nil {
		return nil, err
	}
	patch, err := jsonpatch.CreateMergePatch(originalJSON, modifiedJSON)
	if err != nil {
		return nil, err
	}
	if _, err := apiservice.Patch(APIServiceName, types.MergePatchType, patch); err != nil {
		return patch, err
	}
	return patch, nil
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

// AggregationPreCheck allows verifying if a previous execution of Rancher already checked API Agreggation works in the upstream cluster
func AggregationPreCheck(client wranglerapiregistrationv1.APIServiceClient) bool {
	apiservice, err := client.Get(APIServiceName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return apiservice.Annotations[annotationApiAggregationPreChecked] == "true"
}

// SetAggregationCheck adds an annotation in the extension APIService object, so it can later be retrieved by AggregationPreCheck
func SetAggregationCheck(client wranglerapiregistrationv1.APIServiceClient, value bool) error {
	return retry.OnError(retry.DefaultBackoff, func(err error) bool {
		if err != nil {
			logrus.Warnf("failed to update APIService annotation: %s", err)
			return true
		}

		return false
	}, func() error {
		apiservice, err := client.Get(APIServiceName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get APIService: %w", err)
		}

		previous := apiservice.Annotations[annotationApiAggregationPreChecked] == "true"
		if previous == value {
			return nil
		}

		if apiservice.Annotations == nil {
			apiservice.Annotations = make(map[string]string)
		}

		if value {
			apiservice.Annotations[annotationApiAggregationPreChecked] = "true"
		} else {
			apiservice.Annotations[annotationApiAggregationPreChecked] = "false"
		}

		if _, err := client.Update(apiservice); err != nil {
			return fmt.Errorf("failed to update APIService: %w", err)
		}

		return nil
	})
}
