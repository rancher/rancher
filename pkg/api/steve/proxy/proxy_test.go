package proxy_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/rancher/rancher/pkg/api/steve/proxy"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/remotedialer"
	"github.com/stretchr/testify/assert"
	authv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sUser "k8s.io/apiserver/pkg/authentication/user"
	k8sRequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes/scheme"
	typedv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

func TestLocalCluster(t *testing.T) {
	t.Parallel()
	const defaultResponseCode = 210 // so far an unused code, works to not set off any edge cases (is also in 200s)
	const defaultResponseMessage = "Default response"
	const defaultToken = "01020305081321345589" // token for testing, has shortened, random-like value
	const testUserUsername = "test-user"
	tests := []struct {
		name                   string
		requestPath            string
		userCanAccessLocal     bool
		desiredResponseCode    int
		desiredResponseMessage string
	}{
		{
			name:                   "no matching path",
			requestPath:            "/v1/not/a/path",
			userCanAccessLocal:     false,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "management crd allowed with local cluster access",
			requestPath:            "/apis/management.cattle.io/v3/test-crd",
			userCanAccessLocal:     true,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "management crd disallowed without local cluster access",
			requestPath:            "/apis/management.cattle.io/v3/fake-crd",
			userCanAccessLocal:     false,
			desiredResponseCode:    http.StatusForbidden,
			desiredResponseMessage: "",
		},
		{
			name:                   "management crd disallowed with individual resource without local cluster access",
			requestPath:            "/apis/management.cattle.io/v3/fake-crd/hello",
			userCanAccessLocal:     false,
			desiredResponseCode:    http.StatusForbidden,
			desiredResponseMessage: "",
		},
		{
			name:                   "management crd disallowed with individual resource's status without local cluster access",
			requestPath:            "/apis/management.cattle.io/v3/fake-crd/hello/status",
			userCanAccessLocal:     false,
			desiredResponseCode:    http.StatusForbidden,
			desiredResponseMessage: "",
		},
		{
			name:                   "management crd allowed without local cluster access",
			requestPath:            "/apis/management.cattle.io/v3/podsecurityadmissionconfigurationtemplates",
			userCanAccessLocal:     false,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "management crd allowed without local cluster access with trailing slash",
			requestPath:            "/apis/management.cattle.io/v3/podsecurityadmissionconfigurationtemplates/",
			userCanAccessLocal:     false,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "management crd allowed with local cluster access",
			requestPath:            "/apis/management.cattle.io/v3/podsecurityadmissionconfigurationtemplates",
			userCanAccessLocal:     true,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "management crd allowed with individual resource without local cluster access",
			requestPath:            "/apis/management.cattle.io/v3/podsecurityadmissionconfigurationtemplates/mytemplate",
			userCanAccessLocal:     false,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "management crd allowed with individual resource's status without local cluster access",
			requestPath:            "/apis/management.cattle.io/v3/podsecurityadmissionconfigurationtemplates/mytemplate/status",
			userCanAccessLocal:     false,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "core type without local cluster access",
			requestPath:            "/api/v1/pods",
			userCanAccessLocal:     false,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "core type with local cluster access",
			requestPath:            "/api/v1/pods",
			userCanAccessLocal:     true,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "non management type without local cluster access",
			requestPath:            "/apis/provisioning.cattle.io/v1/clusters",
			userCanAccessLocal:     false,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "non management type with local cluster access",
			requestPath:            "/apis/provisioning.cattle.io/v1/clusters",
			userCanAccessLocal:     true,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "discovery call for management resource without local cluster access",
			requestPath:            "/apis/management.cattle.io/v3",
			userCanAccessLocal:     false,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "discovery call for management resource with local cluster access",
			requestPath:            "/apis/management.cattle.io/v3",
			userCanAccessLocal:     true,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "discovery call for core resource without local cluster access",
			requestPath:            "/api/v1",
			userCanAccessLocal:     false,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "discovery call for core resource with local cluster access",
			requestPath:            "/api/v1",
			userCanAccessLocal:     true,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "management resource of v000 version with local cluster access",
			requestPath:            "/apis/management.cattle.io/v000/podsecurityadmissionconfigurationtemplates",
			userCanAccessLocal:     false,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "management resource of v000 version without local cluster access",
			requestPath:            "/apis/management.cattle.io/v000/podsecurityadmissionconfigurationtemplates",
			userCanAccessLocal:     true,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "bad path without local cluster access",
			requestPath:            "/apis/management.cattle.io/v3/podsecurityadmissionconfigurationtemplates/hello/world/foo/bar",
			userCanAccessLocal:     false,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "bad path with local cluster access",
			requestPath:            "/apis/management.cattle.io/v3/podsecurityadmissionconfigurationtemplates/hello/world/foo/bar/baz",
			userCanAccessLocal:     true,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "special support path without cluster access",
			requestPath:            "/healthz",
			userCanAccessLocal:     false,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "special support path with cluster access",
			requestPath:            "/healthz",
			userCanAccessLocal:     true,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
		{
			name:                   "short path",
			requestPath:            "/apis/hello",
			userCanAccessLocal:     false,
			desiredResponseCode:    defaultResponseCode,
			desiredResponseMessage: defaultResponseMessage,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			responder := DefaultHandler{
				ResponseCode:    defaultResponseCode,
				ResponseMessage: defaultResponseMessage,
			}

			localHandler := DefaultHandler{
				ResponseCode:    http.StatusNotFound,
				ResponseMessage: "local cluster routed",
			}

			reviewer := testReviewer{}
			if test.userCanAccessLocal {
				clusterGVR := schema.GroupVersionResource{
					Group:    managementv3.GroupName,
					Version:  managementv3.Version,
					Resource: "clusters",
				}
				reviewer.AddPermissionForUser(testUserUsername, "get", "", "local", clusterGVR)
			}

			// Note: Grants must be added before this next call
			server, err := NewSARServer(&reviewer, "/webhook")
			assert.NoError(t, err, "error when creating sar server")
			client, err := RestClientForURL(server.URL, defaultToken)
			assert.NoError(t, err, "error when creating rest client")
			sarWrapper := Authv1ClientInterface{Client: client}

			proxyMiddleware, err := proxy.NewProxyMiddleware(&sarWrapper, defaultDialer, nil, true, &localHandler)
			assert.NoError(t, err, "unable to construct proxy middleware")
			// construct the middleware with our default handler
			testHandler := proxyMiddleware(&responder)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("get", test.requestPath, bytes.NewReader([]byte{}))
			request = addUserToRequest(testUserUsername, request)
			testHandler.ServeHTTP(recorder, request)

			assert.Equal(t, test.desiredResponseCode, recorder.Code, "actual response code was different than expected")
			assert.Equal(t, test.desiredResponseMessage, recorder.Body.String(), "body was different than expected")
		})
	}
}

