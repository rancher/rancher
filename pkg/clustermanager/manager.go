package clustermanager

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	clusterController "github.com/rancher/rancher/pkg/controllers/user"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/config/dialer"
	"github.com/sirupsen/logrus"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/cert"
)

type Manager struct {
	httpsPort     int
	ScaledContext *config.ScaledContext
	clusterLister v3.ClusterLister
	controllers   sync.Map
	accessControl types.AccessControl
	dialer        dialer.Factory
}

type record struct {
	clusterRec    *v3.Cluster
	cluster       *config.UserContext
	accessControl types.AccessControl
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewManager(httpsPort int, context *config.ScaledContext) *Manager {
	return &Manager{
		httpsPort:     httpsPort,
		ScaledContext: context,
		dialer:        context.Dialer,
		accessControl: rbac.NewAccessControl(context.RBAC),
		clusterLister: context.Management.Clusters("").Controller().Lister(),
	}
}

func (m *Manager) Stop(cluster *v3.Cluster) {
	obj, ok := m.controllers.Load(cluster.UID)
	if !ok {
		return
	}
	logrus.Info("Stopping cluster agent for", obj.(*record).cluster.ClusterName)
	obj.(*record).cancel()
	m.controllers.Delete(cluster.UID)
}

func (m *Manager) Start(ctx context.Context, cluster *v3.Cluster) error {
	// reload cluster, always use the cached one
	cluster, err := m.clusterLister.Get("", cluster.Name)
	if err != nil {
		return err
	}
	_, err = m.start(ctx, cluster)
	return err
}

func (m *Manager) RESTConfig(cluster *v3.Cluster) (rest.Config, error) {
	obj, ok := m.controllers.Load(cluster.UID)
	if !ok {
		return rest.Config{}, fmt.Errorf("cluster record not found %s %s", cluster.Name, cluster.UID)
	}

	record := obj.(*record)
	return record.cluster.RESTConfig, nil
}

func (m *Manager) start(ctx context.Context, cluster *v3.Cluster) (*record, error) {
	obj, ok := m.controllers.Load(cluster.UID)
	if ok {
		if !m.changed(obj.(*record), cluster) {
			return obj.(*record), nil
		}
		m.Stop(obj.(*record).clusterRec)
	}

	controller, err := m.toRecord(ctx, cluster)
	if err != nil {
		return nil, err
	}
	if controller == nil {
		return nil, httperror.NewAPIError(httperror.ClusterUnavailable, "cluster not found")
	}

	obj, loaded := m.controllers.LoadOrStore(cluster.UID, controller)
	if !loaded {
		go func() {
			if err := m.doStart(obj.(*record)); err != nil {
				m.Stop(cluster)
			}
		}()
	}

	return obj.(*record), nil
}

func (m *Manager) changed(r *record, cluster *v3.Cluster) bool {
	existing := r.clusterRec
	if existing.Status.APIEndpoint != cluster.Status.APIEndpoint ||
		existing.Status.ServiceAccountToken != cluster.Status.ServiceAccountToken ||
		existing.Status.CACert != cluster.Status.CACert {
		return true
	}

	return false
}

func (m *Manager) doStart(rec *record) error {
	logrus.Info("Starting cluster agent for", rec.cluster.ClusterName)
	if err := clusterController.Register(rec.ctx, rec.cluster, m, m); err != nil {
		return err
	}
	return rec.cluster.Start(rec.ctx)
}

func (m *Manager) toRESTConfig(cluster *v3.Cluster) (*rest.Config, error) {
	if cluster == nil {
		return nil, nil
	}

	if cluster.DeletionTimestamp != nil {
		return nil, nil
	}

	if cluster.Spec.Internal {
		return m.ScaledContext.LocalConfig, nil
	}

	if cluster.Status.APIEndpoint == "" || cluster.Status.CACert == "" || cluster.Status.ServiceAccountToken == "" {
		return nil, nil
	}

	if !v3.ClusterConditionProvisioned.IsTrue(cluster) {
		return nil, nil
	}

	u, err := url.Parse(cluster.Status.APIEndpoint)
	if err != nil {
		return nil, err
	}

	caBytes, err := base64.StdEncoding.DecodeString(cluster.Status.CACert)
	if err != nil {
		return nil, err
	}

	clusterDialer, err := m.dialer.ClusterDialer(cluster.Name)
	if err != nil {
		return nil, err
	}

	var tlsDialer dialer.Dialer
	if cluster.Status.Driver == v3.ClusterDriverRKE {
		tlsDialer, err = nameIgnoringTLSDialer(clusterDialer, caBytes)
		if err != nil {
			return nil, err
		}
	}

	rc := &rest.Config{
		Host:        u.Host,
		Prefix:      u.Path,
		BearerToken: cluster.Status.ServiceAccountToken,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: caBytes,
		},
		Timeout: 30 * time.Second,
		WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
			if ht, ok := rt.(*http.Transport); ok {
				ht.DialContext = nil
				ht.DialTLS = tlsDialer
				ht.Dial = clusterDialer
			}
			return rt
		},
	}

	return rc, nil
}

func nameIgnoringTLSDialer(dialer dialer.Dialer, caBytes []byte) (dialer.Dialer, error) {
	rkeVerify, err := VerifyIgnoreDNSName(caBytes)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		// Use custom TLS validate that validates the cert chain, but not the server.  This should be secure because
		// we use a private per cluster CA always for RKE
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: rkeVerify,
	}

	return func(network, address string) (net.Conn, error) {
		rawConn, err := dialer(network, address)
		if err != nil {
			return nil, err
		}
		tlsConn := tls.Client(rawConn, tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			rawConn.Close()
			return nil, err
		}
		return tlsConn, err
	}, nil
}

