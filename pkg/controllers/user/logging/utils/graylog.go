package utils

import (
	"crypto/tls"
	"net"

	"github.com/pkg/errors"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
)

var (
	graylogTestData = []byte(`{ "version": "1.1", "host": "rancher", "short_message": "A short message", "level": 5, "_some_info": "foo" }`)
)

type graylogTestWrap struct {
	*v3.GraylogConfig
}

func (w *graylogTestWrap) TestReachable(dial dialer.Dialer, includeSendTestLog bool) error {
	if w.Protocol == "udp" {
		conn, err := net.Dial("udp", w.Endpoint)
		if err != nil {
			return errors.Wrapf(err, "couldn't dail udp endpoint %s", w.Endpoint)
		}
		defer conn.Close()

		if includeSendTestLog {
			return writeToUDPConn(graylogTestData, w.Endpoint)
		}
		return nil
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

	conn, err := newTCPConn(dial, w.Endpoint, tlsConfig, true)
	if err != nil {
		return err
	}
	defer conn.Close()

	if !includeSendTestLog {
		return nil
	}

	if _, err = conn.Write(graylogTestData); err != nil {
		return errors.Wrapf(err, "couldn't write data to graylog %s", w.Endpoint)
	}

	// try read to check whether the server close connect already
	// because can't set read deadline for remote dialer, so if the error is timeout will treat as remote server not close the connection
	if _, err := readDataWithTimeout(conn); err != nil && err != errReadDataTimeout {
		return errors.Wrapf(err, "couldn't read data from graylog %s", w.Endpoint)
	}

	return nil
}
