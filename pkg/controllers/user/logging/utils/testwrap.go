package utils

import (
	"bytes"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"

	"github.com/pkg/errors"
	kafka "github.com/segmentio/kafka-go"
	"github.com/vmihailenco/msgpack"
)

const (
	sslv23  = "SSLv23"
	tlsv1   = "TLSv1"
	tlsv1_1 = "TLSv1_1"
	tlsv1_2 = "TLSv1_2"
)

const (
	letterHex = "0123456789abcdef"
	pongWait  = time.Duration(60 * time.Second)
)

var (
	httpTestData             = []byte(`{"event": "Rancher logging target setting validated", "sourcetype": "rancher"}`)
	fluentdForwarderTestData = []byte(fmt.Sprintf(`["rancher",[[ %d, {"message": "Rancher logging target setting validated"}]]]`, time.Now().Unix()))
)

type LoggingTargetTestWrap interface {
	TestReachable(dial dialer.Dialer) error
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

type elasticsearchTestWrap struct {
	*v3.ElasticsearchConfig
}

type splunkTestWrap struct {
	*v3.SplunkConfig
}

type kafkaTestWrap struct {
	*v3.KafkaConfig
}

type syslogTestWrap struct {
	*v3.SyslogConfig
}

type fluentForwarderTestWrap struct {
	*v3.FluentForwarderConfig
}

type customTargetTestWrap struct {
	*v3.CustomTargetConfig
}

func (w *elasticsearchTestWrap) TestReachable(dial dialer.Dialer) error {
	url, err := url.Parse(w.Endpoint)
	if err != nil {
		return errors.Wrapf(err, "parse url %s failed", w.Endpoint)
	}

	index := getIndex(w.DateFormat, w.IndexPrefix)

	url.Path = path.Join(url.Path, index, "/_doc")

	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(httpTestData))
	if err != nil {
		return errors.Wrap(err, "create request failed")
	}
	req.Header.Set("Content-Type", "application/json")

	return testReachableHTTP(dial, req, w.Certificate, w.ClientCert, w.ClientKey, w.ClientKeyPass, "", w.SSLVerify)
}

func (w *splunkTestWrap) TestReachable(dial dialer.Dialer) error {
	url, err := url.Parse(w.Endpoint)
	if err != nil {
		return errors.Wrapf(err, "parse url %s failed", url)
	}

	url.Path = path.Join(url.Path, "/services/collector")

	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(httpTestData))
	if err != nil {
		return errors.Wrap(err, "create request failed")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Splunk %s", w.Token))

	return testReachableHTTP(dial, req, w.Certificate, w.ClientCert, w.ClientKey, w.ClientKeyPass, "", w.SSLVerify)
}

func (w *kafkaTestWrap) TestReachable(dial dialer.Dialer) error {
	if w.SaslUsername != "" && w.SaslPassword != "" {
		//TODO: Now we don't have a out of the box Kafka go client fit our request which both support sasl and could pass conn to it.
		//kafka-go has a PR to support sasl, but not merge yet due to the mantainer want support Negotiation and Kerberos as well, we will add test func to sasl after the sasl in kafka-go is stable
		return nil
	}

	partition := 0
	if w.ZookeeperEndpoint != "" {
		url, err := url.Parse(w.ZookeeperEndpoint)
		if err != nil {
			return errors.Wrapf(err, "parse url %s failed", url)
		}

		return sendData2Kafka(w.Topic, partition, dial, "tcp", url.Host, "", "", "", "", "", false)
	}

	for _, v := range w.BrokerEndpoints {
		url, err := url.Parse(v)
		if err != nil {
			return errors.Wrapf(err, "parse url %s failed", url)
		}
		if err := sendData2Kafka(w.Topic, partition, dial, "tcp", url.Host, w.Certificate, w.ClientCert, w.ClientKey, "", "", false); err != nil {
			return err
		}
	}
	return nil
}

func (w *syslogTestWrap) TestReachable(dial dialer.Dialer) error {
	return testReachable(httpTestData, dial, w.Protocol, w.Endpoint, "", w.ClientCert, w.ClientKey, "", "", w.SSLVerify)
}