func VerifyIgnoreDNSName(caCertsPEM []byte) (func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error, error) {
	rootCAs := x509.NewCertPool()
	if len(caCertsPEM) > 0 {
		caCerts, err := cert.ParseCertsPEM(caCertsPEM)
		if err != nil {
			return nil, err
		}
		for _, cert := range caCerts {
			rootCAs.AddCert(cert)
		}
	}

	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		certs := make([]*x509.Certificate, len(rawCerts))
		for i, asn1Data := range rawCerts {
			cert, err := x509.ParseCertificate(asn1Data)
			if err != nil {
				return fmt.Errorf("failed to parse cert")
			}
			certs[i] = cert
		}

		opts := x509.VerifyOptions{
			Roots:         rootCAs,
			CurrentTime:   time.Now(),
			DNSName:       "",
			Intermediates: x509.NewCertPool(),
		}

		for i, cert := range certs {
			if i == 0 {
				continue
			}
			opts.Intermediates.AddCert(cert)
		}
		_, err := certs[0].Verify(opts)
		return err
	}, nil
}

func (m *Manager) toRecord(ctx context.Context, cluster *v3.Cluster) (*record, error) {
	kubeConfig, err := m.toRESTConfig(cluster)
	if kubeConfig == nil || err != nil {
		return nil, err
	}

	clusterContext, err := config.NewUserContext(m.ScaledContext, *kubeConfig, cluster.Name)
	if err != nil {
		return nil, err
	}

	s := &record{
		cluster:       clusterContext,
		clusterRec:    cluster,
		accessControl: rbac.NewAccessControl(clusterContext.RBAC),
	}
	s.ctx, s.cancel = context.WithCancel(ctx)

	return s, nil
}

func (m *Manager) AccessControl(apiContext *types.APIContext, storageContext types.StorageContext) (types.AccessControl, error) {
	record, err := m.record(apiContext, storageContext)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return m.accessControl, nil
	}
	return record.accessControl, nil
}

func (m *Manager) Config(apiContext *types.APIContext, storageContext types.StorageContext) (rest.Config, error) {
	record, err := m.record(apiContext, storageContext)
	if err != nil {
		return rest.Config{}, err
	}
	if record == nil {
		return m.ScaledContext.RESTConfig, nil
	}
	return record.cluster.RESTConfig, nil
}

func (m *Manager) UnversionedClient(apiContext *types.APIContext, storageContext types.StorageContext) (rest.Interface, error) {
	record, err := m.record(apiContext, storageContext)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return m.ScaledContext.UnversionedClient, nil
	}
	return record.cluster.UnversionedClient, nil
}

func (m *Manager) APIExtClient(apiContext *types.APIContext, storageContext types.StorageContext) (clientset.Interface, error) {
	record, err := m.record(apiContext, storageContext)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return m.ScaledContext.APIExtClient, nil
	}
	return record.cluster.APIExtClient, nil
}

func (m *Manager) UserContext(clusterName string) (*config.UserContext, error) {
	cluster, err := m.clusterLister.Get("", clusterName)
	if err != nil {
		return nil, err
	}

	record, err := m.start(context.Background(), cluster)
	if err != nil || record == nil {
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		return nil, httperror.NewAPIError(httperror.ClusterUnavailable, msg)
	}

	if record == nil {
		return nil, httperror.NewAPIError(httperror.NotFound, "failed to find cluster")
	}

	return record.cluster, nil
}

func (m *Manager) record(apiContext *types.APIContext, storageContext types.StorageContext) (*record, error) {
	if apiContext == nil {
		return nil, nil
	}
	cluster, err := m.cluster(apiContext, storageContext)
	if err != nil {
		return nil, httperror.NewAPIError(httperror.ClusterUnavailable, err.Error())
	}
	if cluster == nil {
		return nil, nil
	}
	record, err := m.start(context.Background(), cluster)
	if err != nil {
		return nil, httperror.NewAPIError(httperror.ClusterUnavailable, err.Error())
	}

	return record, nil
}

func (m *Manager) ClusterName(apiContext *types.APIContext) string {
	clusterID := apiContext.SubContext["/v3/schemas/cluster"]
	if clusterID == "" {
		projectID, ok := apiContext.SubContext["/v3/schemas/project"]
		if ok {
			parts := strings.SplitN(projectID, ":", 2)
			if len(parts) == 2 {
				clusterID = parts[0]
			}
		}
	}
	return clusterID
}

func (m *Manager) cluster(apiContext *types.APIContext, context types.StorageContext) (*v3.Cluster, error) {
	switch context {
	case types.DefaultStorageContext:
		return nil, nil
	case config.ManagementStorageContext:
		return nil, nil
	case config.UserStorageContext:
	default:
		return nil, fmt.Errorf("illegal context: %s", context)

	}

	clusterID := m.ClusterName(apiContext)
	if clusterID == "" {
		return nil, nil
	}

	return m.clusterLister.Get("", clusterID)
}

func (m *Manager) KubeConfig(clusterName, token string) *clientcmdapi.Config {
	return &clientcmdapi.Config{
		CurrentContext: "default",
		APIVersion:     "v1",
		Kind:           "Config",
		Clusters: map[string]*clientcmdapi.Cluster{
			"default": {
				Server:                fmt.Sprintf("https://localhost:%d/k8s/clusters/%s", m.httpsPort, clusterName),
				InsecureSkipTLSVerify: true,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"default": {
				AuthInfo: "user",
				Cluster:  "default",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"user": {
				Token: token,
			},
		},
	}
}

func (m *Manager) GetHTTPSPort() int {
	return m.httpsPort
}
