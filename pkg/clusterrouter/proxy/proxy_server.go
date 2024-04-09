package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/rancher/norman/httperror"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	dialer2 "github.com/rancher/rancher/pkg/dialer"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/impersonation"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	"github.com/rancher/wrangler/v2/pkg/schemas/validation"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/httpstream"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/rest"
)

type ClusterContextGetter interface {
	UserContext(string) (*config.UserContext, error)
}

type RemoteService struct {
	sync.Mutex

	cluster   *v3.Cluster
	transport transportGetter
	url       urlGetter

	factory              dialer.Factory
	clusterLister        v3.ClusterLister
	caCert               string
	localAuth            string
	httpTransport        *http.Transport
	clusterContextGetter ClusterContextGetter
}

var (
	er = &errorResponder{}
)

type urlGetter func() (url.URL, error)

type authGetter func() (string, error)

type transportGetter func() (http.RoundTripper, error)

type errorResponder struct {
}

func (e *errorResponder) Error(w http.ResponseWriter, req *http.Request, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err.Error()))
}

func prefix(cluster *v3.Cluster) string {
	return "/k8s/clusters/" + cluster.Name
}

func New(localConfig *rest.Config, cluster *v3.Cluster, clusterLister v3.ClusterLister, factory dialer.Factory, clusterContextGetter ClusterContextGetter) (*RemoteService, error) {
	if cluster.Spec.Internal {
		return NewLocal(localConfig, cluster)
	}
	return NewRemote(cluster, clusterLister, factory, clusterContextGetter)
}

func NewLocal(localConfig *rest.Config, cluster *v3.Cluster) (*RemoteService, error) {
	// the gvk is ignored by us, so just pass in any gvk
	hostURL, _, err := rest.DefaultServerURL(localConfig.Host, localConfig.APIPath, schema.GroupVersion{}, true)
	if err != nil {
		return nil, err
	}

	transport, err := rest.TransportFor(localConfig)
	if err != nil {
		return nil, err
	}

	transportGetter := func() (http.RoundTripper, error) {
		return transport, nil
	}

	rs := &RemoteService{
		cluster: cluster,
		url: func() (url.URL, error) {
			return *hostURL, nil
		},
		transport: transportGetter,
	}
	if localConfig.BearerToken != "" {
		rs.localAuth = "Bearer " + localConfig.BearerToken
	} else if localConfig.Password != "" {
		rs.localAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", localConfig.Username, localConfig.Password)))
	}

	return rs, nil
}

func NewRemote(cluster *v3.Cluster, clusterLister v3.ClusterLister, factory dialer.Factory, clusterContextGetter ClusterContextGetter) (*RemoteService, error) {
	if !v32.ClusterConditionProvisioned.IsTrue(cluster) {
		return nil, httperror.NewAPIError(httperror.ClusterUnavailable, "cluster not provisioned")
	}

	urlGetter := func() (url.URL, error) {
		newCluster, err := clusterLister.Get("", cluster.Name)
		if err != nil {
			return url.URL{}, err
		}

		u, err := url.Parse(newCluster.Status.APIEndpoint)
		if err != nil {
			return url.URL{}, err
		}
		return *u, nil
	}

	return &RemoteService{
		cluster:              cluster,
		url:                  urlGetter,
		clusterLister:        clusterLister,
		factory:              factory,
		clusterContextGetter: clusterContextGetter,
	}, nil
}

func (r *RemoteService) getTransport() (http.RoundTripper, error) {
	if r.transport != nil {
		return r.transport()
	}

	newCluster, err := r.clusterLister.Get("", r.cluster.Name)
	if err != nil {
		return nil, err
	}

	r.Lock()
	defer r.Unlock()

	if r.httpTransport != nil && !r.cacertChanged(newCluster) {
		return r.httpTransport, nil
	}

	transport := &http.Transport{}
	if newCluster.Status.CACert != "" {
		certBytes, err := base64.StdEncoding.DecodeString(newCluster.Status.CACert)
		if err != nil {
			return nil, err
		}
		certs := x509.NewCertPool()
		certs.AppendCertsFromPEM(certBytes)
		transport.TLSClientConfig = &tls.Config{
			RootCAs: certs,
		}
	}

	if r.factory != nil {
		d, err := r.factory.ClusterDialer(newCluster.Name, true)
		if err != nil {
			return nil, err
		}
		transport.DialContext = d
		if dialer2.IsPublicCloudDriver(newCluster) {
			transport.Proxy = http.ProxyFromEnvironment
		}
	}

	r.caCert = newCluster.Status.CACert
	if r.httpTransport != nil {
		r.httpTransport.CloseIdleConnections()
	}
	r.httpTransport = transport

	return transport, nil
}

