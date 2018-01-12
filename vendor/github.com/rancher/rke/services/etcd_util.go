package services

import (
	"context"
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

func getEtcdClient(ctx context.Context, etcdHost *hosts.Host, localConnDialerFactory hosts.DialerFactory) (etcdclient.Client, error) {
	dialer, err := getEtcdDialer(localConnDialerFactory, etcdHost)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a dialer for host [%s]: %v", etcdHost.Address, err)
	}

	var DefaultEtcdTransport etcdclient.CancelableTransport = &http.Transport{
		Dial: dialer,
	}

	cfg := etcdclient.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
		Transport: DefaultEtcdTransport,
	}

	return etcdclient.New(cfg)
}

func isEtcdHealthy(ctx context.Context, localConnDialerFactory hosts.DialerFactory, host *hosts.Host) bool {
	logrus.Debugf("[etcd] Check etcd cluster health")
	for i := 0; i < 3; i++ {
		dialer, err := getEtcdDialer(localConnDialerFactory, host)
		if err != nil {
			logrus.Debugf("Failed to create a dialer for host [%s]: %v", host.Address, err)
			time.Sleep(5 * time.Second)
			continue
		}
		hc := http.Client{
			Transport: &http.Transport{
				Dial: dialer,
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
	resp, err := hc.Get("http://127.0.0.1:2379/health")
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
		initialCluster += fmt.Sprintf("etcd-%s=http://%s:2380", host.HostnameOverride, host.InternalAddress)
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
		connString += "http://" + host.InternalAddress + ":2379"
		if i < (len(hosts) - 1) {
			connString += ","
		}
	}
	return connString
}
