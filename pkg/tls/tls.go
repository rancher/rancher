package tls

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/cert"
	"github.com/rancher/dynamiclistener/server"
	"github.com/rancher/dynamiclistener/storage/kubernetes"
	"github.com/rancher/lasso/pkg/metrics"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/controllers/dashboard/apiservice"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/apps"
	appscontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	corev1controllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	rancherCertFile    = "/etc/rancher/ssl/cert.pem"
	rancherKeyFile     = "/etc/rancher/ssl/key.pem"
	rancherCACertsFile = "/etc/rancher/ssl/cacerts.pem"

	commonName = "rancher"
)

type internalAPI struct{}

var (
	InternalAPI = internalAPI{}
)

func ListenAndServe(ctx context.Context, restConfig *rest.Config, handler http.Handler, bindHost string, httpsPort, httpPort int, acmeDomains []string, noCACerts bool) error {
	restConfig = rest.CopyConfig(restConfig)
	restConfig.Timeout = 10 * time.Minute
	opts := &server.ListenOpts{}
	var err error

	core, err := core.NewFactoryFromConfig(restConfig)
	if err != nil {
		return err
	}
	apps, err := apps.NewFactoryFromConfig(restConfig)
	if err != nil {
		return err
	}

	if httpsPort != 0 {
		opts, err = SetupListener(core.Core().V1().Secret(), acmeDomains, noCACerts)
		if err != nil {
			return errors.Wrap(err, "failed to setup TLS listener")
		}
	}
	opts.DisplayServerLogs = true

	opts.BindHost = bindHost

	migrateConfig(ctx, restConfig, opts)

	backoff := wait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   2,
		Steps:    3,
	}

	// Try listen and serve over if there is an already exist error which comes from
	// creating the ca. Rancher will hit this error during HA startup as all servers
	// will race to create the ca secret.
	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		if err := server.ListenAndServe(ctx, httpsPort, httpPort, handler, opts); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to ListenAndServe")
	}

	internalPort := 0
	if httpsPort != 0 {
		internalPort = httpsPort + 1
	}

	serverOptions := &server.ListenOpts{
		Storage:           opts.Storage,
		Secrets:           opts.Secrets,
		CAName:            "tls-rancher-internal-ca",
		CANamespace:       namespace.System,
		CertNamespace:     namespace.System,
		CertName:          "tls-rancher-internal",
		DisplayServerLogs: true,
	}
	clusterIP, err := getClusterIP(core.Core().V1().Service())
	if err != nil {
		return err
	}
	hostIPs, err := getHostIPs(apps.Apps().V1().Deployment(), core.Core().V1().Node())
	if err != nil {
		return err
	}

	if clusterIP != "" {
		hostIPs = append(hostIPs, clusterIP)
	}
	if len(hostIPs) > 0 {
		serverOptions.TLSListenerConfig = dynamiclistener.Config{
			SANs: hostIPs,
		}
	}

	internalAPICtx := context.WithValue(ctx, InternalAPI, true)
	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		if err := server.ListenAndServe(internalAPICtx, internalPort, 0, handler, serverOptions); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to ListenAndServe for fleet")
	}

	ctx = metrics.WithContextID(ctx, "tlscontext")
	if err := core.Start(ctx, 5); err != nil {
		return err
	}

	<-ctx.Done()
	return ctx.Err()

}

func migrateConfig(ctx context.Context, restConfig *rest.Config, opts *server.ListenOpts) {
	c, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return
	}

	config, err := c.Resource(schema.GroupVersionResource{
		Group:    "management.cattle.io",
		Version:  "v3",
		Resource: "listenconfigs",
	}).Get(ctx, "cli-config", metav1.GetOptions{})
	if err != nil {
		return
	}

	known := convert.ToStringSlice(config.Object["knownIps"])
	for k := range convert.ToMapInterface(config.Object["generatedCerts"]) {
		if strings.HasPrefix(k, "local/") {
			continue
		}
		known = append(known, k)
	}

	for _, k := range known {
		k = strings.SplitN(k, ":", 2)[0]
		found := false
		for _, san := range opts.TLSListenerConfig.SANs {
			if san == k {
				found = true
				break
			}
		}
		if !found {
			opts.TLSListenerConfig.SANs = append(opts.TLSListenerConfig.SANs, k)
		}
	}
}

