package utils

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config/dialer"
)

var (
	httpTestData = []byte(`{"event": "` + testMessage + `", "sourcetype": "rancher"}`)
)

type splunkTestWrap struct {
	*v3.SplunkConfig
}

func (w *splunkTestWrap) TestReachable(ctx context.Context, dial dialer.Dialer, includeSendTestLog bool) error {
	url, err := url.Parse(w.Endpoint)
	if err != nil {
		return errors.Wrapf(err, "couldn't parse url %s", w.Endpoint)
	}

	isTLS := url.Scheme == "https"
	var tlsConfig *tls.Config
	if isTLS {
		tlsConfig, err = buildTLSConfig(w.Certificate, w.ClientCert, w.ClientKey, w.ClientKeyPass, "", url.Hostname(), w.SSLVerify)
		if err != nil {
			return err
		}
	}

	if !includeSendTestLog {
		conn, err := newTCPConn(ctx, dial, url.Host, tlsConfig, true)
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