func (w *fluentForwarderTestWrap) TestReachable(dial dialer.Dialer) error {
	for _, s := range w.FluentServers {
		if err := w.sendData2Server(dial, s.SharedKey, s.Username, s.Password, s.Endpoint, s.Hostname, w.Certificate); err != nil {
			return err
		}
	}
	return nil
}

func (w *fluentForwarderTestWrap) sendData2Server(dial dialer.Dialer, shareKey, username, password, endpoint, hostname, certificate string) error {
	if shareKey == "" && username == "" && password == "" {
		return testReachable(fluentdForwarderTestData, dial, "tcp", endpoint, certificate, "", "", "", "", false)
	}
	conn, err := newConn(dial, "tcp", endpoint, certificate, "", "", "", "", hostname, false)
	if err != nil {
		return err
	}

	conn.SetReadDeadline(time.Now().Add(pongWait))
	defer conn.Close()

	buf := make([]byte, 1024)
	if _, err = conn.Read(buf); err != nil {
		return errors.Wrapf(err, "read data from fluent forwarder server %s failed", endpoint)
	}

	var heloBody []interface{}
	if err = msgpack.Unmarshal(buf, &heloBody); err != nil {
		return errors.Wrapf(err, "use msgpack to unmashal helo from %s failed", endpoint)
	}

	if len(heloBody) < 2 {
		return errors.New("helo body from fluentd don't have enough info")
	}

	var heloOption heloOption
	if err = convert.ToObj(heloBody[1], &heloOption); err != nil {
		return errors.Wrapf(err, "convert helo body from %s failed", endpoint)
	}

	nonce, err := base64.StdEncoding.DecodeString(heloOption.Nonce)
	if err != nil {
		return errors.Wrapf(err, "decode nonce from %s failed", endpoint)
	}

	auth, err := base64.StdEncoding.DecodeString(heloOption.Auth)
	if err != nil {
		return errors.Wrapf(err, "decode auth from %s failed", endpoint)
	}

	ping, err := w.generateFluentForwarderPing(shareKey, string(nonce), username, password, string(auth))
	if err != nil {
		return errors.Wrap(err, "generate fluent forwarder ping failed")
	}

	if _, err = conn.Write([]byte(ping)); err != nil {
		return errors.Wrap(err, "write ping info to fluent forwarder failed")
	}

	if _, err = conn.Write(fluentdForwarderTestData); err != nil {
		return errors.Wrap(err, "write test data to fluent forwarder failed")
	}

	return nil
}

func (w *customTargetTestWrap) TestReachable(dial dialer.Dialer) error {
	return nil
}

type heloOption struct {
	Nonce     string `json:"nonce"`
	Auth      string `json:"auth"`
	Keepalive bool   `json:"keepalive"`
}

func (w *fluentForwarderTestWrap) generateFluentForwarderPing(shareKey, nonce, username, password, auth string) (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", errors.Wrap(err, "get host failed")
	}

	salt := randHex(16)
	fullSharedKey := fmt.Sprintf("%s%s%s%s", salt, hostname, nonce, shareKey)
	hash := sha512.New()
	hash.Write([]byte(fullSharedKey))
	sharedKeyHex := hex.EncodeToString(hash.Sum(nil))

	passwordHex := ""
	if auth != "" {
		fullPassword := fmt.Sprintf("%s%s%s", auth, username, password)
		passwordHash := sha512.New()
		passwordHash.Write([]byte(fullPassword))
		passwordHex = hex.EncodeToString(passwordHash.Sum(nil))
	}
	return fmt.Sprintf(`["PING", "%s", "%s", "%s", "%s", "%s"]`, hostname, salt, sharedKeyHex, username, passwordHex), nil
}

func randHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterHex[rand.Intn(len(letterHex))]
	}
	return string(b)
}