func SetupListener(secrets corev1controllers.SecretController, acmeDomains []string, noCACerts bool) (*server.ListenOpts, error) {
	caForAgent, noCACerts, opts, err := readConfig(secrets, acmeDomains, noCACerts)
	if err != nil {
		return nil, err
	}

	if noCACerts {
		caForAgent = ""
	} else if caForAgent == "" {
		caCert, caKey, err := kubernetes.LoadOrGenCA(secrets, opts.CANamespace, opts.CAName)
		if err != nil {
			return nil, err
		}
		caForAgent = string(cert.EncodeCertPEM(caCert))
		opts.CA = caCert
		opts.CAKey = caKey
	}

	caForAgent = strings.TrimSpace(caForAgent)
	if settings.CACerts.Get() != caForAgent {
		if err := settings.CACerts.Set(caForAgent); err != nil {
			return nil, err
		}
	}

	return opts, nil
}

// readConfig returns the TLS configuration for the Rancher server.
// It determines the source of the TLS certificates and CA bundle based on the environment and flags.
// The order of precedence for TLS configuration is:
// 1. ACME (Let's Encrypt): If acmeDomains are provided. Rancher will obtain a cert from Let's Encrypt.
// 2. User-Provided Certificates: If a cert and key are mounted into the container.
// 3. External Termination: If TLS is terminated by an upstream proxy/load-balancer. A private CA can be provided.
// 4. Rancher-Generated Self-Signed Certificates: If no other configuration is provided.
//
// It returns:
// - ca: The CA certificate chain in PEM format. Can be empty.
// - bool: The value of the noCACerts flag.
// - *server.ListenOpts: The listener options for dynamiclistener.
// - error: An error if the configuration is invalid.
func readConfig(secrets corev1controllers.SecretController, acmeDomains []string, noCACerts bool) (string, bool, *server.ListenOpts, error) {
	var (
		ca  string
		err error
	)

	// Create a base TLS config from settings for TLS version and ciphers.
	tlsConfig, err := baseTLSConfig(settings.TLSMinVersion.Get(), settings.TLSCiphers.Get())
	if err != nil {
		return "", noCACerts, nil, err
	}

	// Get the certificate rotation expiration setting.
	expiration, err := strconv.Atoi(settings.RotateCertsIfExpiringInDays.Get())
	if err != nil {
		return "", noCACerts, nil, errors.Wrapf(err, "parsing %s", settings.RotateCertsIfExpiringInDays.Get())
	}

	// Default SANs for the Rancher-generated certificate.
	sans := []string{"localhost", "127.0.0.1", "rancher.cattle-system"}
	ip, err := net.ChooseHostInterface()
	if err == nil {
		sans = append(sans, ip.String())
	}

	// Initialize listener options. These are used for all TLS configuration types, but some fields
	// are only relevant for Rancher-generated certificates.
	opts := &server.ListenOpts{
		Secrets:       secrets,
		CAName:        "tls-rancher",
		CANamespace:   "cattle-system",
		CertNamespace: "cattle-system",
		AcmeDomains:   acmeDomains,
		TLSListenerConfig: dynamiclistener.Config{
			TLSConfig:             tlsConfig,
			ExpirationDaysCheck:   expiration,
			SANs:                  sans,
			FilterCN:              filterCN,
			CloseConnOnCertChange: true,
		},
	}

	// Case 1: ACME (Let's Encrypt).
	// If --acme-domain is set, Rancher will manage the certificate via ACME.
	// In this mode, Rancher does not use a custom CA, so we return an empty CA string
	// and indicate that no CA certs are expected (noCACerts=true).
	if len(acmeDomains) > 0 {
		return "", true, opts, nil
	}

	// Case 2: User-provided certificates mounted into the container.
	// Check if certificate and key files are mounted at the standard locations.
	certFileExists := fileExists(rancherCertFile)
	keyFileExists := fileExists(rancherKeyFile)

	// Both certificate and key must be present, or neither. A mismatch is a configuration error.
	if (certFileExists && !keyFileExists) || (!certFileExists && keyFileExists) {
		return "", noCACerts, nil, fmt.Errorf("invalid SSL configuration found, please set both certificate file and certificate key file (one is missing)")
	}

	caFileExists := fileExists(rancherCACertsFile)

	// If both certificate and key files exist, we use this user-provided cert.
	if certFileExists && keyFileExists {
		cert, err := tls.LoadX509KeyPair(rancherCertFile, rancherKeyFile)
		if err != nil {
			return "", noCACerts, nil, err
		}
		opts.TLSListenerConfig.TLSConfig.Certificates = []tls.Certificate{cert}

		// When using custom certificates, the user must either provide a CA bundle (for self-signed certs)
		// or explicitly state there is no CA bundle with --no-cacerts (for certs from a recognized CA).
		// It's an error to provide a CA bundle AND --no-cacerts.
		// It's also an error to NOT provide a CA bundle AND NOT use --no-cacerts, as Rancher can't
		// determine if the cert is self-signed or not.
		if (caFileExists && noCACerts) || (!caFileExists && !noCACerts) {
			return "", noCACerts, nil, fmt.Errorf("invalid SSL configuration found, please set cacerts when using self signed certificates or use --no-cacerts when using certificates from a recognized Certificate Authority, do not use both at the same time")
		}
		// If a CA file is provided, read it. This is the CA for the provided cert.
		if caFileExists {
			ca, err = readPEM(rancherCACertsFile)
			if err != nil {
				return "", noCACerts, nil, err
			}
		}
		// Return the custom CA (if any) and the listener options with the custom cert.
		return ca, noCACerts, opts, nil
	}

	// Case 3: External termination.
	// This case is for when TLS is terminated by an external load balancer or proxy, and Rancher itself
	// does not handle TLS termination for external traffic. A private CA may be provided so that
	// Rancher and its agents can trust the endpoint. The '/cacerts' endpoint will return this CA.
	if caFileExists {
		// It's a configuration error to provide a CA for external termination but also specify --no-cacerts.
		if noCACerts {
			return "", noCACerts, nil, fmt.Errorf("invalid SSL configuration found, please set cacerts when using self signed certificates or use --no-cacerts when using certificates from a recognized Certificate Authority, do not use both at the same time")
		}
		ca, err = readPEM(rancherCACertsFile)
		if err != nil {
			return "", noCACerts, nil, err
		}
	}

	// Case 4: Rancher-generated self-signed certs OR external termination without a private CA.
	// If we reach this point, it means no ACME and no mounted certs were found.
	// The 'ca' variable will contain the content of 'rancherCACertsFile' if it existed, otherwise it's empty.
	// If 'ca' is empty and 'noCACerts' is false, the caller ('SetupListener') will generate a new self-signed CA.
	// This generated CA is often referred to as the "Internal certificate". If you are seeing this
	// certificate unexpectedly, it's likely because 'rancherCACertsFile' was not found in the container,
	// even though you intended to provide a private CA.
	return ca, noCACerts, opts, nil
}

