package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/mattn/go-colorable"
	"github.com/rancher/rancher/pkg/agent/clean"
	"github.com/rancher/rancher/pkg/agent/cluster"
	"github.com/rancher/rancher/pkg/agent/node"
	"github.com/rancher/rancher/pkg/agent/rancher"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/logserver"
	"github.com/rancher/rancher/pkg/rkenodeconfigclient"
	"github.com/rancher/rancher/pkg/rkenodeconfigserver"
	"github.com/rancher/remotedialer"
	"github.com/rancher/shepherd/pkg/killserver"
	"github.com/rancher/wrangler/v2/pkg/signals"
	"github.com/sirupsen/logrus"
)

var (
	VERSION = "dev"
)

const (
	Token                    = "X-API-Tunnel-Token"
	KubeletCertValidityLimit = time.Hour * 72
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	killServer := killserver.NewKillServer(killserver.Port, cancel)

	go killServer.Start()
	go runAgent(ctx)
	<-ctx.Done()
}

func runAgent(ctx context.Context) {
	var err error

	if len(os.Args) > 1 {
		err = runArgs(ctx)
	} else {
		if _, err = reconcileKubelet(ctx); err != nil {
			logrus.Warnf("failed to reconcile kubelet, error: %v", err)
		}

		logrus.SetOutput(colorable.NewColorableStdout())
		logserver.StartServerWithDefaults()
		if os.Getenv("CATTLE_DEBUG") == "true" || os.Getenv("RANCHER_DEBUG") == "true" {
			logrus.SetLevel(logrus.DebugLevel)
		}

		initFeatures()

		if os.Getenv("CLUSTER_CLEANUP") == "true" {
			err = clean.Cluster()
		} else if os.Getenv("BINDING_CLEANUP") == "true" {
			var bindingErr error
			err = clean.DuplicateBindings(nil)
			if err != nil {
				bindingErr = errors.Join(bindingErr, err)
			}
			err = clean.OrphanBindings(nil)
			if err != nil {
				bindingErr = errors.Join(bindingErr, err)
			}
			err = clean.OrphanCatalogBindings(nil)
			if err != nil {
				bindingErr = errors.Join(bindingErr, err)
			}
			err = bindingErr
		} else {
			err = run(ctx)
		}
	}

	if err != nil {
		logrus.Fatal(err)
	}
}

func runArgs(ctx context.Context) error {
	switch os.Args[1] {
	case "clean":
		return clean.Run(ctx, os.Args)
	default:
		return run(ctx)
	}
}

func initFeatures() {
	features.InitializeFeatures(nil, os.Getenv("CATTLE_FEATURES"))
}

func isCluster() bool {
	return os.Getenv("CATTLE_CLUSTER") == "true"
}

func getParams() (map[string]interface{}, error) {
	if isCluster() {
		return cluster.Params()
	}
	return node.Params(), nil
}

func getTokenAndURL() (string, string, error) {
	token, url, err := node.TokenAndURL()
	if err != nil {
		return "", "", err
	}
	if token == "" {
		return cluster.TokenAndURL()
	}
	return token, url, nil
}

func isConnect() bool {
	if os.Getenv("CATTLE_AGENT_CONNECT") == "true" {
		return true
	}
	_, err := os.Stat("connected")
	return err == nil
}

func connected() {
	f, err := os.Create("connected")
	if err != nil {
		f.Close()
	}
}

func cleanup(ctx context.Context) error {
	if os.Getenv("CATTLE_K8S_MANAGED") != "true" {
		return nil
	}

	c, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.FromEnv)
	if err != nil {
		return err
	}
	defer c.Close()

	args := filters.NewArgs()
	args.Add("label", "io.cattle.agent=true")

	containers, err := c.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return err
	}

	for _, container := range containers {
		if _, ok := container.Labels["io.kubernetes.pod.namespace"]; ok {
			continue
		}

		if strings.Contains(container.Names[0], "share-mnt") {
			continue
		}

		container := container
		go func() {
			time.Sleep(15 * time.Second)
			logrus.Infof("Removing unmanaged agent %s(%s)", container.Names[0], container.ID)
			c.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{
				Force: true,
			})
		}()
	}

	return nil
}

