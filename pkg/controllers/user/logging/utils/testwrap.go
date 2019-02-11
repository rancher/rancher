package utils

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"

	"github.com/pkg/errors"
)

const (
	sslv23  = "SSLv23"
	tlsv1   = "TLSv1"
	tlsv1_1 = "TLSv1_1"
	tlsv1_2 = "TLSv1_2"
)

const (
	deadlineTimeout = time.Duration(2 * time.Second)
)

var (
	testMessage = "Rancher logging target setting validated"
)

type LoggingTargetTestWrap interface {
	TestReachable(dial dialer.Dialer, includeSendTestLog bool) error
}

func NewLoggingTargetTestWrap(loggingTargets v3.LoggingTargets) LoggingTargetTestWrap {
	if loggingTargets.ElasticsearchConfig != nil {
		return &elasticsearchTestWrap{loggingTargets.ElasticsearchConfig}
	} else if loggingTargets.SplunkConfig != nil {
		return &splunkTestWrap{loggingTargets.SplunkConfig}
	} else if loggingTargets.SyslogConfig != nil {
		return &syslogTestWrap{loggingTargets.SyslogConfig}
	} else if loggingTargets.KafkaConfig != nil {
		return &kafkaTestWrap{loggingTargets.KafkaConfig}
	} else if loggingTargets.FluentForwarderConfig != nil {
		return &fluentForwarderTestWrap{loggingTargets.FluentForwarderConfig}
	} else if loggingTargets.CustomTargetConfig != nil {
		return &customTargetTestWrap{loggingTargets.CustomTargetConfig}
	}

	return nil
}

func testReachableHTTP(dial dialer.Dialer, req *http.Request, tlsConfig *tls.Config) error {
	transport := &http.Transport{
		Dial:            dial,
		TLSClientConfig: tlsConfig,
	}

	client := http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "request to logging target %s failed", req.URL)
	}

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return errors.Wrapf(err, "read response body from %s failed, response code is %v", req.URL.String(), res.StatusCode)
		}
		return fmt.Errorf("response code from %s is %v, not include in the 2xx success HTTP status codes, response body: %s", req.URL.String(), res.StatusCode, string(body))
	}

	return nil
}

func writeToUDPConn(data []byte, smartHost string) error {
	conn, err := net.Dial("udp", smartHost)
	if err != nil {
		return errors.Wrapf(err, "dail to udp endpoint %s failed", smartHost)
	}
	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		return errors.Wrapf(err, "write udp endpoint %s failed", smartHost)
	}
	return nil
}

func newTCPConn(dialer dialer.Dialer, smartHost string, tlsConfig *tls.Config, handshake bool) (net.Conn, error) {
	conn, err := dialer("tcp", smartHost)
	if err != nil {
		return nil, errors.Wrap(err, "create raw conn failed")
	}
	conn.SetDeadline(time.Now().Add(deadlineTimeout))

	if tlsConfig == nil {
		return conn, nil
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if handshake {
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return nil, errors.Wrapf(err, "tls handshake %s failed", smartHost)
		}
	}
	return tlsConn, nil
}

func newUDPConn(smartHost string) (net.Conn, error) {
	conn, err := net.Dial("udp", smartHost)
	if err != nil {
		return nil, errors.Wrapf(err, "dail to udp endpoint %s failed", smartHost)
	}
	conn.SetDeadline(time.Now().Add(deadlineTimeout))
	return conn, nil
}

func decodePEM(clientKey, passphrase string) ([]byte, error) {
	clientKeyBytes := []byte(clientKey)
	pemBlock, _ := pem.Decode(clientKeyBytes)
	if pemBlock == nil {
		return nil, fmt.Errorf("no valid private key found")
	}

	var err error
	if x509.IsEncryptedPEMBlock(pemBlock) {
		clientKeyBytes, err = x509.DecryptPEMBlock(pemBlock, []byte(passphrase))
		if err != nil {
			return nil, errors.Wrap(err, "private key is encrypted, but could not decrypt it")
		}
		clientKeyBytes = pem.EncodeToMemory(&pem.Block{Type: pemBlock.Type, Bytes: clientKeyBytes})
	}

	return clientKeyBytes, nil
}

func buildTLSConfig(rootCA, clientCert, clientKey, clientKeyPass, sslVersion, serverName string, sslVerify bool) (config *tls.Config, err error) {
	if rootCA == "" && clientCert == "" && clientKey == "" && clientKeyPass == "" && sslVersion == "" {
		return nil, nil
	}

	config = &tls.Config{
		InsecureSkipVerify: !sslVerify,
		ServerName:         serverName,
	}

	var decodeClientKeyBytes = []byte(clientKey)
	if clientKeyPass != "" {
		decodeClientKeyBytes, err = decodePEM(clientKey, clientKeyPass)
		if err != nil {
			return nil, err
		}
	}

	if clientCert != "" {
		cert, err := tls.X509KeyPair([]byte(clientCert), decodeClientKeyBytes)
		if err != nil {
			return nil, errors.Wrap(err, "load client cert and key failed")
		}

		config.Certificates = []tls.Certificate{cert}
	}

	if rootCA != "" {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(rootCA))

		config.RootCAs = caCertPool
	}

	if sslVersion != "" {
		switch sslVersion {
		case sslv23:
			config.MaxVersion = tls.VersionSSL30
		case tlsv1:
			config.MaxVersion = tls.VersionTLS10
			config.MinVersion = tls.VersionTLS10
		case tlsv1_1:
			config.MaxVersion = tls.VersionTLS11
			config.MinVersion = tls.VersionTLS11
		case tlsv1_2:
			config.MaxVersion = tls.VersionTLS12
			config.MinVersion = tls.VersionTLS12
		}
	}

	return config, nil
}

func randHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterHex[rand.Intn(len(letterHex))]
	}
	return string(b)
}
