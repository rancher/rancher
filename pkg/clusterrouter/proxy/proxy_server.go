package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/rest"
)

type RemoteService struct {
	cluster   *v3.Cluster
	transport http.RoundTripper
	url       urlGetter
	auth      string
}

var (
	er = &errorResponder{}
)

type urlGetter func() (url.URL, error)

type errorResponder struct {
}

func (e *errorResponder) Error(w http.ResponseWriter, req *http.Request, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err.Error()))
}

func prefix(cluster *v3.Cluster) string {
	return "/k8s/clusters/" + cluster.Name
}

func New(localConfig *rest.Config, cluster *v3.Cluster, clusterLister v3.ClusterLister, factory dialer.Factory) (*RemoteService, error) {
	if cluster.Spec.Internal {
		return NewLocal(localConfig, cluster)
	}
	return NewRemote(cluster, clusterLister, factory)
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

	rs := &RemoteService{
		cluster: cluster,
		url: func() (url.URL, error) {
			return *hostURL, nil
		},
		transport: transport,
	}
	if localConfig.BearerToken != "" {
		rs.auth = "Bearer " + localConfig.BearerToken
	}

	return rs, nil
}

func NewRemote(cluster *v3.Cluster, clusterLister v3.ClusterLister, factory dialer.Factory) (*RemoteService, error) {
	if !v3.ClusterConditionProvisioned.IsTrue(cluster) {
		return nil, httperror.NewAPIError(httperror.ClusterUnavailable, "cluster not provisioned")
	}

	transport := &http.Transport{}

	if factory != nil {
		d, err := factory.ClusterDialer(cluster.Name)
		if err != nil {
			return nil, err
		}
		transport.Dial = d
	}

	if cluster.Status.CACert != "" {
		certBytes, err := base64.StdEncoding.DecodeString(cluster.Status.CACert)
		if err != nil {
			return nil, err
		}
		certs := x509.NewCertPool()
		certs.AppendCertsFromPEM(certBytes)
		transport.TLSClientConfig = &tls.Config{
			RootCAs: certs,
		}
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
		cluster:   cluster,
		transport: transport,
		url:       urlGetter,
		auth:      "Bearer " + cluster.Status.ServiceAccountToken,
	}, nil
}

func (r *RemoteService) Close() {
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
	if r.auth != "" {
		req.Header.Set("Authorization", r.auth)
	}

	httpProxy := proxy.NewUpgradeAwareHandler(&u, r.transport, true, false, er)
	httpProxy.ServeHTTP(rw, req)
}

func (r *RemoteService) Cluster() *v3.Cluster {
	return r.cluster
}

type SimpleProxy struct {
	url       *url.URL
	transport http.RoundTripper
}

func NewSimpleProxy(host string, caData []byte) (*SimpleProxy, error) {
	hostURL, _, err := rest.DefaultServerURL(host, "", schema.GroupVersion{}, true)
	if err != nil {
		return nil, err
	}

	ht := &http.Transport{}
	if len(caData) > 0 {
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(caData)
		ht.TLSClientConfig = &tls.Config{
			RootCAs: certPool,
		}
	}

	return &SimpleProxy{
		url:       hostURL,
		transport: ht,
	}, nil
}

func (s *SimpleProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	u := *s.url
	u.Path = req.URL.Path
	u.RawQuery = req.URL.RawQuery
	req.URL.Scheme = "https"
	req.URL.Host = req.Host
	httpProxy := proxy.NewUpgradeAwareHandler(&u, s.transport, true, false, er)
	httpProxy.ServeHTTP(rw, req)

}