func run(ctx context.Context) error {
	topContext := signals.SetupSignalContext()

	logrus.Infof("Rancher agent version %s is starting", VERSION)
	params, err := getParams()
	if err != nil {
		return err
	}
	writeCertsOnly := os.Getenv("CATTLE_WRITE_CERT_ONLY") == "true"
	bytes, err := json.Marshal(params)
	if err != nil {
		return err
	}

	token, server, err := getTokenAndURL()
	if err != nil {
		return err
	}

	headers := http.Header{
		Token:                      {token},
		rkenodeconfigclient.Params: {base64.StdEncoding.EncodeToString(bytes)},
	}

	err = KubeletNeedsNewCertificate(headers)
	if err != nil {
		return err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return err
	}

	// Check if secure connection can be made successfully
	var httpClient = &http.Client{
		Timeout: time.Second * 5,
	}

	_, err = httpClient.Get(server)
	if err != nil {
		if strings.Contains(err.Error(), "x509:") {
			certErr := err
			if strings.Contains(err.Error(), "certificate signed by unknown authority") {
				certErr = fmt.Errorf("Certificate chain is not complete, please check if all needed intermediate certificates are included in the server certificate (in the correct order) and if the cacerts setting in Rancher either contains the correct CA certificate (in the case of using self signed certificates) or is empty (in the case of using a certificate signed by a recognized CA). Certificate information is displayed above. error: %s", err)
			}
			if strings.Contains(err.Error(), "certificate has expired or is not yet valid") {
				certErr = fmt.Errorf("Server certificate is not valid, please check if the host has the correct time configured and if the server certificate has a notAfter date and time in the future. Certificate information is displayed above. error: %s", err)
			}
			if strings.Contains(err.Error(), "because it doesn't contain any IP SANs") || strings.Contains(err.Error(), "certificate is not valid for any names, but wanted to match") || strings.Contains(err.Error(), "cannot validate certificate for") {
				certErr = fmt.Errorf("Server certificate does not contain correct DNS and/or IP address entries in the Subject Alternative Names (SAN). Certificate information is displayed above. error: %s", err)
			}
			insecureClient := &http.Client{
				Timeout: time.Second * 5,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}
			res, err := insecureClient.Get(server)
			if err != nil {
				logrus.Errorf("Could not connect to %s: %v", server, err)
				os.Exit(1)
			}
			var lastFoundIssuer string
			if res.TLS != nil && len(res.TLS.PeerCertificates) > 0 {
				logrus.Infof("Certificate details from %s", serverURL)
				var previouscert *x509.Certificate
				for i := range res.TLS.PeerCertificates {
					cert := res.TLS.PeerCertificates[i]
					logrus.Infof("Certificate #%d (%s)", i, serverURL)
					certinfo(cert)
					if i > 0 {
						if previouscert.Issuer.String() != cert.Subject.String() {
							logrus.Errorf("Certficate's Subject (%s) does not match with previous certificate Issuer (%s). Please check if the configured server certificate contains all needed intermediate certificates and make sure they are in the correct order (server certificate first, intermediates after)", cert.Subject.String(), previouscert.Issuer.String())
						}
					}
					previouscert = cert
					lastFoundIssuer = cert.Issuer.String()
				}
			}
			caFileLocation := "/etc/kubernetes/ssl/certs/serverca"
			if _, err := os.Stat(caFileLocation); err == nil {
				caFile, err := os.ReadFile(caFileLocation)
				if err != nil {
					return err
				}
				var blocks [][]byte
				for {
					var certDERBlock *pem.Block
					certDERBlock, caFile = pem.Decode(caFile)
					if certDERBlock == nil {
						break
					}

					if certDERBlock.Type == "CERTIFICATE" {
						blocks = append(blocks, certDERBlock.Bytes)
					}
				}
				if len(blocks) > 1 {
					logrus.Warnf("Found %d certificates at %s, should be 1", len(blocks), caFileLocation)
				}
				logrus.Infof("Certificate details for %s", caFileLocation)

				blockcount := 0
				var lastCACert *x509.Certificate
				for _, block := range blocks {
					cert, err := x509.ParseCertificate(block)
					if err != nil {
						logrus.Println(err)
						continue
					}

					logrus.Infof("Certificate #%d (%s)", blockcount, caFileLocation)
					certinfo(cert)

					blockcount = blockcount + 1
					lastCACert = cert
				}
				if lastFoundIssuer != lastCACert.Issuer.String() {
					logrus.Errorf("Issuer of last certificate found in chain (%s) does not match with CA certificate Issuer (%s). Please check if the configured server certificate contains all needed intermediate certificates and make sure they are in the correct order (server certificate first, intermediates after)", lastFoundIssuer, lastCACert.Issuer.String())
				}
			}
			return certErr
		}
	}

	onConnect := func(ctx context.Context, _ *remotedialer.Session) error {
		connected()
		connectConfig := fmt.Sprintf("https://%s/v3/connect/config", serverURL.Host)
		interval, err := rkenodeconfigclient.ConfigClient(ctx, connectConfig, headers, writeCertsOnly)
		if err != nil {
			return err
		}

		if writeCertsOnly {
			exitCertWriter(ctx)
		}

		if isCluster() {
			err = rancher.Run(topContext)
			if err != nil {
				logrus.Fatal(err)
			}
			return nil
		}

		if err := cleanup(context.Background()); err != nil {
			logrus.Warnf("Unable to perform docker cleanup: %v", err)
		}

		go func() {
			logrus.Infof("Starting plan monitor, checking every %v seconds", interval)
			tt := time.Duration(interval) * time.Second
			for {
				select {
				case <-time.After(tt):
					// each time we request a plan we should
					// check if our cert about to expire
					err = KubeletNeedsNewCertificate(headers)
					if err != nil {
						logrus.Errorf("failed to check validity of kubelet certs: %v", err)
					}
					receivedInterval, err := rkenodeconfigclient.ConfigClient(ctx, connectConfig, headers, writeCertsOnly)
					if err != nil {
						logrus.Errorf("failed to check plan: %v", err)
					} else if receivedInterval != 0 && receivedInterval != interval {
						tt = time.Duration(receivedInterval) * time.Second
						logrus.Infof("Plan monitor checking %v seconds", receivedInterval)
					}

				case <-ctx.Done():
					return
				}
			}
		}()

		return nil
	}

	if isCluster() {
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	for {
		wsURL := fmt.Sprintf("wss://%s/v3/connect", serverURL.Host)
		if !isConnect() {
			wsURL += "/register"
		}

		// check if we need a new kubelet cert on reconnection
		err = KubeletNeedsNewCertificate(headers)
		if err != nil {
			return err
		}

		logrus.Infof("Connecting to %s with token starting with %s", wsURL, token[:len(token)/2])
		logrus.Tracef("Connecting to %s with token %s", wsURL, token)
		remotedialer.ClientConnect(ctx, wsURL, headers, nil, func(proto, address string) bool {
			switch proto {
			case "tcp":
				return true
			case "unix":
				return address == "/var/run/docker.sock"
			case "npipe":
				return address == "//./pipe/docker_engine"
			}
			return false
		}, onConnect)
		time.Sleep(5 * time.Second)
	}
}

func exitCertWriter(ctx context.Context) {
	// share-mnt process needs an always restart policy and to be killed so it can restart on startup
	// this functionality is really only needed for OSes with ephemeral /etc like RancherOS
	// everything here will just exit(0) with errors as we need to bail out completely.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)
	// trap SIGTERM here so that container can exit with 0
	go func() {
		<-sigs
		os.Exit(0)
	}()

	logrus.Info("attempting to stop the share-mnt container so it can reboot on startup")
	c, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.FromEnv)
	if err != nil {
		logrus.Error(err)
		os.Exit(0)
	}

	args := filters.NewArgs()
	args.Add("label", "io.rancher.rke.container.name=share-mnt")
	containers, err := c.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		logrus.Error(err)
		os.Exit(0)
	}

	for _, container := range containers {
		if len(container.Names) > 0 && strings.Contains(container.Names[0], "share-mnt") {
			err := c.ContainerKill(ctx, container.ID, "SIGTERM")
			if err != nil {
				logrus.Error(err)
				os.Exit(0) // only need to write certs so exit cleanly
			}
		}
	}
	// wait for itself to be kill with SIGTERM so it can return exit 0
	select {}
}