func testReachableHTTP(dial dialer.Dialer, req *http.Request, rootCA, clientCert, clientKey, clientKeyPass, sslVersion string, sslVerify bool) error {
	tlsConfig, err := buildTLSConfig(rootCA, clientCert, clientKey, clientKeyPass, sslVersion, "", sslVerify)
	if err != nil {
		return errors.Wrap(err, "build tls config failed")
	}

	transport := &http.Transport{
		Dial:            dial,
		TLSClientConfig: tlsConfig,
	}

	client := http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	if _, err = client.Do(req); err != nil {
		return errors.Wrapf(err, "request to logging target %s failed", req.URL)
	}
	return nil
}

func testReachable(data []byte, dialer dialer.Dialer, protocal, smartHost, rootCA, clientCert, clientKey, clientKeyPass, sslVersion string, sslVerify bool) error {
	conn, err := newConn(dialer, protocal, smartHost, rootCA, clientCert, clientKey, clientKeyPass, sslVersion, "", sslVerify)
	if err != nil {
		return err
	}
	_, err = conn.Write(data)
	if err != nil {
		return errors.Wrapf(err, "write data to server %s failed", smartHost)
	}
	defer conn.Close()
	return nil
}

func newConn(dialer dialer.Dialer, protocal, smartHost, rootCA, clientCert, clientKey, clientKeyPass, sslVersion, serverName string, sslVerify bool) (net.Conn, error) {
	tlsConfig, err := buildTLSConfig(rootCA, clientCert, clientKey, clientKeyPass, sslVersion, serverName, sslVerify)
	if err != nil {
		return nil, errors.Wrap(err, "build tls config failed")
	}

	conn, err := dialer(protocal, smartHost)
	if err != nil {
		return nil, errors.Wrap(err, "create raw conn failed")
	}

	if tlsConfig == nil {
		return conn, nil
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, errors.Wrap(err, "tls handshake failed")
	}

	return tlsConn, nil
}

func sendData2Kafka(topic string, partition int, dialer dialer.Dialer, protocal, smartHost, rootCA, clientCert, clientKey, clientKeyPass, sslVersion string, sslVerify bool) error {
	conn, err := newConn(dialer, protocal, smartHost, rootCA, clientCert, clientKey, clientKeyPass, sslVersion, "", sslVerify)
	if err != nil {
		return err
	}
	defer conn.Close()

	topicConf := kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	}
	kafkaConn := kafka.NewConn(conn, topic, partition)
	if err = kafkaConn.CreateTopics(topicConf); err != nil {
		return errors.Wrapf(err, "create topic %s failed", topic)
	}
	defer kafkaConn.Close()

	if err = kafkaConn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return errors.Wrap(err, "set kafka conn deadline failed")
	}
	if _, err = kafkaConn.WriteMessages(
		kafka.Message{
			Value: []byte("Rancher logging target setting validated!")},
	); err != nil {
		return errors.Wrap(err, "write test message to kafka failed")
	}

	return nil
}

func decodePEM(clientKey, clientKeyPass string) (string, error) {
	p, _ := pem.Decode([]byte(clientKey))

	decodeClientKey, err := x509.DecryptPEMBlock(p, []byte(clientKeyPass))
	if err != nil {
		return "", errors.Wrap(err, "decrrypt client key failed")
	}

	return string(decodeClientKey[:]), err
}

func buildTLSConfig(rootCA, clientCert, clientKey, clientKeyPass, sslVersion, serverName string, sslVerify bool) (config *tls.Config, err error) {
	if rootCA == "" && clientCert == "" && clientKey == "" && clientKeyPass == "" && sslVersion == "" {
		return nil, nil
	}

	config = &tls.Config{
		InsecureSkipVerify: !sslVerify,
	}

	if serverName != "" {
		config.InsecureSkipVerify = false
		config.ServerName = serverName
	}

	var decodeClientKey = clientKey
	if clientKeyPass != "" {
		decodeClientKey, err = decodePEM(clientKey, clientKeyPass)
		if err != nil {
			return nil, err
		}
	}

	if clientCert != "" {
		cert, err := tls.X509KeyPair([]byte(clientCert), []byte(decodeClientKey))
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