func addUserToRequest(username string, request *http.Request) *http.Request {
	currentContext := request.Context()
	user := &k8sUser.DefaultInfo{
		Name: username,
	}
	return request.WithContext(k8sRequest.WithUser(currentContext, user))
}

type DefaultHandler struct {
	ResponseCode    int
	ResponseMessage string
}

func (d *DefaultHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(d.ResponseCode)
	w.Write([]byte(d.ResponseMessage))
}

func defaultDialer(clusterID string) remotedialer.Dialer {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return nil, fmt.Errorf("unable to construct dialer for %s", address)
	}
}

type httpError struct {
	StatusCode int
	Message    string // Generally should be "HTTP Error". Use something else at your own risk
}

type responseStatus struct {
	Allowed         bool   `json:"allowed"`
	Reason          string `json:"reason"`
	EvaluationError string `json:"evaluationError"`
}

type response struct {
	APIVersion string         `json:"apiVersion"`
	Status     responseStatus `json:"status"`
}

type grant struct {
	username  string
	verb      string
	resource  schema.GroupVersionResource
	namespace string
	name      string
}

type testReviewer struct {
	grants []grant
}

func (t *testReviewer) Review(sar *authv1.SubjectAccessReview) (*response, *httpError) {
	sarGrant := grant{
		username: sar.Spec.User,
		verb:     sar.Spec.ResourceAttributes.Verb,
		resource: schema.GroupVersionResource{
			Group:    sar.Spec.ResourceAttributes.Group,
			Version:  sar.Spec.ResourceAttributes.Version,
			Resource: sar.Spec.ResourceAttributes.Resource,
		},
		namespace: sar.Spec.ResourceAttributes.Namespace,
		name:      sar.Spec.ResourceAttributes.Name,
	}
	for _, grant := range t.grants {
		if grant == sarGrant {
			return &response{
				APIVersion: authv1.SchemeGroupVersion.String(),
				Status: responseStatus{
					Allowed:         true,
					Reason:          "user has permissions",
					EvaluationError: "",
				},
			}, nil
		}
	}
	return &response{
		APIVersion: authv1.SchemeGroupVersion.String(),
		Status: responseStatus{
			Allowed: false,
			// TODO: More detailed reason
			Reason:          "No grant gives permissions",
			EvaluationError: "",
		},
	}, nil
}