func certinfo(cert *x509.Certificate) {
	logrus.Infof("Subject: %+v", cert.Subject)
	logrus.Infof("Issuer: %+v", cert.Issuer)
	logrus.Infof("IsCA: %+v", cert.IsCA)
	if len(cert.DNSNames) > 0 {
		logrus.Infof("DNS Names: %+v", cert.DNSNames)
	} else {
		logrus.Infof("DNS Names: <none>")
	}
	if len(cert.IPAddresses) > 0 {
		logrus.Infof("IPAddresses: %+v", cert.IPAddresses)
	} else {
		logrus.Info("IPAddresses: <none>")
	}
	logrus.Infof("NotBefore: %+v", cert.NotBefore)
	logrus.Infof("NotAfter: %+v", cert.NotAfter)
	logrus.Infof("SignatureAlgorithm: %+v", cert.SignatureAlgorithm)
	logrus.Infof("PublicKeyAlgorithm: %+v", cert.PublicKeyAlgorithm)
}

// reconcileKubelet restarts kubelet in unmanaged agents
func reconcileKubelet(ctx context.Context) (bool, error) {
	if os.Getenv("CATTLE_K8S_MANAGED") == "true" {
		return true, nil
	}

	writeCertsOnly := os.Getenv("CATTLE_WRITE_CERT_ONLY") == "true"
	if writeCertsOnly {
		return true, nil
	}

	c, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.FromEnv)
	if err != nil {
		return false, err
	}
	defer c.Close()

	args := filters.NewArgs()
	args.Add("label", "io.rancher.rke.container.name=kubelet")

	containers, err := c.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return false, err
	}

	for _, container := range containers {
		if len(container.Names) > 0 && strings.Contains(container.Names[0], "kubelet") {
			nodeName := os.Getenv("CATTLE_NODE_NAME")
			logrus.Infof("node %v is not registered, restarting kubelet now", nodeName)
			if err := c.ContainerRestart(ctx, container.ID, nil); err != nil {
				return false, err
			}
			break
		}
	}
	return false, nil
}

