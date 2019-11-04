package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/rancher/rke/hosts"
	"github.com/sirupsen/logrus"
)

func getEtcdClient(ctx context.Context, etcdHost *hosts.Host, localConnDialerFactory hosts.DialerFactory, cert, key []byte) (etcdclient.Client, error) {
	dialer, err := getEtcdDialer(localConnDialerFactory, etcdHost)
	if err != nil {
		return nil, fmt.Errorf("failed to create a dialer for host [%s]: %v", etcdHost.Address, err)
	}
	tlsConfig, err := getEtcdTLSConfig(cert, key)
	if err != nil {
		return nil, err
	}

	var DefaultEtcdTransport etcdclient.CancelableTransport = &http.Transport{
		Dial:                dialer,
		TLSClientConfig:     tlsConfig,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	cfg := etcdclient.Config{
		Endpoints: []string{"https://" + etcdHost.InternalAddress + ":2379"},
		Transport: DefaultEtcdTransport,
	}

	return etcdclient.New(cfg)
}

func isEtcdHealthy(localConnDialerFactory hosts.DialerFactory, host *hosts.Host, cert, key []byte, url string) error {
	logrus.Debugf("[etcd] check etcd cluster health on host [%s]", host.Address)
	var finalErr error
	var healthy string
	for i := 0; i < 3; i++ {
		dialer, err := getEtcdDialer(localConnDialerFactory, host)
		if err != nil {
			return err
		}
		tlsConfig, err := getEtcdTLSConfig(cert, key)
		if err != nil {
			return fmt.Errorf("[etcd] failed to create etcd tls config for host [%s]: %v", host.Address, err)
		}

		hc := http.Client{
			Transport: &http.Transport{
				Dial:                dialer,
				TLSClientConfig:     tlsConfig,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		}
		healthy, finalErr = getHealthEtcd(hc, host, url)
		if finalErr != nil {
			logrus.Debugf("[etcd] failed to check health for etcd host [%s]: %v", host.Address, finalErr)
			time.Sleep(5 * time.Second)
			continue
		}
		// log in debug here as we don't want to log in warn on every iteration
		// the error will be logged in the caller stack
		logrus.Debugf("[etcd] etcd host [%s] reported healthy=%s", host.Address, healthy)
		if healthy == "true" {
			return nil
		}
	}
	if finalErr != nil {
		return fmt.Errorf("[etcd] host [%s] failed to check etcd health: %v", host.Address, finalErr)
	}
	return fmt.Errorf("[etcd] host [%s] reported healthy=%s", host.Address, healthy)
}

func getHealthEtcd(hc http.Client, host *hosts.Host, url string) (string, error) {
	healthy := struct{ Health string }{}
	resp, err := hc.Get(url)
	if err != nil {
		return healthy.Health, fmt.Errorf("failed to get /health for host [%s]: %v", host.Address, err)
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return healthy.Health, fmt.Errorf("failed to read response of /health for host [%s]: %v", host.Address, err)
	}
	resp.Body.Close()
	if err := json.Unmarshal(bytes, &healthy); err != nil {
		return healthy.Health, fmt.Errorf("failed to unmarshal response of /health for host [%s]: %v", host.Address, err)
	}
	return healthy.Health, nil
}

func GetEtcdInitialCluster(hosts []*hosts.Host) string {
	initialCluster := ""
	for i, host := range hosts {
		initialCluster += fmt.Sprintf("etcd-%s=https://%s:2380", host.HostnameOverride, host.InternalAddress)
		if i < (len(hosts) - 1) {
			initialCluster += ","
		}
	}
	return initialCluster
}

func getEtcdDialer(localConnDialerFactory hosts.DialerFactory, etcdHost *hosts.Host) (func(network, address string) (net.Conn, error), error) {
	etcdHost.LocalConnPort = 2379
	var etcdFactory hosts.DialerFactory
	if localConnDialerFactory == nil {
		etcdFactory = hosts.LocalConnFactory
	} else {
		etcdFactory = localConnDialerFactory
	}
	return etcdFactory(etcdHost)
}

func GetEtcdConnString(hosts []*hosts.Host, hostAddress string) string {
	connHosts := []string{}
	containsHostAddress := false
	for _, host := range hosts {
		if host.InternalAddress == hostAddress {
			containsHostAddress = true
			continue
		}
		connHosts = append(connHosts, "https://"+host.InternalAddress+":2379")
	}
	if containsHostAddress {
		connHosts = append([]string{"https://" + hostAddress + ":2379"}, connHosts...)
	}
	return strings.Join(connHosts, ",")
}

func getEtcdTLSConfig(certificate, key []byte) (*tls.Config, error) {
	// get tls config
	x509Pair, err := tls.X509KeyPair([]byte(certificate), []byte(key))
	if err != nil {
		return nil, err

	}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{x509Pair},
	}
	return tlsConfig, nil
}