func (r *RemoteService) cacertChanged(cluster *v3.Cluster) bool {
	return r.caCert != cluster.Status.CACert
}

func (r *RemoteService) Close() {
	if r.httpTransport != nil {
		r.httpTransport.CloseIdleConnections()
	}
}

func (r *RemoteService) Handler() http.Handler {
	return r
}

func (r *RemoteService) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	u, err := r.url()
	if err != nil {
		er.Error(rw, req, err)
		return
	}

	u.Path = strings.TrimPrefix(req.URL.Path, prefix(r.cluster))
	u.RawQuery = req.URL.RawQuery

	proto := req.Header.Get("X-Forwarded-Proto")
	if proto != "" {
		req.URL.Scheme = proto
	} else if req.TLS == nil {
		req.URL.Scheme = "http"
	} else {
		req.URL.Scheme = "https"
	}

	req.URL.Host = req.Host
	transport, err := r.getTransport()
	if err != nil {
		er.Error(rw, req, err)
		return
	}

	if r.cluster.Spec.Internal && r.localAuth == "" {
		req.Header.Del("Authorization")
	} else {
		userInfo, authed := request.UserFrom(req.Context())
		if !authed {
			er.Error(rw, req, validation.Unauthorized)
			return
		}
		token, err := r.getImpersonatorAccountToken(userInfo)
		if err != nil && !strings.Contains(err.Error(), dialer2.ErrAgentDisconnected.Error()) {
			er.Error(rw, req, fmt.Errorf("unable to create impersonator account: %w", err))
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if httpstream.IsUpgradeRequest(req) {
		upgradeProxy := NewUpgradeProxy(&u, transport)
		upgradeProxy.ServeHTTP(rw, req)
		return
	}

	httpProxy := proxy.NewUpgradeAwareHandler(&u, transport, true, false, er)
	httpProxy.ServeHTTP(rw, req)
}

func (r *RemoteService) Cluster() *v3.Cluster {
	return r.cluster
}

type UpgradeProxy struct {
	Location  *url.URL
	Transport http.RoundTripper
}

func NewUpgradeProxy(location *url.URL, transport http.RoundTripper) *UpgradeProxy {
	return &UpgradeProxy{
		Location:  location,
		Transport: transport,
	}
}

func (p *UpgradeProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	loc := *p.Location
	loc.RawQuery = req.URL.RawQuery

	newReq := req.WithContext(req.Context())
	newReq.Header = utilnet.CloneHeader(req.Header)
	newReq.URL = &loc

	httpProxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: p.Location.Scheme, Host: p.Location.Host})
	httpProxy.Transport = p.Transport
	httpProxy.ServeHTTP(rw, newReq)
}

// getImpersonatorAccountToken creates, if not already present, a service account and role bindings
// whose only permission is to impersonate the given user, and returns the bearer token for the account.
func (r *RemoteService) getImpersonatorAccountToken(user user.Info) (string, error) {
	clusterContext, err := r.clusterContextGetter.UserContext(r.cluster.Name)
	if err != nil {
		return "", err
	}

	i, err := impersonation.New(user, clusterContext)
	if err != nil {
		return "", fmt.Errorf("error creating impersonation for user %s: %w", user.GetUID(), err)
	}

	sa, err := i.SetUpImpersonation()
	if err != nil {
		return "", fmt.Errorf("error setting up impersonation for user %s: %w", user.GetUID(), err)
	}
	saToken, err := i.GetToken(sa)
	if err != nil {
		return "", fmt.Errorf("error getting service account token: %w", err)
	}

	return saToken, nil
}
