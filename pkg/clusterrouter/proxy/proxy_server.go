package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
	"k8s.io/apimachinery/pkg/util/proxy"
)

type RemoteService struct {
	cluster   *v3.Cluster
	transport *http.Transport
	url       *url.URL
	auth      string
}

var (
	er = &errorResponder{}
)

type errorResponder struct {
}

func (e *errorResponder) Error(w http.ResponseWriter, req *http.Request, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err.Error()))
}

func prefix(cluster *v3.Cluster) string {
	return "/k8s/clusters/" + cluster.Name
}

func New(cluster *v3.Cluster, factory dialer.Factory) (*RemoteService, error) {
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

	u, err := url.Parse(cluster.Status.APIEndpoint)
	if err != nil {
		return nil, err
	}

	return &RemoteService{
		cluster:   cluster,
		transport: transport,
		url:       u,
		auth:      "Bearer " + cluster.Status.ServiceAccountToken,
	}, nil
}

func (r *RemoteService) Close() {
}

func (r *RemoteService) Handler() http.Handler {
	return r
}

func (r *RemoteService) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	u := *r.url
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
	req.Header.Set("Authorization", r.auth)

	httpProxy := proxy.NewUpgradeAwareHandler(&u, r.transport, true, false, er)
	httpProxy.ServeHTTP(rw, req)
}

func (r *RemoteService) Cluster() *v3.Cluster {
	return r.cluster
}