func (t *testReviewer) AddPermissionForUser(username, verb, namespace, name string, resource schema.GroupVersionResource) {
	if t.grants == nil {
		t.grants = []grant{}
	}
	t.grants = append(t.grants, grant{
		username:  username,
		verb:      verb,
		name:      name,
		namespace: namespace,
		resource:  resource,
	})
}

// Adapted version of NewV1TestServer from https://github.com/kubernetes/apiserver/blob/06158e986473ead1397ab8dd7a17339430256999/plugin/pkg/authorizer/webhook/webhook_v1_test.go#L225
func NewSARServer(reviewer *testReviewer, rootPath string) (*httptest.Server, error) {
	serveHTTP := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, fmt.Sprintf("unexpected method %s", r.Method), http.StatusMethodNotAllowed)
			return
		}
		// if this request isn't for something at root path, return a not found error
		if !strings.HasPrefix(r.URL.Path, rootPath) {
			http.Error(w, fmt.Sprintf("unexpected path %s", r.URL.Path), http.StatusNotFound)
			return
		}
		var review authv1.SubjectAccessReview
		bodyData, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(bodyData, &review); err != nil {
			http.Error(w, fmt.Sprintf("failed to decode body %s", err), http.StatusBadRequest)
			return
		}

		if review.APIVersion != "authorization.k8s.io/v1" {
			http.Error(w, fmt.Sprintf("wrong api version %s", string(bodyData)), http.StatusBadRequest)
			return
		}
		resp, err := reviewer.Review(&review)
		if err != nil {
			http.Error(w, err.Message, err.StatusCode)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(*resp)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(serveHTTP))
	server.StartTLS()

	serverURL, _ := url.Parse(server.URL)
	serverURL.Path = rootPath
	server.URL = serverURL.String()

	return server, nil
}

// Simple implementation of the authv1.AuthorizationV1Interface which can just return a stored client
type Authv1ClientInterface struct {
	Client rest.Interface
}

func (c *Authv1ClientInterface) RESTClient() rest.Interface {
	return c.Client
}

func (c *Authv1ClientInterface) LocalSubjectAccessReviews(namespace string) typedv1.LocalSubjectAccessReviewInterface {
	return nil
}

func (c *Authv1ClientInterface) SelfSubjectAccessReviews() typedv1.SelfSubjectAccessReviewInterface {
	return nil
}

func (c *Authv1ClientInterface) SelfSubjectRulesReviews() typedv1.SelfSubjectRulesReviewInterface {
	return nil
}

func (c *Authv1ClientInterface) SubjectAccessReviews() typedv1.SubjectAccessReviewInterface {
	return nil
}

// RestClientForURL constructs a k8s rest client which has been configured to communicate with the server at serverURL
// using token as a source of auth
func RestClientForURL(serverURL string, token string) (rest.Interface, error) {
	config := v1.Config{
		Clusters: []v1.NamedCluster{
			{
				// skip tls here because our server implementation ignores this
				Cluster: v1.Cluster{Server: serverURL, InsecureSkipTLSVerify: true},
			},
		},
		AuthInfos: []v1.NamedAuthInfo{
			{
				AuthInfo: v1.AuthInfo{Token: token},
			},
		},
	}
	tempfile, err := os.CreateTemp("", "")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tempfile.Name())
	if err := json.NewEncoder(tempfile).Encode(config); err != nil {
		return nil, err
	}
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = tempfile.Name()
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	restConfig, err := loader.ClientConfig()
	if err != nil {
		return nil, err
	}
	restConfig.GroupVersion = &v1.SchemeGroupVersion
	restConfig.ContentConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	return rest.UnversionedRESTClientFor(restConfig)
}
