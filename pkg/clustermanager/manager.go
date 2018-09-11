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
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/cert"
)

type Manager struct {
	httpsPort     int
	ScaledContext *config.ScaledContext
	clusterLister v3.ClusterLister
	clusters      v3.ClusterInterface
	controllers   sync.Map
	accessControl types.AccessControl
	dialer        dialer.Factory
}

type record struct {
	sync.Mutex
	clusterRec    *v3.Cluster
	cluster       *config.UserContext
	accessControl types.AccessControl
	started       bool
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
		clusters:      context.Management.Clusters(""),
	}
}

func (m *Manager) Stop(cluster *v3.Cluster) {
	obj, ok := m.controllers.Load(cluster.UID)
	if !ok {
		return
	}
	logrus.Infof("Stopping cluster agent for %s", obj.(*record).cluster.ClusterName)
	obj.(*record).cancel()
	m.controllers.Delete(cluster.UID)
}

func (m *Manager) Start(ctx context.Context, cluster *v3.Cluster, runAgent bool) error {
	if cluster.DeletionTimestamp != nil {
		return nil
	}
	// reload cluster, always use the cached one
	if runAgent {
		c, err := m.clusters.Get(cluster.Name, v1.GetOptions{})
		if err != nil {
			return err
		}
		cluster = c
	} else {
		c, err := m.clusterLister.Get("", cluster.Name)
		if err != nil {
			return err
		}
		cluster = c
	}

	_, err := m.start(ctx, cluster, true, runAgent)
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

func (m *Manager) markUnavailable(clusterName string) {
	if cluster, err := m.clusters.Get(clusterName, v1.GetOptions{}); err == nil {
		if !v3.ClusterConditionReady.IsFalse(cluster) {
			v3.ClusterConditionReady.False(cluster)
			m.clusters.Update(cluster)
		}
	}
}

func (m *Manager) start(ctx context.Context, cluster *v3.Cluster, controllers, runAgent bool) (*record, error) {
	obj, ok := m.controllers.Load(cluster.UID)
	if ok {
		if !m.changed(obj.(*record), cluster) {
			return obj.(*record), m.startController(obj.(*record), controllers, runAgent)
		}
		m.Stop(obj.(*record).clusterRec)
	}

	controller, err := m.toRecord(ctx, cluster, runAgent)
	if err != nil {
		m.markUnavailable(cluster.Name)
		return nil, err
	}
	if controller == nil {
		return nil, httperror.NewAPIError(httperror.ClusterUnavailable, "cluster not found")
	}

	obj, _ = m.controllers.LoadOrStore(cluster.UID, controller)
	return obj.(*record), m.startController(obj.(*record), controllers, runAgent)
}

func (m *Manager) startController(r *record, controllers bool, runAgent bool) error {
	if !controllers {
		return nil
	}

	r.Lock()
	defer r.Unlock()
	if !r.started {
		if err := m.doStart(r, runAgent); err != nil {
			m.Stop(r.clusterRec)
			return err
		}
		r.started = true
	}
	return nil
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

func (m *Manager) doStart(rec *record, runAgent bool) error {
	logrus.Info("Starting cluster agent for ", rec.cluster.ClusterName)
	if err := clusterController.Register(rec.ctx, rec.cluster, m, m, runAgent); err != nil {
		return err
	}
	return rec.cluster.Start(rec.ctx)
}

func (m *Manager) toRESTConfig(cluster *v3.Cluster, runAgent bool) (*rest.Config, error) {
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

	if runAgent {
		return rest.InClusterConfig()
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

	// adding suffix to make tlsConfig hashkey unique
	suffix := []byte("\n" + cluster.Name)
	rc := &rest.Config{
		Host:        u.String(),
		BearerToken: cluster.Status.ServiceAccountToken,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: append(caBytes, suffix...),
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

func (m *Manager) toRecord(ctx context.Context, cluster *v3.Cluster, runAgent bool) (*record, error) {
	kubeConfig, err := m.toRESTConfig(cluster, runAgent)
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
	return m.ScaledContext.APIExtClient, nil
}

func (m *Manager) UserContext(clusterName string) (*config.UserContext, error) {
	cluster, err := m.clusterLister.Get("", clusterName)
	if err != nil {
		return nil, err
	}

	record, err := m.start(context.Background(), cluster, false, false)
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
	record, err := m.start(context.Background(), cluster, false, false)
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