// getClusterIP retrieves the ClusterIP of the rancher-internal service.
// It returns the ClusterIP as a string, or an error if the service exists but is not assigned an IP.
// If the service does not exist, it returns an empty string and no error.
func getClusterIP(services corev1controllers.ServiceController) (string, error) {
	service, err := services.Get(namespace.System, apiservice.RancherInternalServiceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	if service.Spec.ClusterIP == "" {
		return "", fmt.Errorf("waiting on service %s/%s to be assigned a ClusterIP",
			namespace.System, apiservice.RancherInternalServiceName)
	}
	return service.Spec.ClusterIP, nil
}

func getHostIPs(deployments appscontrollers.DeploymentController, nodes corev1controllers.NodeController) ([]string, error) {
	deployment, err := deployments.Get(namespace.System, commonName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name != commonName {
			continue
		}
		for _, port := range container.Ports {
			if port.HostIP != "" || port.HostPort != 0 {
				return collectNodeIPs(nodes)
			}
		}
	}
	return nil, nil
}

func collectNodeIPs(nodeController corev1controllers.NodeController) ([]string, error) {
	nodes, err := nodeController.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	nodeIPs := make([]string, 0, 2*len(nodes.Items))
	for _, node := range nodes.Items {
		for _, ip := range node.Status.Addresses {
			if ip.Type == v1.NodeInternalIP || ip.Type == v1.NodeExternalIP {
				nodeIPs = append(nodeIPs, ip.Address)
			}
		}
	}

	return nodeIPs, nil
}

func filterCN(cns ...string) []string {
	serverURL := settings.ServerURL.Get()
	if serverURL == "" {
		return cns
	}
	u, err := url.Parse(serverURL)
	if err != nil {
		logrus.Errorf("invalid server-url, can not parse %s: %v", serverURL, err)
		return cns
	}
	host := u.Hostname()
	if host != "" {
		return []string{host}
	}
	return cns
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func readPEM(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}
