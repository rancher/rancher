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
	"io/ioutil"
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
	"github.com/rancher/rancher/pkg/agent/clean/adunmigration"
	"github.com/rancher/rancher/pkg/agent/cluster"
	"github.com/rancher/rancher/pkg/agent/node"
	"github.com/rancher/rancher/pkg/agent/rancher"
	"github.com/rancher/rancher/pkg/controllers/managementuser/cavalidator"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/logserver"
	"github.com/rancher/rancher/pkg/rkenodeconfigclient"
	"github.com/rancher/remotedialer"
	"github.com/rancher/wrangler/v3/pkg/signals"
	"github.com/sirupsen/logrus"
)

var (
	VERSION = "dev"
)

const (
	Token          = "X-API-Tunnel-Token"
	caFileLocation = "/etc/kubernetes/ssl/certs/serverca"
)

func main() {
	var err error
	ctx := context.Background()

	if len(os.Args) > 1 {
		err = runArgs(ctx)
	} else {
		if _, err = reconcileKubelet(ctx); err != nil {
			logrus.Warnf("failed to reconcile kubelet, error: %v", err)
		}

		configureLogrus()

		logserver.StartServerWithDefaults()

		initFeatures()

		if os.Getenv("CLUSTER_CLEANUP") == "true" {
			err = clean.Cluster()
		} else if os.Getenv("BINDING_CLEANUP") == "true" {
			err = errors.Join(
				clean.DuplicateBindings(nil),
				clean.OrphanBindings(nil),
				clean.OrphanCatalogBindings(nil),
			)
		} else if os.Getenv("AD_GUID_CLEANUP") == "true" {
			dryrun := os.Getenv("DRY_RUN") == "true"
			deleteMissingUsers := os.Getenv("AD_DELETE_MISSING_GUID_USERS") == "true"
			err = adunmigration.UnmigrateAdGUIDUsers(nil, dryrun, deleteMissingUsers)
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

	serverURL, err := url.Parse(server)
	if err != nil {
		return err
	}

	topContext = context.WithValue(topContext, cavalidator.CacertsValid, false)

	// Perform root CA verification
	var transport *http.Transport
	systemStoreConnectionCheckRequired := true
	transport = rootCATransport()
	if transport != nil {
		logrus.Infof("Testing connection to %s using trusted certificate authorities within: %s", server, caFileLocation)
		var httpClient = &http.Client{
			Timeout:   time.Second * 5,
			Transport: transport,
		}
		if _, err = httpClient.Get(server); err != nil {
			if cluster.CAStrictVerify() {
				logrus.Errorf("Could not securely connect to %s: %v", server, err)
				os.Exit(1)
			}
			// onConnect will use the transport later on, so discard it as it doesn't work and fallback to the system store.
			transport = nil
		} else {
			topContext = context.WithValue(topContext, cavalidator.CacertsValid, true)
			systemStoreConnectionCheckRequired = false
		}
	} else if cluster.CAStrictVerify() {
		logrus.Errorf("Strict CA verification is enabled but encountered error finding root CA")
		os.Exit(1)
	}

	if systemStoreConnectionCheckRequired {
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
				if _, err := os.Stat(caFileLocation); err == nil {
					caFile, err := ioutil.ReadFile(caFileLocation)
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
	}

	onConnect := func(ctx context.Context, _ *remotedialer.Session) error {
		connected()
		connectConfig := fmt.Sprintf("https://%s/v3/connect/config", serverURL.Host)
		httpClient := http.Client{
			Timeout: 300 * time.Second,
		}
		if transport != nil {
			httpClient.Transport = transport
		}
		interval, err := rkenodeconfigclient.ConfigClient(ctx, &httpClient, connectConfig, headers, writeCertsOnly)
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
					receivedInterval, err := rkenodeconfigclient.ConfigClient(ctx, &httpClient, connectConfig, headers, writeCertsOnly)
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

func configureLogrus() {
	logrus.SetOutput(colorable.NewColorableStdout())

	if os.Getenv("CATTLE_TRACE") == "true" || os.Getenv("RANCHER_TRACE") == "true" {
		logrus.SetLevel(logrus.TraceLevel)
	} else if os.Getenv("CATTLE_DEBUG") == "true" || os.Getenv("RANCHER_DEBUG") == "true" {
		logrus.SetLevel(logrus.DebugLevel)
	}
}

// rootCATransport generates a http.Transport that contains the contents of the CA file as the Root CA for strict validation.
func rootCATransport() *http.Transport {
	caFile, err := os.ReadFile(caFileLocation)
	if err != nil {
		logrus.Errorf("unable to read CA file from %s: %v", caFileLocation, err)
		return nil
	}
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(caFile); !ok {
		logrus.Errorf("unable to parse CA file %s", caFileLocation)
		return nil
	}
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: certPool,
		},
	}
}
