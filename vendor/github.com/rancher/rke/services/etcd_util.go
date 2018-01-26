package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/rancher/rke/hosts"
	"github.com/sirupsen/logrus"
)

func getEtcdClient(ctx context.Context, etcdHost *hosts.Host, localConnDialerFactory hosts.DialerFactory, cert, key []byte) (etcdclient.Client, error) {
	dialer, err := getEtcdDialer(localConnDialerFactory, etcdHost)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a dialer for host [%s]: %v", etcdHost.Address, err)
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
		Endpoints: []string{"https://127.0.0.1:2379"},
		Transport: DefaultEtcdTransport,
	}

	return etcdclient.New(cfg)
}

func isEtcdHealthy(ctx context.Context, localConnDialerFactory hosts.DialerFactory, host *hosts.Host, cert, key []byte) bool {
	logrus.Debugf("[etcd] Check etcd cluster health")
	for i := 0; i < 3; i++ {
		dialer, err := getEtcdDialer(localConnDialerFactory, host)
		if err != nil {
			return false
		}
		tlsConfig, err := getEtcdTLSConfig(cert, key)
		if err != nil {
			logrus.Debugf("[etcd] Failed to create etcd tls config for host [%s]: %v", host.Address, err)
			return false
		}

		hc := http.Client{
			Transport: &http.Transport{
				Dial:                dialer,
				TLSClientConfig:     tlsConfig,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		}
		healthy, err := getHealthEtcd(hc, host)
		if err != nil {
			logrus.Debug(err)
			time.Sleep(5 * time.Second)
			continue
		}
		if healthy == "true" {
			logrus.Debugf("[etcd] etcd cluster is healthy")
			return true
		}
	}
	return false
}

func getHealthEtcd(hc http.Client, host *hosts.Host) (string, error) {
	healthy := struct{ Health string }{}
	resp, err := hc.Get("https://127.0.0.1:2379/health")
	if err != nil {
		return healthy.Health, fmt.Errorf("Failed to get /health for host [%s]: %v", host.Address, err)
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return healthy.Health, fmt.Errorf("Failed to read response of /health for host [%s]: %v", host.Address, err)
	}
	resp.Body.Close()
	if err := json.Unmarshal(bytes, &healthy); err != nil {
		return healthy.Health, fmt.Errorf("Failed to unmarshal response of /health for host [%s]: %v", host.Address, err)
	}
	return healthy.Health, nil
}

func getEtcdInitialCluster(hosts []*hosts.Host) string {
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

func GetEtcdConnString(hosts []*hosts.Host) string {
	connString := ""
	for i, host := range hosts {
		connString += "https://" + host.InternalAddress + ":2379"
		if i < (len(hosts) - 1) {
			connString += ","
		}
	}
	return connString
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
	if err != nil {
		return nil, err
	}
	return tlsConfig, nil
}
