package services

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/pki/cert"
	"github.com/sirupsen/logrus"
)

const (
	HealthzAddress   = "localhost"
	HealthzEndpoint  = "/healthz"
	HTTPProtoPrefix  = "http://"
	HTTPSProtoPrefix = "https://"
)

func runHealthcheck(ctx context.Context, host *hosts.Host, serviceName string, localConnDialerFactory hosts.DialerFactory, url string, certMap map[string]pki.CertificatePKI) error {
	log.Infof(ctx, "[healthcheck] Start Healthcheck on service [%s] on host [%s]", serviceName, host.Address)
	var x509Pair tls.Certificate

	port, err := getPortFromURL(url)
	if err != nil {
		return err
	}
	if serviceName == KubeAPIContainerName {
		certificate := cert.EncodeCertPEM(certMap[pki.KubeAPICertName].Certificate)
		key := cert.EncodePrivateKeyPEM(certMap[pki.KubeAPICertName].Key)
		x509Pair, err = tls.X509KeyPair(certificate, key)
		if err != nil {
			return err
		}
	}
	client, err := getHealthCheckHTTPClient(host, port, localConnDialerFactory, &x509Pair)
	if err != nil {
		return fmt.Errorf("Failed to initiate new HTTP client for service [%s] for host [%s]: %v", serviceName, host.Address, err)
	}
	for retries := 0; retries < 10; retries++ {
		if err = getHealthz(client, serviceName, host.Address, url); err != nil {
			logrus.Debugf("[healthcheck] %v, try #%v", err, retries+1)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Infof(ctx, "[healthcheck] service [%s] on host [%s] is healthy", serviceName, host.Address)
		return nil
	}
	logrus.Debug("Checking container logs")
	containerLog, _, logserr := docker.GetContainerLogsStdoutStderr(ctx, host.DClient, serviceName, "1", false)
	containerLog = strings.TrimSuffix(containerLog, "\n")
	if logserr != nil {
		return fmt.Errorf("Failed to verify healthcheck for service [%s]: %v", serviceName, logserr)
	}
	return fmt.Errorf("Failed to verify healthcheck: %v, log: %v", err, containerLog)
}

func getHealthCheckHTTPClient(host *hosts.Host, port int, localConnDialerFactory hosts.DialerFactory, x509KeyPair *tls.Certificate) (*http.Client, error) {
	host.LocalConnPort = port
	var factory hosts.DialerFactory
	if localConnDialerFactory == nil {
		factory = hosts.LocalConnFactory
	} else {
		factory = localConnDialerFactory
	}
	dialer, err := factory(host)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a dialer for host [%s]: %v", host.Address, err)
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	if x509KeyPair != nil {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
			Certificates:       []tls.Certificate{*x509KeyPair},
		}
	}
	return &http.Client{
		Transport: &http.Transport{
			Dial:            dialer,
			TLSClientConfig: tlsConfig,
		},
	}, nil
}

func getHealthz(client *http.Client, serviceName, hostAddress, url string) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("Failed to check %s for service [%s] on host [%s]: %v", url, serviceName, hostAddress, err)
	}
	if resp.StatusCode != http.StatusOK {
		statusBody, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("Service [%s] is not healthy on host [%s]. Response code: [%d], response body: %s", serviceName, hostAddress, resp.StatusCode, statusBody)
	}
	return nil
}

func getPortFromURL(url string) (int, error) {
	port := strings.Split(strings.Split(url, ":")[2], "/")[0]
	intPort, err := strconv.Atoi(port)
	if err != nil {
		return 0, err
	}
	return intPort, nil
}
