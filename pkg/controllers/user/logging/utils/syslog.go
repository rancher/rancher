package utils

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/pkg/errors"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
)

type syslogTestWrap struct {
	*v3.SyslogConfig
}

func (w *syslogTestWrap) TestReachable(dial dialer.Dialer, includeSendTestLog bool) error {
	//TODO: for udp we can't use cluster dialer now, how to handle in cluster deploy syslog
	syslogTestData := newRFC5424Message(w.Severity, w.Program, w.Token, testMessage)
	if w.Protocol == "udp" {
		return testReachableUDP(includeSendTestLog, w.Endpoint, syslogTestData)
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
	return testReachableTCP(dial, includeSendTestLog, w.Endpoint, tlsConfig, syslogTestData)
}

func newRFC5424Message(severityStr, app, token, msg string) []byte {
	if app == "" {
		app = "rancher"
	}

	severity := getSeverity(severityStr)
	syslogVersion := 1
	timestamp := time.Now().Format(time.RFC3339)
	msgID := randHex(6)
	hostname, _ := os.Hostname()

	return []byte(fmt.Sprintf("<%d>%v %v %v %v %v %v [%v] %v\n",
		severity,
		syslogVersion,
		timestamp,
		hostname,
		app,
		os.Getpid(),
		msgID,
		token,
		msg,
	))
}

func GetWrapSeverity(severity string) string {
	// for adapt api and fluentd config
	severityMap := map[string]string{
		"warning": "warn",
	}

	wrapSeverity := severityMap[severity]
	if wrapSeverity == "" {
		return severity
	}

	return wrapSeverity
}
