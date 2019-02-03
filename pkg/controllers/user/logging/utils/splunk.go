package utils

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
)

var (
	httpTestData = []byte(`{"event": "` + testMessage + `", "sourcetype": "rancher"}`)
)

type splunkTestWrap struct {
	*v3.SplunkConfig
}

func (w *splunkTestWrap) TestReachable(dial dialer.Dialer, includeSendTestLog bool) error {
	url, err := url.Parse(w.Endpoint)
	if err != nil {
		return errors.Wrapf(err, "parse url %s failed", url)
	}

	isTLS := url.Scheme == "https"
	var tlsConfig *tls.Config
	if isTLS {
		tlsConfig, err = buildTLSConfig(w.Certificate, w.ClientCert, w.ClientKey, w.ClientKeyPass, "", url.Hostname(), w.SSLVerify)
		if err != nil {
			return errors.Wrap(err, "build tls config failed")
		}
	}

	if !includeSendTestLog {
		conn, err := newTCPConn(dial, url.Host, tlsConfig, true)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}

	url.Path = path.Join(url.Path, "/services/collector")
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(httpTestData))
	if err != nil {
		return errors.Wrap(err, "create request failed")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Splunk %s", w.Token))

	return testReachableHTTP(dial, req, tlsConfig)
}
