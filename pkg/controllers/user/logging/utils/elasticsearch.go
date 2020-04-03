package utils

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/pkg/errors"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
)

type elasticsearchTestWrap struct {
	*v3.ElasticsearchConfig
}

func (w *elasticsearchTestWrap) TestReachable(ctx context.Context, dial dialer.Dialer, includeSendTestLog bool) error {
	url, err := url.Parse(w.Endpoint)
	if err != nil {
		return errors.Wrapf(err, "couldn't parse url %s", w.Endpoint)
	}

	isTLS := url.Scheme == "https"
	var tlsConfig *tls.Config
	if isTLS {
		tlsConfig, err = buildTLSConfig(w.Certificate, w.ClientCert, w.ClientKey, w.ClientKeyPass, w.SSLVersion, url.Hostname(), w.SSLVerify)
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

	index := getIndex(w.DateFormat, w.IndexPrefix)

	url.Path = path.Join(url.Path, index, "/container_log")

	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(httpTestData))
	if err != nil {
		return errors.Wrap(err, "create request failed")
	}
	req.Header.Set("Content-Type", "application/json")

	if w.AuthUserName != "" && w.AuthPassword != "" {
		req.SetBasicAuth(w.AuthUserName, w.AuthPassword)
	}

	return testReachableHTTP(dial, req, tlsConfig)
}

func getIndex(dateFormat, prefix string) string {
	var index string
	today := time.Now()
	switch dateFormat {
	case "YYYY":
		index = fmt.Sprintf("%s-%04d", prefix, today.Year())
	case "YYYY-MM":
		index = fmt.Sprintf("%s-%04d-%02d", prefix, today.Year(), today.Month())
	case "YYYY-MM-DD":
		index = fmt.Sprintf("%s-%04d-%02d-%02d", prefix, today.Year(), today.Month(), today.Day())
	}
	return index
}

func GetDateFormat(dateformat string) string {
	ToRealMap := map[string]string{
		"YYYY-MM-DD": "%Y-%m-%d",
		"YYYY-MM":    "%Y-%m",
		"YYYY":       "%Y",
	}
	if _, ok := ToRealMap[dateformat]; ok {
		return ToRealMap[dateformat]
	}
	return "%Y-%m-%d"
}
