package utils

import (
	"crypto/tls"
	"net"

	"github.com/pkg/errors"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
)

var (
	graylogTestData = []byte(`{ "version": "1.1", "host": "rancher", "level": 5, "short_message": "` + testMessage + `"}`)
)

type graylogTestWrap struct {
	*v3.GraylogConfig
}

func (w *graylogTestWrap) TestReachable(dial dialer.Dialer, includeSendTestLog bool) error {
	if w.Protocol == "udp" {
		return testReachableUDP(includeSendTestLog, w.Endpoint, graylogTestData)
	}
	var tlsConfig *tls.Config
	if w.EnableTLS {
		hostName, _, err := net.SplitHostPort(w.Endpoint)
		if err != nil {
			return errors.Wrapf(err, "couldn't parse url %s", w.Endpoint)
		}

		tlsConfig, err = buildTLSConfig(w.Certificate, w.ClientCert, w.ClientKey, "", "", hostName, w.SSLVerify)
		if err != nil {
			return err
		}
	}
	return testReachableTCP(dial, includeSendTestLog, w.Endpoint, tlsConfig, graylogTestData)
}
