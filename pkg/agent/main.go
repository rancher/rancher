package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/mattn/go-colorable"
	"github.com/rancher/rancher/pkg/agent/clean"
	"github.com/rancher/rancher/pkg/agent/cluster"
	"github.com/rancher/rancher/pkg/agent/node"
	"github.com/rancher/rancher/pkg/logserver"
	"github.com/rancher/rancher/pkg/rkenodeconfigclient"
	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
)

var (
	VERSION = "dev"
)

const (
	Token  = "X-API-Tunnel-Token"
	Params = "X-API-Tunnel-Params"
)

func main() {
	logrus.SetOutput(colorable.NewColorableStdout())
	logserver.StartServerWithDefaults()
	if os.Getenv("CATTLE_DEBUG") == "true" || os.Getenv("RANCHER_DEBUG") == "true" {
		logrus.SetLevel(logrus.DebugLevel)
	}

	var err error

	if os.Getenv("CLUSTER_CLEANUP") == "true" {
		err = clean.Cluster()
	} else {
		err = run()
	}

	if err != nil {
		logrus.Fatal(err)
	}
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

	c, err := client.NewEnvClient()
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

func run() error {
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

	headers := map[string][]string{
		Token:  {token},
		Params: {base64.StdEncoding.EncodeToString(bytes)},
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

	onConnect := func(ctx context.Context) error {
		connected()
		connectConfigURL := fmt.Sprintf("https://%s/v3/connect/config", serverURL.Host)
		upgradeStatusURL := fmt.Sprintf("https://%s/v3/connect/upgradestatus", serverURL.Host)
		if err := rkenodeconfigclient.ConfigClient(ctx, connectConfigURL, upgradeStatusURL, headers, writeCertsOnly); err != nil {
			return err
		}

		if isCluster() {
			err = cluster.RunControllers()
			if err != nil {
				logrus.Fatal(err)
			}
			return nil
		}

		if err := cleanup(context.Background()); err != nil {
			logrus.Warnf("Unable to perform docker cleanup: %v", err)
		}

		go func() {
			logrus.Infof("Starting plan monitor")
			for {
				select {
				case <-time.After(2 * time.Minute):
					err := rkenodeconfigclient.ConfigClient(ctx, connectConfigURL, upgradeStatusURL, headers, writeCertsOnly)
					if err != nil {
						logrus.Errorf("failed to check plan: %v", err)
					}
				case <-ctx.Done():
					return
				}
			}
		}()

		return nil
	}

	for {
		wsURL := fmt.Sprintf("wss://%s/v3/connect", serverURL.Host)
		if !isConnect() {
			wsURL += "/register"
		}
		logrus.Infof("Connecting to %s with token %s", wsURL, token)
		remotedialer.ClientConnect(context.Background(), wsURL, http.Header(headers), nil, func(proto, address string) bool {
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