// KubeletNeedsNewCertificate will set the
// 'RegenerateKubeletCertificate' header field to true if
// a) the kubelet serving certificate does not exist
// b) the certificate will expire in 72 hours
// c) the certificate does not accurately represent the
//
//	current IP address and Hostname of the node
//
// While the agent may denote it needs a new kubelet certificate
// in its connection request, a new certificate will only be
// delivered by Rancher if the generate_serving_certificate property
// is set to 'true' for the clusters kubelet service.
func KubeletNeedsNewCertificate(headers http.Header) error {
	currentHostname := os.Getenv("CATTLE_NODE_NAME")

	// RKE will save the certs on the node using either the public or private IP address, depending on the infrastructure provider.
	// For example, the certs will be stored using the public IP address for VM's on digital ocean, but will use the private IP
	// address for VM's on AWS. We do not know which IP address RKE decided to use, so we need to check both locations.
	kubeletCertFile, kubeletCertKeyFile, ipAddress := findCertificateFiles(os.Getenv("CATTLE_ADDRESS"), os.Getenv("CATTLE_INTERNAL_ADDRESS"))
	if kubeletCertFile == "" || kubeletCertKeyFile == "" || ipAddress == "" {
		logrus.Tracef("did not find kubelet certificate files using either public ip address (%s) or private ip address (%s)", os.Getenv("CATTLE_ADDRESS"), os.Getenv("CATTLE_INTERNAL_ADDRESS"))
	}

	cert, err := tls.LoadX509KeyPair(kubeletCertFile, kubeletCertKeyFile)
	if err != nil && !strings.Contains(err.Error(), "no such file") {
		return err
	}

	needsRegen, err := KubeletCertificateNeedsRegeneration(ipAddress, currentHostname, cert, time.Now())
	if err != nil {
		return err
	}

	if needsRegen {
		headers.Set(rkenodeconfigserver.RegenerateKubeletCertificate, "true")
	} else {
		headers.Set(rkenodeconfigserver.RegenerateKubeletCertificate, "false")
	}

	return nil
}

func findCertificateFiles(IPAddresses ...string) (string, string, string) {
	for _, ip := range IPAddresses {
		fileSafeIPAddress := strings.ReplaceAll(ip, ".", "-")
		certFile := fmt.Sprintf("/etc/kubernetes/ssl/kube-kubelet-%s.pem", fileSafeIPAddress)
		certKeyFile := fmt.Sprintf("/etc/kubernetes/ssl/kube-kubelet-%s-key.pem", fileSafeIPAddress)
		_, certErr := os.Stat(certFile)
		_, keyErr := os.Stat(certKeyFile)
		// check that both files exist
		if certErr == nil && keyErr == nil {
			return certFile, certKeyFile, ip
		}
	}
	return "", "", ""
}

func KubeletCertificateNeedsRegeneration(ipAddress, currentHostname string, cert tls.Certificate, currentTime time.Time) (bool, error) {
	if len(cert.Certificate) == 0 {
		return true, nil
	}

	parsedCert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return false, err
	}

	if !CertificateIncludesHostname(currentHostname, parsedCert) {
		logrus.Tracef("certificate does not include current hostname, requesting new certificate")
		return true, nil
	}

	if CertificateIsExpiring(parsedCert, currentTime) {
		logrus.Tracef("certificate is expiring soon, requesting new certificate")
		return true, nil
	}

	if !CertificateIncludesCurrentIP(ipAddress, parsedCert) {
		logrus.Tracef("certificate does not include current IP address, requesting new certificate")
		return true, nil
	}

	return false, nil
}

// CertificateIsExpiring checks if the passed certificate will expire within
// the KubeletCertValidityLimit
func CertificateIsExpiring(cert *x509.Certificate, currentTime time.Time) bool {
	return cert.NotAfter.Sub(currentTime) < KubeletCertValidityLimit
}

// CertificateIncludesHostname checks that the passed certificate includes
// the provided hostname in its SAN list
func CertificateIncludesHostname(hostname string, cert *x509.Certificate) bool {
	for _, name := range cert.DNSNames {
		if name == hostname {
			return true
		}
	}
	return false
}

// CertificateIncludesCurrentIP checks that the passed certificate includes the provided IP address
func CertificateIncludesCurrentIP(ipAddress string, cert *x509.Certificate) bool {
	for _, ip := range cert.IPAddresses {
		if ipAddress == ip.String() {
			return true
		}
	}
	return false
}
