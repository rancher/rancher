package services

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/rancher/rke/hosts"
	"github.com/sirupsen/logrus"
)

const (
	HealthzAddress   = "localhost"
	HealthzEndpoint  = "/healthz"
	HTTPProtoPrefix  = "http://"
	HTTPSProtoPrefix = "https://"
)

func runHealthcheck(host *hosts.Host, port int, useTLS bool, serviceName string, healthcheckDialerFactory hosts.DialerFactory) error {
	logrus.Infof("[healthcheck] Start Healthcheck on service [%s] on host [%s]", serviceName, host.Address)
	client, err := getHealthCheckHTTPClient(host, port, healthcheckDialerFactory)
	if err != nil {
		return fmt.Errorf("Failed to initiate new HTTP client for service [%s] for host [%s]", serviceName, host.Address)
	}
	for retries := 0; retries < 3; retries++ {
		if err = getHealthz(client, useTLS, serviceName, host.Address); err != nil {
			logrus.Debugf("[healthcheck] %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		logrus.Infof("[healthcheck] service [%s] on host [%s] is healthy", serviceName, host.Address)
		return nil
	}
	return fmt.Errorf("Failed to verify healthcheck: %v", err)
}

func getHealthCheckHTTPClient(host *hosts.Host, port int, healthcheckDialerFactory hosts.DialerFactory) (*http.Client, error) {
	host.HealthcheckPort = port
	var factory hosts.DialerFactory
	if healthcheckDialerFactory == nil {
		factory = hosts.HealthcheckFactory
	} else {
		factory = healthcheckDialerFactory
	}
	dialer, err := factory(host)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a dialer for host [%s]: %v", host.Address, err)
	}
	return &http.Client{
		Transport: &http.Transport{
			Dial:            dialer,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}, nil
}

func getHealthz(client *http.Client, useTLS bool, serviceName, hostAddress string) error {
	proto := HTTPProtoPrefix
	if useTLS {
		proto = HTTPSProtoPrefix
	}
	resp, err := client.Get(fmt.Sprintf("%s%s%s", proto, HealthzAddress, HealthzEndpoint))
	if err != nil {
		return fmt.Errorf("Failed to check %s for service [%s] on host [%s]: %v", HealthzEndpoint, serviceName, hostAddress, err)
	}
	if resp.StatusCode != http.StatusOK {
		statusBody, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("service [%s] is not healthy response code: [%d], response body: %s", serviceName, resp.StatusCode, statusBody)
	}
	return nil
}
